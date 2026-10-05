# Kubernetes Deployment Instructions

This directory contains Kubernetes manifests to deploy the ecommerce sample app to a local Kind cluster.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed.
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed.
- [Docker](https://docs.docker.com/get-docker/) installed.

## Deployment Steps

1.  **Create a Kind Cluster** (if you haven't already):
    ```bash
    kind create cluster --name ecommerce
    ```

2.  **Build Docker Images**:
    You need to build the images for the services locally.
    ```bash
    docker build -t user-service:latest ./user_service
    docker build -t product-service:latest ./product_service
    docker build -t order-service:latest ./order_service
    docker build -t apigateway:latest ./apigateway
    ```

3.  **Load Images into Kind**:
    Since the manifests use `imagePullPolicy: Never`, you must load the images into the Kind cluster nodes.
    ```bash
    kind load docker-image user-service:latest --name ecommerce
    kind load docker-image product-service:latest --name ecommerce
    kind load docker-image order-service:latest --name ecommerce
    kind load docker-image apigateway:latest --name ecommerce
    ```
    *Note: The `mysql:8.0` and `localstack/localstack:3.3` images will be pulled by Kind automatically if not present, or you can load them to speed up startup.*

4.  **Apply Manifests**:
    Apply the manifests in the following order (or all at once):
    ```bash
    kubectl apply -f k8s/
    ```

5.  **Access the Application**:
    The API Gateway is exposed via a NodePort service on port `30083`.
    To access it, you might need to port-forward if you are on Mac/Windows or depending on your Kind setup:
    ```bash
    kubectl port-forward service/apigateway 8083:8083
    ```
    Then access the API at `http://localhost:8083`.

## Troubleshooting

-   Check pod status: `kubectl get pods`
-   Check logs: `kubectl logs <pod-name>`
-   If pods are stuck in `ImagePullBackOff` or `ErrImageNeverPull`, ensure you have loaded the images into Kind as described in step 3.
