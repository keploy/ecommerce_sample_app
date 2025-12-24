import atexit
import datetime
import subprocess
import time
import uuid

import jwt
import requests

# Configuration
USER_SERVICE_URL = "http://localhost:8082/api/v1"
PRODUCT_SERVICE_URL = "http://localhost:8081/api/v1"
ORDER_SERVICE_URL = "http://localhost:8080/api/v1"
JWT_SECRET = "dev-secret-change-me"
JWT_ALG = "HS256"

# Port Forwarding Processes
pf_processes = []

def start_port_forward(service_name, local_port, remote_port):
    """Start port forwarding for a service"""
    print(f"Starting port forwarding for {service_name}...")
    
    # Check if port is already in use (might already be forwarded)
    try:
        import socket
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        result = sock.connect_ex(('localhost', local_port))
        sock.close()
        if result == 0:
            print(f"Port {local_port} already in use - assuming port-forward already active for {service_name}")
            return "already_active"  # Return a marker instead of None
    except Exception:
        pass
    
    try:
        cmd = ["kubectl", "port-forward", f"deployment/{service_name}", f"{local_port}:{remote_port}"]
        process = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(2)  # Give it more time to establish
        
        if process.poll() is not None:
            print(f"Port forwarding for {service_name} failed to start (process exited).")
            return None
        else:
            print(f"Port forwarding for {service_name} started (PID: {process.pid}).")
            return process
    except Exception as e:
        print(f"Error starting port forwarding for {service_name}: {e}")
        return None

def start_all_port_forwards():
    """Start port forwarding for all services"""
    global pf_processes
    pf_processes.append(start_port_forward("user-service", 8082, 8082))
    pf_processes.append(start_port_forward("product-service", 8081, 8081))
    pf_processes.append(start_port_forward("order-service", 8080, 8080))
    time.sleep(2)  # Give all port forwards time to establish

def stop_all_port_forwards():
    """Stop all port forwarding processes"""
    global pf_processes
    print("\nStopping port forwarding...")
    for process in pf_processes:
        if process and process != "already_active":
            try:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
            except Exception:
                pass
    print("Port forwarding stopped.")
    pf_processes = []

def generate_token(user_id="test-user"):
    payload = {
        "sub": user_id,
        "user_id": user_id, 
        "exp": datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(hours=1)
    }
    token = jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALG)
    return token

def get_headers(user_id="test-user"):
    token = generate_token(user_id)
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

def wait_delay(seconds=2):
    print(f"Waiting {seconds} seconds...")
    time.sleep(seconds)

# User Service Functions

def get_user(user_id):
    """Get a user by ID"""
    url = f"{USER_SERVICE_URL}/users/{user_id}"
    try:
        response = requests.get(url, headers=get_headers(user_id))
        if response.status_code == 200:
            return response.json()
        return None
    except Exception as e:
        print(f"Error getting user: {e}")
        return None

def create_user(username, email, password="testpass123"):
    """Create a new user"""
    url = f"{USER_SERVICE_URL}/users"
    payload = {
        "username": username,
        "email": email,
        "password": password
    }
    try:
        response = requests.post(url, json=payload, headers=get_headers(), timeout=10)
        if response.status_code == 201:
            user_data = response.json()
            print(f"Created user: {user_data.get('id')} ({username})")
            return user_data
        elif response.status_code == 500:
            # Database error or other server error
            error_msg = response.text[:200] if response.text else "Unknown error"
            print(f"User creation failed with server error (500): {error_msg}")
            return None
        else:
            print(f"Failed to create user: {response.status_code} - {response.text[:200]}")
            return None
    except Exception as e:
        print(f"Error creating user: {e}")
        return None

def ensure_user(base_username="test-user", max_attempts=3):
    """Ensure a user exists by creating one with a unique identifier"""
    # Always create a unique user to avoid conflicts
    for attempt in range(max_attempts):
        unique_suffix = uuid.uuid4().hex[:8]
        username = f"{base_username}-{unique_suffix}"
        email = f"{username}@example.com"
        
        user_data = create_user(username, email)
        if user_data:
            return user_data.get('id')
        
        # Wait a bit before retrying
        if attempt < max_attempts - 1:
            wait_delay(1)
    
    # If still failing, return None and let the caller handle it
    print(f"ERROR: Could not create user after {max_attempts} attempts")
    return None

