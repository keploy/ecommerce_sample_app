import os
import uuid
import json
import requests
import jwt
import boto3
import mysql.connector
from flask import Flask, request, jsonify
import coverage as _coverage

app = Flask(__name__)

USER_SERVICE_URL = os.environ.get('USER_SERVICE_URL', 'http://localhost:8082/api/v1')
PRODUCT_SERVICE_URL = os.environ.get('PRODUCT_SERVICE_URL', 'http://localhost:8081/api/v1')
JWT_SECRET = os.environ.get('JWT_SECRET', 'dev-secret-change-me')
JWT_ALG = 'HS256'

SQS_QUEUE_URL = os.environ.get('SQS_QUEUE_URL')
# Always honor explicit endpoint when provided (e.g., LocalStack: http://localstack:4566)
# For SQS, SendMessage talks to the service endpoint and passes QueueUrl as a parameter,
# so we must set endpoint_url to hit LocalStack instead of real AWS.
endpoint_url = os.environ.get('AWS_ENDPOINT') or os.environ.get('AWS_ENDPOINT_URL')
sqs = boto3.client(
    'sqs',
    region_name=os.environ.get('AWS_REGION', 'us-east-1'),
    endpoint_url=endpoint_url
)
from functools import wraps

def _auth_ok():
    auth = request.headers.get('Authorization') or ''
    if not auth.startswith('Bearer '):
        return False
    token = auth.split(' ', 1)[1].strip()
    try:
        jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALG])
        return True
    except Exception:
        return False

def require_auth(fn):
    @wraps(fn)
    def wrapper(*args, **kwargs):
        if not _auth_ok():
            return jsonify({'error': 'Unauthorized'}), 401
        return fn(*args, **kwargs)
    return wrapper


def _fwd_auth_headers():
    auth = request.headers.get('Authorization')
    return {'Authorization': auth} if auth else {}


def get_db_connection():
    try:
        conn = mysql.connector.connect(
            host=os.environ.get('DB_HOST', 'localhost'),
            user=os.environ.get('DB_USER', 'user'),
            password=os.environ.get('DB_PASSWORD', 'password'),
            database=os.environ.get('DB_NAME', 'order_db')
        )
        return conn
    except mysql.connector.Error as err:
        print(f"Error: {err}")
        return None


def _validate_items(items):
    if not isinstance(items, list) or len(items) == 0:
        return False, 'items must be a non-empty array'
    for it in items:
        if not isinstance(it, dict):
            return False, 'Each item must be an object'
        if 'productId' not in it or 'quantity' not in it:
            return False, 'Each item requires productId and quantity'
        try:
            qty = int(it['quantity'])
            if qty <= 0:
                return False, 'quantity must be > 0'
        except Exception:
            return False, 'quantity must be a positive integer'
    return True, None


def _emit_event(event_type, payload):
    try:
        body = { 'eventType': event_type, **payload }
        sqs.send_message(QueueUrl=SQS_QUEUE_URL, MessageBody=json.dumps(body))
    except Exception as e:
        print(f"Failed to send SQS message: {e}")


