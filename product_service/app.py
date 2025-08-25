import os
import uuid
import mysql.connector
from flask import Flask, jsonify, request

app = Flask(__name__)


def get_db_connection():
    try:
        conn = mysql.connector.connect(
            host=os.environ.get('DB_HOST', 'localhost'),
            user=os.environ.get('DB_USER', 'user'),
            password=os.environ.get('DB_PASSWORD', 'password'),
            database=os.environ.get('DB_NAME', 'product_db')
        )
        return conn
    except mysql.connector.Error as err:
        print(f"Error: {err}")
        return None


@app.route('/api/v1/products', methods=['GET'])
def get_products():
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor(dictionary=True)
    cursor.execute("SELECT id, name, description, price, stock FROM products")
    products = cursor.fetchall()
    cursor.close()
    conn.close()
    return jsonify(products), 200


@app.route('/api/v1/products/<string:product_id>', methods=['GET'])
def get_product(product_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor(dictionary=True)
    cursor.execute("SELECT id, name, description, price, stock FROM products WHERE id = %s", (product_id,))
    product = cursor.fetchone()
    cursor.close()
    conn.close()
    if product:
        return jsonify(product), 200
    else:
        return jsonify({'error': 'Product not found'}), 404


@app.route('/api/v1/products', methods=['POST'])
def create_product():
    data = request.get_json(silent=True) or {}
    required = ('name', 'price', 'stock')
    if not all(k in data for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    try:
        price = float(data['price'])
        stock = int(data['stock'])
        if price < 0 or stock < 0:
            return jsonify({'error': 'price and stock must be non-negative'}), 400
    except (ValueError, TypeError):
        return jsonify({'error': 'Invalid price or stock'}), 400
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor()
    pid = str(uuid.uuid4())
    try:
        cursor.execute(
            "INSERT INTO products (id, name, description, price, stock) VALUES (%s, %s, %s, %s, %s)",
            (pid, data['name'].strip(), data.get('description'), price, stock)
        )
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to create product: {err}'}), 500
    finally:
        cursor.close()
        conn.close()
    return jsonify({'id': pid}), 201


@app.route('/api/v1/products/<string:product_id>/reserve', methods=['POST'])
def reserve_stock(product_id):
    data = request.get_json(silent=True) or {}
    qty = int(data.get('quantity', 0))
    if qty <= 0:
        return jsonify({'error': 'quantity must be > 0'}), 400
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor()
    try:
        cursor.execute(
            "UPDATE products SET stock = stock - %s WHERE id = %s AND stock >= %s",
            (qty, product_id, qty)
        )
        if cursor.rowcount == 0:
            conn.rollback()
            return jsonify({'error': 'Insufficient stock or product not found'}), 409
        conn.commit()
        # fetch new stock
        cursor = conn.cursor(dictionary=True)
        cursor.execute("SELECT stock FROM products WHERE id=%s", (product_id,))
        row = cursor.fetchone()
        new_stock = row['stock'] if row else None
        return jsonify({'reserved': qty, 'stock': new_stock}), 200
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to reserve stock: {err}'}), 500
    finally:
        try:
            cursor.close()
        except Exception:
            pass
        conn.close()


@app.route('/api/v1/products/<string:product_id>/release', methods=['POST'])
def release_stock(product_id):
    data = request.get_json(silent=True) or {}
    qty = int(data.get('quantity', 0))
    if qty <= 0:
        return jsonify({'error': 'quantity must be > 0'}), 400
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor()
    try:
        cursor.execute(
            "UPDATE products SET stock = stock + %s WHERE id = %s",
            (qty, product_id)
        )
        if cursor.rowcount == 0:
            conn.rollback()
            return jsonify({'error': 'Product not found'}), 404
        conn.commit()
        cursor = conn.cursor(dictionary=True)
        cursor.execute("SELECT stock FROM products WHERE id=%s", (product_id,))
        row = cursor.fetchone()
        new_stock = row['stock'] if row else None
        return jsonify({'released': qty, 'stock': new_stock}), 200
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to release stock: {err}'}), 500
    finally:
        try:
            cursor.close()
        except Exception:
            pass
        conn.close()


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'ok': True}), 200


@app.route('/api/v1/products/<string:product_id>', methods=['PUT'])
def update_product(product_id):
    data = request.get_json(silent=True) or {}
    fields = {}
    if 'name' in data:
        fields['name'] = str(data['name']).strip()
    if 'description' in data:
        fields['description'] = data['description']
    if 'price' in data:
        try:
            price = float(data['price'])
            if price < 0:
                return jsonify({'error': 'price must be non-negative'}), 400
            fields['price'] = price
        except (ValueError, TypeError):
            return jsonify({'error': 'invalid price'}), 400
    if 'stock' in data:
        try:
            stock = int(data['stock'])
            if stock < 0:
                return jsonify({'error': 'stock must be non-negative'}), 400
            fields['stock'] = stock
        except (ValueError, TypeError):
            return jsonify({'error': 'invalid stock'}), 400
    if not fields:
        return jsonify({'error': 'no fields to update'}), 400
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    sets = ', '.join([f"{k}=%s" for k in fields.keys()])
    vals = list(fields.values()) + [product_id]
    cursor = conn.cursor()
    try:
        cursor.execute(f"UPDATE products SET {sets} WHERE id=%s", tuple(vals))
        if cursor.rowcount == 0:
            conn.rollback()
            return jsonify({'error': 'Product not found'}), 404
        conn.commit()
        return jsonify({'updated': True}), 200
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to update product: {err}'}), 500
    finally:
        cursor.close(); conn.close()


@app.route('/api/v1/products/<string:product_id>', methods=['DELETE'])
def delete_product(product_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor()
    try:
        cursor.execute("DELETE FROM products WHERE id=%s", (product_id,))
        if cursor.rowcount == 0:
            conn.rollback()
            return jsonify({'error': 'Product not found'}), 404
        conn.commit()
        return jsonify({'deleted': True}), 200
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to delete product: {err}'}), 500
    finally:
        cursor.close(); conn.close()


@app.route('/api/v1/products/search', methods=['GET'])
def search_products():
    q = (request.args.get('q') or '').strip()
    min_price = request.args.get('minPrice')
    max_price = request.args.get('maxPrice')
    clauses = []
    params = []
    if q:
        clauses.append("name LIKE ?")
        params.append(f"%{q}%")
    if min_price is not None:
        try:
            clauses.append("price >= ?"); params.append(float(min_price))
        except ValueError:
            return jsonify({'error': 'invalid minPrice'}), 400
    if max_price is not None:
        try:
            clauses.append("price <= ?"); params.append(float(max_price))
        except ValueError:
            return jsonify({'error': 'invalid maxPrice'}), 400
    where = (" WHERE " + " AND ".join(clauses)) if clauses else ""
    sql = "SELECT id, name, description, price, stock FROM products" + where
    conn = get_db_connection();
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    # mysql-connector uses %s placeholders, fix placeholders
    sql = sql.replace('?', '%s')
    cursor = conn.cursor(dictionary=True)
    cursor.execute(sql, tuple(params))
    rows = cursor.fetchall()
    cursor.close(); conn.close()
    return jsonify(rows), 200


if __name__ == '__main__':
    port = int(os.environ.get('FLASK_RUN_PORT', 8081))
    app.run(host='0.0.0.0', port=port, debug=True)
