import unittest
from unittest.mock import Mock, patch

import app as gateway


class GatewayReadinessTests(unittest.TestCase):

    def setUp(self):
        self.client = gateway.app.test_client()

    @patch.object(gateway.requests, "get")
    def test_ready_when_all_services_are_healthy(self, mock_get):
        mock_get.return_value = Mock(status_code=200)

        response = self.client.get("/ready")

        self.assertEqual(response.status_code, 200)
        data = response.get_json()
        self.assertEqual(data["service"], "apigateway")
        self.assertEqual(data["status"], "ready")
        self.assertEqual(mock_get.call_count, 3)

    @patch.object(gateway.requests, "get")
    def test_not_ready_when_backend_returns_503(self, mock_get):
        mock_get.side_effect = [
            Mock(status_code=200),
            Mock(status_code=503),
            Mock(status_code=200),
        ]

        response = self.client.get("/ready")

        self.assertEqual(response.status_code, 503)
        self.assertEqual(response.get_json()["status"], "not_ready")

    @patch.object(gateway.requests, "get")
    def test_not_ready_when_backend_connection_fails(self, mock_get):
        mock_get.side_effect = [
            Mock(status_code=200),
            gateway.requests.ConnectionError("backend unavailable"),
            Mock(status_code=200),
        ]

        response = self.client.get("/ready")

        self.assertEqual(response.status_code, 503)
        self.assertEqual(response.get_json()["status"], "not_ready")


if __name__ == "__main__":
    unittest.main()
