#!/bin/bash
# Run this script to set up port forwarding for all services
# Keep this running while you run test_order_service.sh in another terminal

echo "Setting up port forwarding for all services..."
echo "Press Ctrl+C to stop all port forwards"
echo ""

# Trap Ctrl+C to kill all background jobs
trap 'kill $(jobs -p); exit' INT TERM

# Start port forwards in background
kubectl port-forward deployment/user-service 8082:8082 &
PID_USER=$!
echo "✓ User service forwarding on port 8082 (PID: $PID_USER)"

kubectl port-forward deployment/product-service 8081:8081 &
PID_PRODUCT=$!
echo "✓ Product service forwarding on port 8081 (PID: $PID_PRODUCT)"

kubectl port-forward deployment/order-service 8080:8080 &
PID_ORDER=$!
echo "✓ Order service forwarding on port 8080 (PID: $PID_ORDER)"

echo ""
echo "All port forwards are running. You can now run ./test_order_service.sh"
echo "Press Ctrl+C to stop all port forwards"

# Wait for all background jobs
wait