@app.route('/api/v1/orders', methods=['POST'])
@require_auth
def create_order():
    data = request.get_json()
    if not data or not all(k in data for k in ('userId', 'items')):
        return jsonify({'error': 'Missing required fields'}), 400

    user_id = data['userId']
    items = data['items']
    # New model: store only shippingAddressId; details are fetched from user-service on read.
    shipping_address_id = data.get('shippingAddressId')
    idmp_key = request.headers.get('Idempotency-Key')

    ok, err = _validate_items(items)
    if not ok:
        return jsonify({'error': err}), 400

    try:
        user_response = requests.get(f"{USER_SERVICE_URL}/users/{user_id}", headers=_fwd_auth_headers())
        if user_response.status_code != 200:
            return jsonify({'error': 'Invalid user ID'}), 400
        user_json = user_response.json()
    except requests.exceptions.RequestException as e:
        return jsonify({'error': f'Could not connect to User Service: {e}'}), 503

    # Resolve shipping_address_id: either validate provided ID or pick default
    def _pick_default_address_id(uid):
        try:
            r = requests.get(f"{USER_SERVICE_URL}/users/{uid}/addresses", headers=_fwd_auth_headers(), timeout=5)
            if r.status_code == 200:
                arr = r.json()
                if isinstance(arr, list) and len(arr) > 0:
                    return arr[0].get('id')
        except Exception:
            return None
        return None

    if shipping_address_id:
        # Validate it belongs to the user
        try:
            r = requests.get(f"{USER_SERVICE_URL}/users/{user_id}/addresses", headers=_fwd_auth_headers(), timeout=5)
            if r.status_code == 200:
                ids = {a.get('id') for a in r.json() if isinstance(a, dict)}
                if shipping_address_id not in ids:
                    return jsonify({'error': 'shippingAddressId does not belong to user'}), 400
            else:
                return jsonify({'error': 'Failed to validate shippingAddressId'}), 400
        except Exception:
            return jsonify({'error': 'Failed to validate shippingAddressId'}), 400
    else:
        shipping_address_id = _pick_default_address_id(user_id)

    total_amount = 0
    for item in items:
        product_id = item['productId']
        try:
            product_response = requests.get(f"{PRODUCT_SERVICE_URL}/products/{product_id}", headers=_fwd_auth_headers())
            if product_response.status_code != 200:
                return jsonify({'error': f'Product with ID {product_id} not found'}), 400
            product_data = product_response.json()
            if product_data['stock'] < item['quantity']:
                return jsonify({'error': f"Not enough stock for product {product_data['name']}"}), 400
            item['price'] = float(product_data['price'])
            total_amount += item['price'] * item['quantity']
        except requests.exceptions.RequestException as e:
            return jsonify({'error': f'Could not connect to Product Service: {e}'}), 503

    # Reserve stock for each item
    reserved = []
    for item in items:
        try:
            resp = requests.post(
                f"{PRODUCT_SERVICE_URL}/products/{item['productId']}/reserve",
                json={'quantity': item['quantity']},
                headers=_fwd_auth_headers()
            )
            reserved.append({'productId': item['productId'], 'quantity': item['quantity']})
        except requests.exceptions.RequestException as e:
            return jsonify({'error': f'Could not reserve stock: {e}'}), 503

    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500

    cursor = conn.cursor(dictionary=True)
    order_id = str(uuid.uuid4())
    try:
        cursor.execute(
            "INSERT INTO orders (id, user_id, status, idempotency_key, total_amount, shipping_address_id) VALUES (%s, %s, %s, %s, %s, %s)",
            (order_id, user_id, 'PENDING', idmp_key, round(total_amount, 2), shipping_address_id)
        )
        item_params = [(order_id, i['productId'], i['quantity'], i['price']) for i in items]
        cursor.executemany(
            "INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (%s, %s, %s, %s)",
            item_params
        )
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback()
        # release reserved stock on failure
        for r in reserved:
            try:
                requests.post(f"{PRODUCT_SERVICE_URL}/products/{r['productId']}/release", json={'quantity': r['quantity']})
            except Exception:
                pass
        return jsonify({'error': f'Failed to create order: {err}'}), 500
    finally:
        cursor.close()
        conn.close()

    _emit_event('order_created', {
        'orderId': order_id,
        'userId': user_id,
        'totalAmount': total_amount,
        'items': items
    })

    return jsonify({'id': order_id, 'status': 'PENDING'}), 201


@app.route('/api/v1/orders', methods=['GET'])
@require_auth
def list_orders():
    # Filters: userId, status; pagination: limit, cursor(created_at, id)
    user_id = request.args.get('userId')
    status = request.args.get('status')
    limit = min(max(int(request.args.get('limit', 20)), 1), 100)

    sql = "SELECT id, user_id, status, total_amount, created_at FROM orders"
    params = []
    where = []
    if user_id:
        where.append("user_id=%s")
        params.append(user_id)
    if status:
        where.append("status=%s")
        params.append(status)
    if where:
        sql += " WHERE " + " AND ".join(where)
    # Keyset pagination using (created_at, id)
    sql += " ORDER BY created_at DESC, id ASC LIMIT %s"
    params.append(limit + 1)

    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute(sql, params)
        rows = cur.fetchall()
    finally:
        conn.close()

    next_cursor = None

    return jsonify({'orders': rows, 'nextCursor': next_cursor}), 200


