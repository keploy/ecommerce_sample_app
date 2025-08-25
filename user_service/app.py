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
            "INSERT INTO users (id, username, email, password_hash) VALUES (%s, %s, %s, %s)",
            (user_id, username, email, password_hash)
        )
        conn.commit()
    except mysql.connector.Error as err:
        conn.rollback()
        return jsonify({'error': f'Failed to create user: {err}'}), 500
    finally:
        cursor.close()
        conn.close()

    return jsonify({'id': user_id, 'username': username, 'email': email}), 201


@app.route('/api/v1/users/<string:user_id>', methods=['GET'])
def get_user(user_id):
    conn = get_db_connection()
    if not conn:
        return jsonify({'error': 'Database connection failed'}), 500

    cursor = conn.cursor(dictionary=True)
    cursor.execute("SELECT id, username, email, created_at FROM users WHERE id = %s", (user_id,))
    user = cursor.fetchone()
    cursor.close()
    conn.close()

    if user:
        return jsonify(user), 200
    else:
        return jsonify({'error': 'User not found'}), 404


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
