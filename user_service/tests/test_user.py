import os
import sys
import jwt
import pytest

from unittest.mock import patch, MagicMock
from werkzeug.security import generate_password_hash

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import app as user_app


@pytest.fixture
def client():
    user_app.app.config["TESTING"] = True
    with user_app.app.test_client() as client:
        yield client


@pytest.fixture
def auth_headers():
    token = jwt.encode(
        {"sub": "test-user-123"},
        user_app.JWT_SECRET,
        algorithm=user_app.JWT_ALG
    )
    return {"Authorization": f"Bearer {token}"}


def mock_connection(dictionary=False):
    conn = MagicMock()
    cursor = MagicMock()
    conn.cursor.return_value = cursor
    return conn, cursor


def test_login_missing_fields(client):
    response = client.post("/api/v1/login", json={})

    assert response.status_code == 400
    assert response.json["error"] == "Missing required fields"


def test_login_invalid_credentials(client):
    conn, cursor = mock_connection()
    cursor.fetchone.return_value = None

    with patch.object(user_app, "get_db_connection", return_value=conn):
        response = client.post(
            "/api/v1/login",
            json={"username": "unknown", "password": "wrong123"}
        )

    assert response.status_code == 401
    assert response.json["error"] == "Invalid credentials"


def test_login_success(client):
    password_hash = generate_password_hash("password123")

    conn, cursor = mock_connection()
    cursor.fetchone.return_value = {
        "id": "user-123",
        "username": "dinesh",
        "email": "dinesh@example.com",
        "password_hash": password_hash
    }

    with patch.object(user_app, "get_db_connection", return_value=conn):
        response = client.post(
            "/api/v1/login",
            json={"username": "dinesh", "password": "password123"}
        )

    assert response.status_code == 200
    assert response.json["username"] == "dinesh"
    assert "token" in response.json


def test_create_user_without_token(client):
    response = client.post(
        "/api/v1/users",
        json={
            "username": "dinesh",
            "email": "dinesh@example.com",
            "password": "password123"
        }
    )

    assert response.status_code == 401


def test_create_user_invalid_username(client, auth_headers):
    response = client.post(
        "/api/v1/users",
        headers=auth_headers,
        json={
            "username": "ab",
            "email": "dinesh@example.com",
            "password": "password123"
        }
    )

    assert response.status_code == 400
    assert response.json["error"] == "username must be 3-50 chars"


def test_create_user_missing_fields(client, auth_headers):
    response = client.post(
        "/api/v1/users",
        headers=auth_headers,
        json={"username": "dinesh"}
    )

    assert response.status_code == 400


def test_create_user_database_unavailable(client, auth_headers):
    with patch.object(user_app, "get_db_connection", return_value=None):
        response = client.post(
            "/api/v1/users",
            headers=auth_headers,
            json={
                "username": "dinesh",
                "email": "dinesh@example.com",
                "password": "password123"
            }
        )

    assert response.status_code == 500
    assert response.json["error"] == "Database connection failed"


def test_get_user_database_unavailable(client, auth_headers):
    with patch.object(user_app, "get_db_connection", return_value=None):
        response = client.get(
            "/api/v1/users/user-123",
            headers=auth_headers
        )

    assert response.status_code == 500


def test_create_address_missing_fields(client, auth_headers):
    response = client.post(
        "/api/v1/users/user-123/addresses",
        headers=auth_headers,
        json={"city": "Chennai"}
    )

    assert response.status_code == 400


def test_update_address_no_fields(client, auth_headers):
    response = client.put(
        "/api/v1/users/user-123/addresses/address-123",
        headers=auth_headers,
        json={}
    )

    assert response.status_code == 400
    assert response.json["error"] == "no fields to update"


def test_delete_address_database_unavailable(client, auth_headers):
    with patch.object(user_app, "get_db_connection", return_value=None):
        response = client.delete(
            "/api/v1/users/user-123/addresses/address-123",
            headers=auth_headers
        )

    assert response.status_code == 500