def create_address(user_id, line1="123 Main St", city="Test City", state="TS", postal_code="12345", country="US"):
    """Create an address for a user"""
    url = f"{USER_SERVICE_URL}/users/{user_id}/addresses"
    payload = {
        "line1": line1,
        "city": city,
        "state": state,
        "postal_code": postal_code,
        "country": country,
        "is_default": True
    }
    try:
        response = requests.post(url, json=payload, headers=get_headers(user_id))
        if response.status_code == 201:
            addr_data = response.json()
            print(f"Created address: {addr_data.get('id')}")
            return addr_data.get('id')
        else:
            print(f"Failed to create address: {response.status_code} - {response.text}")
            return None
    except Exception as e:
        print(f"Error creating address: {e}")
        return None

# Product Service Functions

def list_products():
    """List all products"""
    url = f"{PRODUCT_SERVICE_URL}/products"
    try:
        response = requests.get(url, headers=get_headers())
        if response.status_code == 200:
            return response.json()
        return []
    except Exception as e:
        print(f"Error listing products: {e}")
        return []

def get_product(product_id):
    """Get a product by ID"""
    url = f"{PRODUCT_SERVICE_URL}/products/{product_id}"
    try:
        response = requests.get(url, headers=get_headers())
        if response.status_code == 200:
            return response.json()
        return None
    except Exception as e:
        print(f"Error getting product: {e}")
        return None

def create_product(name="Test Product", description="A test product", price=99.99, stock=100):
    """Create a new product"""
    url = f"{PRODUCT_SERVICE_URL}/products"
    payload = {
        "name": name,
        "description": description,
        "price": price,
        "stock": stock
    }
    try:
        response = requests.post(url, json=payload, headers=get_headers())
        if response.status_code == 201:
            product_data = response.json()
            print(f"Created product: {product_data.get('id')} ({name})")
            return product_data.get('id')
        else:
            print(f"Failed to create product: {response.status_code} - {response.text}")
            return None
    except Exception as e:
        print(f"Error creating product: {e}")
        return None

def ensure_product():
    """Ensure at least one product exists, create if not"""
    products = list_products()
    if products and len(products) > 0:
        product_id = products[0].get('id')
        print(f"Using existing product: {product_id}")
        return product_id
    
    # Create a test product
    product_id = create_product(name="Test Laptop", description="A test laptop for orders", price=999.99, stock=50)
    if product_id:
        return product_id
    
    print("Warning: Could not create product")
    return None

# Order Service Functions

def create_order(user_id, items, shipping_address_id=None):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders"
    payload = {
        "userId": user_id,
        "items": items
    }
    if shipping_address_id:
        payload["shippingAddressId"] = shipping_address_id
    
    # Idempotency key for safety
    headers = get_headers(user_id)
    headers["Idempotency-Key"] = str(uuid.uuid4())

    print(f"Creating order for user {user_id} with items {items}...")
    try:
        response = requests.post(url, json=payload, headers=headers)
        print(f"Status: {response.status_code}")
        if response.status_code == 201:
            order_data = response.json()
            print(f"Order created: {order_data.get('id')} (status: {order_data.get('status')})")
            return order_data
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None

def list_orders(user_id):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders"
    params = {"userId": user_id}
    print(f"Listing orders for user {user_id}...")
    try:
        response = requests.get(url, params=params, headers=get_headers(user_id))
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            orders = data.get('orders', [])
            print(f"Found {len(orders)} order(s)")
            return data
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None

def get_order(order_id, user_id):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders/{order_id}"
    print(f"Getting order {order_id}...")
    try:
        response = requests.get(url, headers=get_headers(user_id))
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            order_data = response.json()
            print(f"Order {order_id}: status={order_data.get('status')}, total=${order_data.get('total_amount')}")
            return order_data
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None

