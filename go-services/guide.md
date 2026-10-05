install keploy 

curl --silent -O -L https://keploy.io/ent/install.sh && source install.sh

Local setup

currently the token expiration is set to 10 seconds in the config. so we need to change it to a higher value if freezeTime is not being used.

For recording, run

```bash
keploy record -c "docker compose up" --container-name="order_service" --build-delay 120 --path="./order_service" --config-path="./order_service"
```

wait for 120 seconds

try checking whether ca certificates is installed otherwise mysql mocks won't be recorded

```bash
order_service    | NODE_EXTRA_CA_CERTS is set to: /tmp/ca.crt
order_service    | REQUESTS_CA_BUNDLE is set to: /tmp/ca.crt
order_service    | Setup successful
```

and then run the following command to record the test cases:

```bash
chmod +x test_order_service.sh
./test_order_service.sh
```

this will record all the test which you can find in the `order_service/keploy` folder.

considering the token expiration is set to 10 seconds, you then need to change the `order_service/Dockerfile` to use the freezeTime agent which is currently commented out.

then you need to rebuild the order service container by running the following command:

```bash
docker build -f order_service/Dockerfile -t order-service .
```

then you can run the following command to start the test mode:

```bash
keploy test -c "docker compose up" --container-name="order_service" --delay 50 --path="./order_service" --config-path="./order_service" -t test-set-0 --freezeTime
```

Now you can run the dynamic dedup for these tests, because some of the tests that was recorded was similar to each other.

for that you first need to build it using cover flag, the code for that is commented out in the `order_service/Dockerfile`. uncommment it and build the container again.

you can increase the expiration time to 100 seconds to make sure that the tests do not fail

```bash
docker build -f order_service/Dockerfile -t order-service .
```

record again if you have increased the expiration time and then run the test command with dedup flag. 

```bash
keploy test -c "docker compose up" --container-name="order_service" --delay 50 --path="./order_service" --config-path="./order_service" -t test-set-0 --dedup
```

This will dedup the tests and it will generate the `dedupData.yaml` file which will have all the lines that was executed in the source code for every test case that got replayed.

now to see which all tests are marked as duplicate you can run the following command:

```bash
keploy dedup
```

k8s setup



first set up a new cluster

```bash
kind delete cluster
kind create cluster --config kind-config.yaml
```

run the following command to load the images into the cluster:

```bash
sudo kind load docker-image apigateway:latest
sudo kind load docker-image order-service:latest
sudo kind load docker-image product-service:latest
sudo kind load docker-image user-service:latest
```

then run the following command to deploy the services:

```bash
kubectl apply -f ./k8s

```

forward the port after the pods are running 

```bash 
chmod +x port-forward.sh
./port-forward.sh
```

then you can start recording from the dashboard wait for the pods to be running and then run the following command to record the test cases: 

```bash 
chmod +x test_order_service.sh
./test_order_service.sh
```

this will record 11 test cases because rest gets marked as duplicate by static dedup. 

stop recording

and start test mode

some test will fail because of noise. Run it again, noise filteration will work and now the tests will pass. 


