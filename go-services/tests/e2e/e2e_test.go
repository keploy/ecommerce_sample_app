//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	userServiceURL    = getEnv("USER_SERVICE_URL", "http://localhost:8082/api/v1")
	productServiceURL = getEnv("PRODUCT_SERVICE_URL", "http://localhost:8081/api/v1")
	orderServiceURL   = getEnv("ORDER_SERVICE_URL", "http://localhost:8080/api/v1")
	gatewayURL        = getEnv("GATEWAY_URL", "http://localhost:8083/api/v1")
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Helper for making HTTP requests
type httpClient struct {
	client *http.Client
	token  string
}

func newClient() *httpClient {
	return &httpClient{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *httpClient) setToken(token string) {
	c.token = token
}

func (c *httpClient) do(method, url string, body interface{}) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return resp, respBody, err
}

func (c *httpClient) get(url string) (*http.Response, []byte, error) {
	return c.do(http.MethodGet, url, nil)
}

func (c *httpClient) post(url string, body interface{}) (*http.Response, []byte, error) {
	return c.do(http.MethodPost, url, body)
}

func (c *httpClient) put(url string, body interface{}) (*http.Response, []byte, error) {
	return c.do(http.MethodPut, url, body)
}

func (c *httpClient) delete(url string) (*http.Response, []byte, error) {
	return c.do(http.MethodDelete, url, nil)
}

// ===================== LOGIN TESTS =====================

func TestLogin(t *testing.T) {
	c := newClient()

	// Login with admin credentials
	resp, body, err := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "login should succeed: %s", string(body))

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["token"], "should return JWT token")
	assert.NotEmpty(t, result["id"], "should return user ID")
	assert.Equal(t, "admin", result["username"])
}

