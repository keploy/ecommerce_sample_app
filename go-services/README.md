in order to run keploy record mode for order service, run the following command:

```bash
 keploy record -c "docker compose up" --container-name="order_service" --build-delay 40 --path="./order_service" --config-path="./order_service"
```

wait for it to start the services

then run the `test_api_script.py` file to run the tests.

```bash
python3 -m venv venv
source venv/bin/activate
pip install requests
python3 test_api_script.py
```

this will record all the test cases and store it in `order_service/keploy` folder.