@app.route('/api/v1/orders/<order_id>', methods=['GET'])
@require_auth
def get_order(order_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute("SELECT id, user_id, status, total_amount, shipping_address_id, created_at, updated_at FROM orders WHERE id=%s", (order_id,))
        order = cur.fetchone()
        if not order:
            return jsonify({'error': 'Not found'}), 404
        cur.execute("SELECT product_id, quantity, price FROM order_items WHERE order_id=%s", (order_id,))
        items = cur.fetchall()
        order['items'] = items
    finally:
        conn.close()
    return jsonify(order), 200


@app.route('/api/v1/orders/<order_id>/details', methods=['GET'])
@require_auth
def get_order_details(order_id):
    """
    Microservice-friendly projection: return minimal stored fields and fetch details
    from dependent services (user-service, product-service) at read time.
    """
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute("SELECT id, user_id, status, total_amount, created_at, updated_at FROM orders WHERE id=%s", (order_id,))
        order = cur.fetchone()
        if not order:
            return jsonify({'error': 'Not found'}), 404
        cur.execute("SELECT product_id, quantity FROM order_items WHERE order_id=%s", (order_id,))
        items = cur.fetchall() or []
    finally:
        conn.close()

    # Fetch user details from user-service
    user_obj = None
    try:
        uresp = requests.get(f"{USER_SERVICE_URL}/users/{order['user_id']}", headers=_fwd_auth_headers(), timeout=5)
        if uresp.status_code == 200:
            user_obj = uresp.json()
    except Exception:
        # keep user_obj as None on failure
        pass

    # Fetch product details for each item from product-service
    enriched_items = []
    for it in items:
        pid = it['product_id']
        product_obj = None
        try:
            presp = requests.get(f"{PRODUCT_SERVICE_URL}/products/{pid}", headers=_fwd_auth_headers(), timeout=5)
            if presp.status_code == 200:
                product_obj = presp.json()
        except Exception:
            pass
        enriched_items.append({
            'productId': pid,
            'quantity': it['quantity'],
            'product': product_obj
        })

    # Fetch shipping address details if we have an id; otherwise optionally try default
    shipping_addr = None
    try:
        # We don't have a direct GET-by-id address endpoint; list and filter
        ar = requests.get(f"{USER_SERVICE_URL}/users/{order['user_id']}/addresses", headers=_fwd_auth_headers(), timeout=5)
        if ar.status_code == 200:
            arr = ar.json() or []
            if order.get('shipping_address_id'):
                shipping_addr = next((a for a in arr if a.get('id') == order['shipping_address_id']), None)
            else:
                shipping_addr = arr[0] if len(arr) > 0 else None
    except Exception:
        pass

    response = {
        'id': order['id'],
        'status': order['status'],
        'total_amount': float(order['total_amount']) if order.get('total_amount') is not None else None,
        'created_at': order.get('created_at').isoformat() if order.get('created_at') else None,
        'updated_at': order.get('updated_at').isoformat() if order.get('updated_at') else None,
        'userId': order['user_id'],
        'shippingAddressId': order.get('shipping_address_id'),
        'shippingAddress': shipping_addr,
        'user': user_obj,
        'items': enriched_items
    }

    return jsonify(response), 200


@app.route('/api/v1/orders/<order_id>/cancel', methods=['POST'])
@require_auth
def cancel_order(order_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute("SELECT status FROM orders WHERE id=%s FOR UPDATE", (order_id,))
        row = cur.fetchone()
        if not row:
            return jsonify({'error': 'Not found'}), 404
        if row['status'] == 'CANCELLED':
            return jsonify({'status': 'CANCELLED'}), 200
        if row['status'] == 'PAID':
            return jsonify({'error': 'Cannot cancel a paid order'}), 409
        # release stock
        cur.execute("SELECT product_id, quantity FROM order_items WHERE order_id=%s", (order_id,))
        items = cur.fetchall()
        for it in items:
            try:
                requests.post(f"{PRODUCT_SERVICE_URL}/products/{it['product_id']}/release", json={'quantity': it['quantity']})
            except Exception:
                pass
        cur.execute("UPDATE orders SET status='CANCELLED' WHERE id=%s", (order_id,))
        conn.commit()
    finally:
        conn.close()
    _emit_event('order_cancelled', {'orderId': order_id})
    return jsonify({'id': order_id, 'status': 'CANCELLED'}), 200


@app.route('/api/v1/orders/<order_id>/pay', methods=['POST'])
@require_auth
def pay_order(order_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute("SELECT status, user_id, total_amount FROM orders WHERE id=%s FOR UPDATE", (order_id,))
        row = cur.fetchone()
        if not row:
            return jsonify({'error': 'Not found'}), 404
        if row['status'] == 'CANCELLED':
            return jsonify({'error': 'Cannot pay a cancelled order'}), 409
        if row['status'] == 'PAID':
            return jsonify({'id': order_id, 'status': 'PAID'}), 200
        cur.execute("UPDATE orders SET status='PAID' WHERE id=%s", (order_id,))
        conn.commit()
        user_id = row['user_id']
        total_amount = float(row['total_amount'])
    finally:
        conn.close()
    _emit_event('order_paid', {'orderId': order_id, 'userId': user_id, 'totalAmount': total_amount})
    return jsonify({'id': order_id, 'status': 'PAID'}), 200


if __name__ == '__main__':
    import signal
    def _graceful(signum, frame):
        if _coverage:
            try:
                cov = _coverage.Coverage.current()
                if cov is not None:
                    cov.stop(); cov.save()
            except Exception:
                pass
        raise SystemExit(0)
    signal.signal(signal.SIGTERM, _graceful)
    signal.signal(signal.SIGINT, _graceful)
    port = int(os.environ.get('FLASK_RUN_PORT', 8080))
    app.run(host='0.0.0.0', port=port, debug=False, use_reloader=False)