def get_order_details(order_id, user_id):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders/{order_id}/details"
    print(f"Getting order details for {order_id}...")
    try:
        response = requests.get(url, headers=get_headers(user_id))
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            details = response.json()
            print(f"Order details retrieved: {len(details.get('items', []))} item(s)")
            return details
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None

def pay_order(order_id, user_id):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders/{order_id}/pay"
    print(f"Paying order {order_id}...")
    try:
        response = requests.post(url, headers=get_headers(user_id))
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            result = response.json()
            print(f"Order {order_id} paid successfully (status: {result.get('status')})")
            return result
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None

def cancel_order(order_id, user_id):
    wait_delay(1)
    url = f"{ORDER_SERVICE_URL}/orders/{order_id}/cancel"
    print(f"Cancelling order {order_id}...")
    try:
        response = requests.post(url, headers=get_headers(user_id))
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            result = response.json()
            print(f"Order {order_id} cancelled successfully (status: {result.get('status')})")
            return result
        elif response.status_code == 409:
            print(f"Cannot cancel order (likely already paid): {response.text}")
            return None
        else:
            print(f"Response: {response.text}")
            return None
    except Exception as e:
        print(f"Request failed: {e}")
        return None


def ensure_migrations():
    """Ensure database migrations are run for all services"""
    print("--- PRE-FLIGHT: Ensuring database migrations ---")
    services = ["user-service", "product-service", "order-service"]
    for service in services:
        try:
            result = subprocess.run(
                ["kubectl", "exec", f"deployment/{service}", "--", "python3", "migrate.py"],
                capture_output=True,
                text=True,
                timeout=30
            )
            if result.returncode == 0:
                print(f"✓ Migrations OK for {service}")
            else:
                print(f"⚠ Migration warning for {service} (might already be applied)")
        except Exception as e:
            print(f"⚠ Could not check migrations for {service}: {str(e)[:50]}")
    print()

def main():
    atexit.register(stop_all_port_forwards)
    start_all_port_forwards()

    print("\n" + "="*60)
    print("E-COMMERCE ORDER FLOW TEST")
    print("="*60 + "\n")

    # Ensure migrations are run before starting
    ensure_migrations()
    wait_delay(2)  # Give migrations time to complete

    # Setup: Ensure user exists
    print("--- SETUP: User ---")
    user_id = ensure_user("test-user")
    if not user_id:
        print("ERROR: Could not ensure user exists. Aborting.")
        print("Hint: Check if database migrations completed successfully.")
        return
    print(f"Using user ID: {user_id}\n")
    wait_delay(1)

    # Setup: Ensure product exists
    print("--- SETUP: Product ---")
    product_id = ensure_product()
    if not product_id:
        print("ERROR: Could not ensure product exists. Aborting.")
        return
    print(f"Using product ID: {product_id}\n")
    wait_delay(1)

    # Setup: Create address (optional, order service will use default if not provided)
    print("--- SETUP: Address (optional) ---")
    address_id = create_address(user_id)
    if address_id:
        print(f"Using address ID: {address_id}\n")
    else:
        print("No address created (order service will use default if available)\n")
    wait_delay(1)

    # Test Order Flow
    print("--- ORDER FLOW TEST ---\n")

    # 1. Create Order
    items = [{"productId": product_id, "quantity": 1}]
    order = create_order(user_id, items, address_id)
    
    if not order:
        print("\nERROR: Failed to create order. Cannot proceed with flow.")
        # Still try to list orders
        list_orders(user_id)
        return
    
    order_id = order.get('id')
    print(f"Created order with ID: {order_id}\n")

    # 2. Get Order
    get_order(order_id, user_id)
    print()

    # 3. Get Order Details
    get_order_details(order_id, user_id)
    print()

    # 4. Pay Order
    pay_result = pay_order(order_id, user_id)
    print()

    # 5. Verify status changed to PAID
    if pay_result:
        get_order(order_id, user_id)
        print()

        # 6. Try to Cancel Order (Should fail if PAID)
        cancel_order(order_id, user_id)
        print()
    else:
        print("Skipping cancel test (order not paid)\n")

    # 7. List Orders for user
    list_orders(user_id)
    print()

    print("="*60)
    print("TEST COMPLETE")
    print("="*60)

if __name__ == "__main__":
    main()