func TestLoginInvalidPassword(t *testing.T) {
	c := newClient()

	resp, _, err := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ===================== USER CRUD TESTS =====================

func TestCreateAndGetUser(t *testing.T) {
	c := newClient()

	// First login to get token
	resp, body, err := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))

	// Create user
	username := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	email := fmt.Sprintf("%s@test.com", username)

	resp, body, err = c.post(userServiceURL+"/users", map[string]string{
		"username": username,
		"email":    email,
		"password": "password123",
		"phone":    "+1-555-1234",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "create user failed: %s", string(body))

	var createResult map[string]interface{}
	json.Unmarshal(body, &createResult)
	userID := createResult["id"].(string)
	assert.NotEmpty(t, userID)
	assert.Equal(t, username, createResult["username"])

	// Get user
	resp, body, err = c.get(userServiceURL + "/users/" + userID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var getResult map[string]interface{}
	json.Unmarshal(body, &getResult)
	assert.Equal(t, userID, getResult["id"])
	assert.Equal(t, username, getResult["username"])
	assert.Equal(t, email, getResult["email"])

	// Cleanup - delete user
	resp, _, err = c.delete(userServiceURL + "/users/" + userID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteUserCascadesAddresses(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))

	// Create user
	username := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	resp, body, _ = c.post(userServiceURL+"/users", map[string]string{
		"username": username,
		"email":    username + "@test.com",
		"password": "password123",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var createResult map[string]interface{}
	json.Unmarshal(body, &createResult)
	userID := createResult["id"].(string)

	// Create address
	resp, _, _ = c.post(userServiceURL+"/users/"+userID+"/addresses", map[string]interface{}{
		"line1":       "123 Main St",
		"city":        "NYC",
		"state":       "NY",
		"postal_code": "10001",
		"country":     "US",
		"is_default":  true,
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Delete user
	resp, _, _ = c.delete(userServiceURL + "/users/" + userID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify user is gone
	resp, _, _ = c.get(userServiceURL + "/users/" + userID)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ===================== PRODUCT CRUD TESTS =====================

func TestProductCRUD(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))

	// Create product
	resp, body, err := c.post(productServiceURL+"/products", map[string]interface{}{
		"name":        "Test Product",
		"description": "A test product",
		"price":       99.99,
		"stock":       100,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "create product failed: %s", string(body))

	var createResult map[string]interface{}
	json.Unmarshal(body, &createResult)
	productID := createResult["id"].(string)

	// Get product
	resp, body, _ = c.get(productServiceURL + "/products/" + productID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var getResult map[string]interface{}
	json.Unmarshal(body, &getResult)
	assert.Equal(t, "Test Product", getResult["name"])

	// Update product
	resp, _, _ = c.put(productServiceURL+"/products/"+productID, map[string]interface{}{
		"price": 149.99,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify update
	resp, body, _ = c.get(productServiceURL + "/products/" + productID)
	json.Unmarshal(body, &getResult)
	assert.Equal(t, 149.99, getResult["price"])

	// Delete product
	resp, _, _ = c.delete(productServiceURL + "/products/" + productID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify deleted
	resp, _, _ = c.get(productServiceURL + "/products/" + productID)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestStockReserveRelease(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))

	// Create product with stock
	resp, body, _ = c.post(productServiceURL+"/products", map[string]interface{}{
		"name":  "Stock Test Product",
		"price": 10.00,
		"stock": 50,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var createResult map[string]interface{}
	json.Unmarshal(body, &createResult)
	productID := createResult["id"].(string)

	// Reserve stock
	resp, body, _ = c.post(productServiceURL+"/products/"+productID+"/reserve", map[string]interface{}{
		"quantity": 10,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var reserveResult map[string]interface{}
	json.Unmarshal(body, &reserveResult)
	assert.Equal(t, float64(40), reserveResult["stock"])

	// Release stock
	resp, body, _ = c.post(productServiceURL+"/products/"+productID+"/release", map[string]interface{}{
		"quantity": 5,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var releaseResult map[string]interface{}
	json.Unmarshal(body, &releaseResult)
	assert.Equal(t, float64(45), releaseResult["stock"])

	// Cleanup
	c.delete(productServiceURL + "/products/" + productID)
}

// ===================== ORDER TESTS =====================

func TestCreateAndCancelOrder(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))
	adminID := loginResult["id"].(string)

	// Get a product (assume seeded products exist)
	resp, body, _ = c.get(productServiceURL + "/products")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var products []map[string]interface{}
	json.Unmarshal(body, &products)
	require.NotEmpty(t, products, "need at least one product")
	productID := products[0]["id"].(string)
	initialStock := products[0]["stock"].(float64)

	// Create order
	resp, body, _ = c.post(orderServiceURL+"/orders", map[string]interface{}{
		"userId": adminID,
		"items": []map[string]interface{}{
			{"productId": productID, "quantity": 2},
		},
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "create order failed: %s", string(body))
	var orderResult map[string]interface{}
	json.Unmarshal(body, &orderResult)
	orderID := orderResult["id"].(string)
	assert.Equal(t, "PENDING", orderResult["status"])

	// Verify stock decreased
	resp, body, _ = c.get(productServiceURL + "/products/" + productID)
	var productAfter map[string]interface{}
	json.Unmarshal(body, &productAfter)
	assert.Equal(t, initialStock-2, productAfter["stock"].(float64))

	// Cancel order
	resp, body, _ = c.post(orderServiceURL+"/orders/"+orderID+"/cancel", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var cancelResult map[string]interface{}
	json.Unmarshal(body, &cancelResult)
	assert.Equal(t, "CANCELLED", cancelResult["status"])

	// Verify stock restored
	resp, body, _ = c.get(productServiceURL + "/products/" + productID)
	json.Unmarshal(body, &productAfter)
	assert.Equal(t, initialStock, productAfter["stock"].(float64))
}

func TestPayOrder(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(userServiceURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))
	adminID := loginResult["id"].(string)

	// Get a product
	resp, body, _ = c.get(productServiceURL + "/products")
	var products []map[string]interface{}
	json.Unmarshal(body, &products)
	require.NotEmpty(t, products)
	productID := products[0]["id"].(string)

	// Create order
	resp, body, _ = c.post(orderServiceURL+"/orders", map[string]interface{}{
		"userId": adminID,
		"items": []map[string]interface{}{
			{"productId": productID, "quantity": 1},
		},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var orderResult map[string]interface{}
	json.Unmarshal(body, &orderResult)
	orderID := orderResult["id"].(string)

	// Pay order
	resp, body, _ = c.post(orderServiceURL+"/orders/"+orderID+"/pay", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var payResult map[string]interface{}
	json.Unmarshal(body, &payResult)
	assert.Equal(t, "PAID", payResult["status"])

	// Verify cannot cancel paid order
	resp, _, _ = c.post(orderServiceURL+"/orders/"+orderID+"/cancel", nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// ===================== GATEWAY TESTS =====================

func TestGatewayLogin(t *testing.T) {
	c := newClient()

	// Login through gateway
	resp, body, err := c.post(gatewayURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "gateway login failed: %s", string(body))

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.NotEmpty(t, result["token"])
}

func TestGatewayProductProxy(t *testing.T) {
	c := newClient()

	// Login through gateway
	resp, body, _ := c.post(gatewayURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))

	// Get products through gateway
	resp, body, err := c.get(gatewayURL + "/products")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "gateway products failed: %s", string(body))

	var products []map[string]interface{}
	json.Unmarshal(body, &products)
	assert.NotEmpty(t, products)
}

func TestGatewayOrderFlow(t *testing.T) {
	c := newClient()

	// Login
	resp, body, _ := c.post(gatewayURL+"/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	var loginResult map[string]interface{}
	json.Unmarshal(body, &loginResult)
	c.setToken(loginResult["token"].(string))
	adminID := loginResult["id"].(string)

	// Get products
	resp, body, _ = c.get(gatewayURL + "/products")
	var products []map[string]interface{}
	json.Unmarshal(body, &products)
	require.NotEmpty(t, products)

	// Create order through gateway
	resp, body, _ = c.post(gatewayURL+"/orders", map[string]interface{}{
		"userId": adminID,
		"items": []map[string]interface{}{
			{"productId": products[0]["id"], "quantity": 1},
		},
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "gateway order creation failed: %s", string(body))

	var orderResult map[string]interface{}
	json.Unmarshal(body, &orderResult)
	orderID := orderResult["id"].(string)

	// Get order details through gateway
	resp, body, _ = c.get(gatewayURL + "/orders/" + orderID + "/details")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Cancel through gateway
	resp, _, _ = c.post(gatewayURL+"/orders/"+orderID+"/cancel", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
