import os
import signal
from flask import Flask, request, Response, jsonify
import requests

app = Flask(__name__)

USER_SERVICE_URL = os.environ.get('USER_SERVICE_URL', 'http://user_service:8082/api/v1')
PRODUCT_SERVICE_URL = os.environ.get('PRODUCT_SERVICE_URL', 'http://product_service:8081/api/v1')
ORDER_SERVICE_URL = os.environ.get('ORDER_SERVICE_URL', 'http://order_service:8080/api/v1')


def _forward_headers():
    # Forward only safe headers; include Authorization, Content-Type, Accept, Idempotency-Key
    headers = {}
    for h in ('Authorization', 'Content-Type', 'Accept', 'Idempotency-Key'):
        v = request.headers.get(h)
        if v:
            headers[h] = v
    return headers


def _proxy(base_url: str, subpath: str | None = None):
    url = base_url if not subpath else f"{base_url.rstrip('/')}/{subpath}"
    method = request.method

    data = None
    json_body = None
    if request.method in ('POST', 'PUT', 'PATCH'):
        # Prefer JSON if provided
        json_body = request.get_json(silent=True)
        if json_body is None:
            data = request.get_data()

    try:
        resp = requests.request(
            method,
            url,
            params=request.args,
            json=json_body,
            data=data,
            headers=_forward_headers(),
            timeout=15,
        )
    except requests.RequestException as e:
        return jsonify({'error': f'Upstream unavailable: {e}'}), 502

    # Build Flask Response with upstream status and content-type
    headers = {}
    if 'Content-Type' in resp.headers:
        headers['Content-Type'] = resp.headers['Content-Type']
    return Response(resp.content, status=resp.status_code, headers=headers)


# Users
@app.route('/api/v1/login', methods=['POST'])
def gw_login():
    # Forward login to user-service without auth requirement
    return _proxy(USER_SERVICE_URL, 'login')

@app.route('/api/v1/users', methods=['GET', 'POST'])
def gw_users_root():
    return _proxy(USER_SERVICE_URL)


@app.route('/api/v1/users/<path:subpath>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH'])
def gw_users(subpath):
    return _proxy(USER_SERVICE_URL, subpath)


# Products
@app.route('/api/v1/products', methods=['GET', 'POST'])
def gw_products_root():
    return _proxy(PRODUCT_SERVICE_URL)


@app.route('/api/v1/products/<path:subpath>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH'])
def gw_products(subpath):
    return _proxy(PRODUCT_SERVICE_URL, subpath)


# Orders
@app.route('/api/v1/orders', methods=['GET', 'POST'])
def gw_orders_root():
    return _proxy(ORDER_SERVICE_URL)


@app.route('/api/v1/orders/<path:subpath>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH'])
def gw_orders(subpath):
    return _proxy(ORDER_SERVICE_URL, subpath)


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'ok': True}), 200


if __name__ == '__main__':
    # Ensure we exit quickly on SIGTERM/SIGINT during docker stop
    def _graceful_exit(signum, frame):
        # Let Flask/Werkzeug shutdown
        os._exit(0)
    signal.signal(signal.SIGTERM, _graceful_exit)
    signal.signal(signal.SIGINT, _graceful_exit)
    port = int(os.environ.get('FLASK_RUN_PORT', 8083))
    app.run(host='0.0.0.0', port=port)
