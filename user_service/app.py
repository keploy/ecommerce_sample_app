import os
import uuid
import mysql.connector
from flask import Flask, request, jsonify
from werkzeug.security import generate_password_hash, check_password_hash

app = Flask(__name__)


def get_db_connection():
    try:
        conn = mysql.connector.connect(
            host=os.environ.get('DB_HOST', 'localhost'),
            user=os.environ.get('DB_USER', 'user'),
            password=os.environ.get('DB_PASSWORD', 'password'),
            database=os.environ.get('DB_NAME', 'user_db')
        )
        return conn
    except mysql.connector.Error as err:
        print(f"Error: {err}")
        return None


@app.route('/api/v1/users', methods=['POST'])
def create_user():
    data = request.get_json(silent=True) or {}
    if not all(k in data for k in ('username', 'email', 'password')):
        return jsonify({'error': 'Missing required fields'}), 400
    username = str(data['username']).strip()
    email = str(data['email']).strip()
    password = str(data['password'])
    phone = str(data.get('phone', '')).strip() or None
    if len(username) < 3 or len(username) > 50:
        return jsonify({'error': 'username must be 3-50 chars'}), 400
    if '@' not in email or len(email) > 255:
        return jsonify({'error': 'invalid email'}), 400
    if len(password) < 6:
        return jsonify({'error': 'password too short'}), 400

    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500

    cursor = conn.cursor()
    user_id = str(uuid.uuid4())
    try:
        password_hash = generate_password_hash(password)
        cursor.execute(
            "INSERT INTO users (id, username, email, password_hash, phone) VALUES (%s, %s, %s, %s, %s)",
            (user_id, username, email, password_hash, phone)
        )
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to create user: {err}'}), 500
    finally:
        cursor.close()
        conn.close()

    return jsonify({'id': user_id, 'username': username, 'email': email, 'phone': phone}), 201


@app.route('/api/v1/users/<string:user_id>', methods=['GET'])
def get_user(user_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500

    cursor = conn.cursor(dictionary=True)
    cursor.execute("SELECT id, username, email, phone, created_at FROM users WHERE id = %s", (user_id,))
    user = cursor.fetchone()
    if user:
        cursor.execute("SELECT id, line1, line2, city, state, postal_code, country, phone, is_default FROM addresses WHERE user_id=%s ORDER BY is_default DESC, created_at DESC", (user_id,))
        user['addresses'] = cursor.fetchall()
    cursor.close(); conn.close()

    if user:
        return jsonify(user), 200
    else:
        return jsonify({'error': 'User not found'}), 404


# Addresses CRUD
@app.route('/api/v1/users/<string:user_id>/addresses', methods=['POST'])
def create_address(user_id):
    data = request.get_json(silent=True) or {}
    required = ('line1', 'city', 'state', 'postal_code', 'country')
    if not all(k in data for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    addr_id = str(uuid.uuid4())
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cur = conn.cursor()
    try:
        cur.execute("SELECT id FROM users WHERE id=%s", (user_id,))
        if not cur.fetchone():
            return jsonify({'error': 'User not found'}), 404
        is_default = 1 if data.get('is_default') else 0
        cur.execute(
            "INSERT INTO addresses (id, user_id, line1, line2, city, state, postal_code, country, phone, is_default) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)",
            (addr_id, user_id, data['line1'], data.get('line2'), data['city'], data['state'], data['postal_code'], data['country'], data.get('phone'), is_default)
        )
        if is_default:
            cur.execute("UPDATE addresses SET is_default=0 WHERE user_id=%s AND id<>%s", (user_id, addr_id))
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback(); return jsonify({'error': f'Failed to create address: {err}'}), 500
    finally:
        cur.close(); conn.close()
    return jsonify({'id': addr_id}), 201


@app.route('/api/v1/users/<string:user_id>/addresses', methods=['GET'])
def list_addresses(user_id):
    conn = get_db_connection();
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cur = conn.cursor(dictionary=True)
    cur.execute("SELECT id FROM users WHERE id=%s", (user_id,))
    if not cur.fetchone():
        cur.close(); conn.close(); return jsonify({'error': 'User not found'}), 404
    cur.execute("SELECT id, line1, line2, city, state, postal_code, country, phone, is_default FROM addresses WHERE user_id=%s ORDER BY is_default DESC, created_at DESC", (user_id,))
    rows = cur.fetchall(); cur.close(); conn.close()
    return jsonify(rows), 200


@app.route('/api/v1/users/<string:user_id>/addresses/<string:addr_id>', methods=['PUT'])
def update_address(user_id, addr_id):
    data = request.get_json(silent=True) or {}
    fields = {}
    for k in ('line1','line2','city','state','postal_code','country','phone'):
        if k in data:
            fields[k] = data[k]
    if 'is_default' in data:
        fields['is_default'] = 1 if data['is_default'] else 0
    if not fields:
        return jsonify({'error': 'no fields to update'}), 400
    sets = ', '.join([f"{k}=%s" for k in fields.keys()])
    vals = list(fields.values()) + [user_id, addr_id]
    conn = get_db_connection();
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cur = conn.cursor()
    try:
        cur.execute(f"UPDATE addresses SET {sets} WHERE user_id=%s AND id=%s", tuple(vals))
        if cur.rowcount == 0:
            conn.rollback(); return jsonify({'error': 'Address not found'}), 404
        if 'is_default' in fields and fields['is_default'] == 1:
            cur.execute("UPDATE addresses SET is_default=0 WHERE user_id=%s AND id<>%s", (user_id, addr_id))
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback(); return jsonify({'error': f'Failed to update address: {err}'}), 500
    finally:
        cur.close(); conn.close()
    return jsonify({'updated': True}), 200


@app.route('/api/v1/users/<string:user_id>/addresses/<string:addr_id>', methods=['DELETE'])
def delete_address(user_id, addr_id):
    conn = get_db_connection();
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cur = conn.cursor()
    try:
        cur.execute("DELETE FROM addresses WHERE user_id=%s AND id=%s", (user_id, addr_id))
        if cur.rowcount == 0:
            conn.rollback(); return jsonify({'error': 'Address not found'}), 404
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback(); return jsonify({'error': f'Failed to delete address: {err}'}), 500
    finally:
        cur.close(); conn.close()
    return jsonify({'deleted': True}), 200


@app.route('/api/v1/login', methods=['POST'])
def login():
    data = request.get_json() or {}
    if not all(k in data for k in ('username', 'password')):
        return jsonify({'error': 'Missing required fields'}), 400
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500
    cursor = conn.cursor(dictionary=True)
    cursor.execute("SELECT id, username, email, password_hash FROM users WHERE username=%s", (data['username'],))
    row = cursor.fetchone()
    cursor.close()
    conn.close()
    if not row or not check_password_hash(row['password_hash'], data['password']):
        return jsonify({'error': 'Invalid credentials'}), 401
    return jsonify({'id': row['id'], 'username': row['username'], 'email': row['email']}), 200


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'ok': True}), 200


if __name__ == '__main__':
    port = int(os.environ.get('FLASK_RUN_PORT', 8082))
    app.run(host='0.0.0.0', port=port, debug=True)
