import os
import uuid
import json
import requests
import boto3
import mysql.connector
from flask import Flask, request, jsonify

app = Flask(__name__)

USER_SERVICE_URL = os.environ.get('USER_SERVICE_URL', 'http://localhost:8082/api/v1')
PRODUCT_SERVICE_URL = os.environ.get('PRODUCT_SERVICE_URL', 'http://localhost:8081/api/v1')

SQS_QUEUE_URL = os.environ.get('SQS_QUEUE_URL')
_endpoint = os.environ.get('AWS_ENDPOINT') or os.environ.get('AWS_ENDPOINT_URL')
# Workaround: when using a full QueueUrl (http://host:port/acc/queue), don't override endpoint_url,
# let boto3 use the QueueUrl host directly to avoid LocalStack query-protocol parsing issues.
endpoint_url = None if (SQS_QUEUE_URL and SQS_QUEUE_URL.startswith('http')) else _endpoint
sqs = boto3.client('sqs', region_name=os.environ.get('AWS_REGION', 'us-east-1'), endpoint_url=endpoint_url)


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
    if not SQS_QUEUE_URL:
        return
    try:
        body = { 'eventType': event_type, **payload }
        sqs.send_message(QueueUrl=SQS_QUEUE_URL, MessageBody=json.dumps(body))
    except Exception as e:
        print(f"Failed to send SQS message: {e}")


@app.route('/api/v1/orders', methods=['POST'])
def create_order():
    data = request.get_json()
    if not data or not all(k in data for k in ('userId', 'items')):
        return jsonify({'error': 'Missing required fields'}), 400

    user_id = data['userId']
    items = data['items']
    shipping_address = data.get('shippingAddress')
    idmp_key = request.headers.get('Idempotency-Key')

    ok, err = _validate_items(items)
    if not ok:
        return jsonify({'error': err}), 400

    # Fast-path idempotency before any external calls or reservations
    if idmp_key:
        conn_check = get_db_connection()
        if not conn_check:
            return jsonify({'error': 'Database connection failed'}), 500
        try:
            c = conn_check.cursor(dictionary=True)
            c.execute("SELECT id, status FROM orders WHERE idempotency_key=%s", (idmp_key,))
            existing = c.fetchone()
            if existing:
                return jsonify({'id': existing['id'], 'status': existing['status']}), 200
        finally:
            conn_check.close()

    try:
        user_response = requests.get(f"{USER_SERVICE_URL}/users/{user_id}")
        if user_response.status_code != 200:
            return jsonify({'error': 'Invalid user ID'}), 400
        user_json = user_response.json()
    except requests.exceptions.RequestException as e:
        return jsonify({'error': f'Could not connect to User Service: {e}'}), 503

    # If shippingAddress not provided, try using user's default address from User service
    if not shipping_address:
        try:
            addrs_resp = requests.get(f"{USER_SERVICE_URL}/users/{user_id}/addresses")
            if addrs_resp.status_code == 200:
                arr = addrs_resp.json()
                if isinstance(arr, list) and len(arr) > 0:
                    shipping_address = arr[0]
        except Exception:
            pass

    total_amount = 0
    for item in items:
        product_id = item['productId']
        try:
            product_response = requests.get(f"{PRODUCT_SERVICE_URL}/products/{product_id}")
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
                json={'quantity': item['quantity']}
            )
            if resp.status_code != 200:
                # release any previously reserved items
                for r in reserved:
                    try:
                        requests.post(f"{PRODUCT_SERVICE_URL}/products/{r['productId']}/release", json={'quantity': r['quantity']})
                    except Exception:
                        pass
                return jsonify({'error': 'Stock reservation failed'}), resp.status_code
            reserved.append({'productId': item['productId'], 'quantity': item['quantity']})
        except requests.exceptions.RequestException as e:
            for r in reserved:
                try:
                    requests.post(f"{PRODUCT_SERVICE_URL}/products/{r['productId']}/release", json={'quantity': r['quantity']})
                except Exception:
                    pass
            return jsonify({'error': f'Could not reserve stock: {e}'}), 503

    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500

    cursor = conn.cursor(dictionary=True)
    order_id = str(uuid.uuid4())
    try:
        cursor.execute(
            "INSERT INTO orders (id, user_id, status, idempotency_key, total_amount, shipping_address) VALUES (%s, %s, %s, %s, %s, %s)",
            (order_id, user_id, 'PENDING', idmp_key, round(total_amount, 2), json.dumps(shipping_address) if shipping_address else None)
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
def list_orders():
    # Filters: userId, status; pagination: limit, cursor(created_at, id)
    user_id = request.args.get('userId')
    status = request.args.get('status')
    limit = min(max(int(request.args.get('limit', 20)), 1), 100)
    cursor_token = request.args.get('cursor')

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
    if cursor_token:
        try:
            created_at_str, last_id = cursor_token.split('|', 1)
            where_clause = ("created_at < %s OR (created_at = %s AND id > %s)")
            sql += (" AND " if where else " WHERE ") + where_clause
            params.extend([created_at_str, created_at_str, last_id])
        except Exception:
            return jsonify({'error': 'Invalid cursor'}), 400
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
    if len(rows) > limit:
        last = rows[limit - 1]
        next_cursor = f"{last['created_at'].isoformat()}|{last['id']}"
        rows = rows[:limit]

    return jsonify({'orders': rows, 'nextCursor': next_cursor}), 200


@app.route('/api/v1/orders/<order_id>', methods=['GET'])
def get_order(order_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    try:
        cur = conn.cursor(dictionary=True)
        cur.execute("SELECT id, user_id, status, total_amount, shipping_address, created_at, updated_at FROM orders WHERE id=%s", (order_id,))
        order = cur.fetchone()
        if not order:
            return jsonify({'error': 'Not found'}), 404
        cur.execute("SELECT product_id, quantity, price FROM order_items WHERE order_id=%s", (order_id,))
        items = cur.fetchall()
        order['items'] = items
        # decode shipping_address JSON if present
        sa = order.get('shipping_address')
        if isinstance(sa, str) and sa:
            try:
                order['shipping_address'] = json.loads(sa)
            except Exception:
                pass
    finally:
        conn.close()
    return jsonify(order), 200


@app.route('/api/v1/orders/<order_id>/cancel', methods=['POST'])
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


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'ok': True}), 200


if __name__ == '__main__':
    port = int(os.environ.get('FLASK_RUN_PORT', 8080))
    app.run(host='0.0.0.0', port=port, debug=True)
