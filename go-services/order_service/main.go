package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/keploy/ecommerce-sample-go/internal/config"
	"github.com/keploy/ecommerce-sample-go/internal/db"
	"github.com/keploy/ecommerce-sample-go/internal/middleware"
)

var (
	cfg       *config.Config
	database  *sqlx.DB
	sqsClient *sqs.Client
)

func main() {
	cfg = config.Load()
	cfg.DBName = "order_db"
	cfg.Port = 8080

	database = db.MustConnect(cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	defer database.Close()

	// Initialize SQS client
	initSQS()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		api.POST("/orders", handleCreateOrder)
		api.GET("/orders", handleListOrders)
		api.GET("/orders/:id", handleGetOrder)
		api.GET("/orders/:id/details", handleGetOrderDetails)
		api.POST("/orders/:id/cancel", handleCancelOrder)
		api.POST("/orders/:id/pay", handlePayOrder)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func initSQS() {
	ctx := context.Background()

	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		)),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		log.Printf("Warning: Failed to load AWS config: %v", err)
		return
	}

	sqsOpts := []func(*sqs.Options){}
	if cfg.AWSEndpoint != "" {
		sqsOpts = append(sqsOpts, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
		})
	}

	sqsClient = sqs.NewFromConfig(awsCfg, sqsOpts...)
}

func emitEvent(eventType string, payload map[string]interface{}) {
	if sqsClient == nil || cfg.SQSQueueURL == "" {
		return
	}

	payload["eventType"] = eventType
	body, _ := json.Marshal(payload)

	_, err := sqsClient.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(cfg.SQSQueueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		log.Printf("Failed to send SQS message: %v", err)
	}
}

// HTTP client helpers
func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func fwdAuthHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	if auth := c.GetHeader("Authorization"); auth != "" {
		headers["Authorization"] = auth
	}
	return headers
}

func doRequest(method, url string, body interface{}, headers map[string]string) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	}

	req, _ := http.NewRequest(method, url, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody, nil
}

// ===================== HANDLERS =====================

type OrderItem struct {
	ProductID string  `json:"productId"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price,omitempty"`
}

type CreateOrderRequest struct {
	UserID            string      `json:"userId" binding:"required"`
	Items             []OrderItem `json:"items" binding:"required"`
	ShippingAddressID string      `json:"shippingAddressId"`
}

func handleCreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items must be a non-empty array"})
		return
	}

	headers := fwdAuthHeaders(c)
	idmpKey := c.GetHeader("Idempotency-Key")

	// Validate user
	resp, _, err := doRequest("GET", cfg.UserServiceURL+"/users/"+req.UserID, nil, headers)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("Could not connect to User Service: %v", err)})
		return
	}
	if resp.StatusCode != 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Validate shipping address
	shippingAddressID := req.ShippingAddressID
	if shippingAddressID != "" {
		resp, body, _ := doRequest("GET", cfg.UserServiceURL+"/users/"+req.UserID+"/addresses", nil, headers)
		if resp.StatusCode == 200 {
			var addresses []map[string]interface{}
			json.Unmarshal(body, &addresses)
			found := false
			for _, addr := range addresses {
				if addr["id"] == shippingAddressID {
					found = true
					break
				}
			}
			if !found {
				c.JSON(http.StatusBadRequest, gin.H{"error": "shippingAddressId does not belong to user"})
				return
			}
		}
	} else {
		// Pick default address
		resp, body, _ := doRequest("GET", cfg.UserServiceURL+"/users/"+req.UserID+"/addresses", nil, headers)
		if resp.StatusCode == 200 {
			var addresses []map[string]interface{}
			json.Unmarshal(body, &addresses)
			if len(addresses) > 0 {
				if id, ok := addresses[0]["id"].(string); ok {
					shippingAddressID = id
				}
			}
		}
	}

	// Validate products and calculate total
	var totalAmount float64
	for i := range req.Items {
		item := &req.Items[i]
		if item.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be > 0"})
			return
		}

		resp, body, err := doRequest("GET", cfg.ProductServiceURL+"/products/"+item.ProductID, nil, headers)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("Could not connect to Product Service: %v", err)})
			return
		}
		if resp.StatusCode != 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Product with ID %s not found", item.ProductID)})
			return
		}

		var product map[string]interface{}
		json.Unmarshal(body, &product)

		stock := int(product["stock"].(float64))
		if stock < item.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Not enough stock for product %s", product["name"])})
			return
		}

		item.Price = product["price"].(float64)
		totalAmount += item.Price * float64(item.Quantity)
	}

	// Reserve stock
	var reserved []OrderItem
	for _, item := range req.Items {
		_, _, err := doRequest("POST", cfg.ProductServiceURL+"/products/"+item.ProductID+"/reserve",
			map[string]int{"quantity": item.Quantity}, headers)
		if err == nil {
			reserved = append(reserved, item)
		}
	}

	// Create order in DB
	orderID := uuid.New().String()
	tx, _ := database.Beginx()

	var idmpKeyPtr *string
	if idmpKey != "" {
		idmpKeyPtr = &idmpKey
	}
	var shipAddrPtr *string
	if shippingAddressID != "" {
		shipAddrPtr = &shippingAddressID
	}

	_, err = tx.Exec(
		"INSERT INTO orders (id, user_id, status, idempotency_key, total_amount, shipping_address_id) VALUES (?, ?, ?, ?, ?, ?)",
		orderID, req.UserID, "PENDING", idmpKeyPtr, totalAmount, shipAddrPtr,
	)
	if err != nil {
		tx.Rollback()
		// Release reserved stock
		for _, r := range reserved {
			doRequest("POST", cfg.ProductServiceURL+"/products/"+r.ProductID+"/release",
				map[string]int{"quantity": r.Quantity}, headers)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create order: %v", err)})
		return
	}

	for _, item := range req.Items {
		tx.Exec(
			"INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (?, ?, ?, ?)",
			orderID, item.ProductID, item.Quantity, item.Price,
		)
	}
	tx.Commit()

	// Emit event
	emitEvent("order_created", map[string]interface{}{
		"orderId":     orderID,
		"userId":      req.UserID,
		"totalAmount": totalAmount,
		"items":       req.Items,
	})

	c.JSON(http.StatusCreated, gin.H{"id": orderID, "status": "PENDING"})
}

func handleListOrders(c *gin.Context) {
	userID := c.Query("userId")
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	var clauses []string
	var params []interface{}

	if userID != "" {
		clauses = append(clauses, "user_id=?")
		params = append(params, userID)
	}
	if status != "" {
		clauses = append(clauses, "status=?")
		params = append(params, status)
	}

	query := "SELECT id, user_id, status, total_amount, created_at FROM orders"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC, id ASC LIMIT ?"
	params = append(params, limit+1)

	var orders []struct {
		ID          string    `db:"id" json:"id"`
		UserID      string    `db:"user_id" json:"user_id"`
		Status      string    `db:"status" json:"status"`
		TotalAmount float64   `db:"total_amount" json:"total_amount"`
		CreatedAt   time.Time `db:"created_at" json:"created_at"`
	}
	database.Select(&orders, query, params...)

	c.JSON(http.StatusOK, gin.H{"orders": orders, "nextCursor": nil})
}

func handleGetOrder(c *gin.Context) {
	orderID := c.Param("id")

	var order struct {
		ID                string    `db:"id" json:"id"`
		UserID            string    `db:"user_id" json:"user_id"`
		Status            string    `db:"status" json:"status"`
		TotalAmount       float64   `db:"total_amount" json:"total_amount"`
		ShippingAddressID *string   `db:"shipping_address_id" json:"shipping_address_id"`
		CreatedAt         time.Time `db:"created_at" json:"created_at"`
		UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
	}

	err := database.Get(&order, "SELECT id, user_id, status, total_amount, shipping_address_id, created_at, updated_at FROM orders WHERE id=?", orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	var items []struct {
		ProductID string  `db:"product_id" json:"product_id"`
		Quantity  int     `db:"quantity" json:"quantity"`
		Price     float64 `db:"price" json:"price"`
	}
	database.Select(&items, "SELECT product_id, quantity, price FROM order_items WHERE order_id=?", orderID)

	c.JSON(http.StatusOK, gin.H{
		"id":                  order.ID,
		"user_id":             order.UserID,
		"status":              order.Status,
		"total_amount":        order.TotalAmount,
		"shipping_address_id": order.ShippingAddressID,
		"created_at":          order.CreatedAt,
		"updated_at":          order.UpdatedAt,
		"items":               items,
	})
}

func handleGetOrderDetails(c *gin.Context) {
	orderID := c.Param("id")
	headers := fwdAuthHeaders(c)

	var order struct {
		ID                string    `db:"id"`
		UserID            string    `db:"user_id"`
		Status            string    `db:"status"`
		TotalAmount       float64   `db:"total_amount"`
		ShippingAddressID *string   `db:"shipping_address_id"`
		CreatedAt         time.Time `db:"created_at"`
		UpdatedAt         time.Time `db:"updated_at"`
	}

	err := database.Get(&order, "SELECT id, user_id, status, total_amount, shipping_address_id, created_at, updated_at FROM orders WHERE id=?", orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	var items []struct {
		ProductID string `db:"product_id"`
		Quantity  int    `db:"quantity"`
	}
	database.Select(&items, "SELECT product_id, quantity FROM order_items WHERE order_id=?", orderID)

	// Fetch user details
	var userObj map[string]interface{}
	resp, body, _ := doRequest("GET", cfg.UserServiceURL+"/users/"+order.UserID, nil, headers)
	if resp != nil && resp.StatusCode == 200 {
		json.Unmarshal(body, &userObj)
	}

	// Fetch product details for each item
	var enrichedItems []map[string]interface{}
	for _, it := range items {
		var productObj map[string]interface{}
		resp, body, _ := doRequest("GET", cfg.ProductServiceURL+"/products/"+it.ProductID, nil, headers)
		if resp != nil && resp.StatusCode == 200 {
			json.Unmarshal(body, &productObj)
		}
		enrichedItems = append(enrichedItems, map[string]interface{}{
			"productId": it.ProductID,
			"quantity":  it.Quantity,
			"product":   productObj,
		})
	}

	// Fetch shipping address
	var shippingAddr map[string]interface{}
	resp, body, _ = doRequest("GET", cfg.UserServiceURL+"/users/"+order.UserID+"/addresses", nil, headers)
	if resp != nil && resp.StatusCode == 200 {
		var addresses []map[string]interface{}
		json.Unmarshal(body, &addresses)
		for _, addr := range addresses {
			if order.ShippingAddressID != nil && addr["id"] == *order.ShippingAddressID {
				shippingAddr = addr
				break
			}
		}
		if shippingAddr == nil && len(addresses) > 0 {
			shippingAddr = addresses[0]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                order.ID,
		"status":            order.Status,
		"total_amount":      order.TotalAmount,
		"created_at":        order.CreatedAt.Format(time.RFC3339),
		"updated_at":        order.UpdatedAt.Format(time.RFC3339),
		"userId":            order.UserID,
		"shippingAddressId": order.ShippingAddressID,
		"shippingAddress":   shippingAddr,
		"user":              userObj,
		"items":             enrichedItems,
	})
}

func handleCancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	headers := fwdAuthHeaders(c)

	var order struct {
		Status string `db:"status"`
	}
	err := database.Get(&order, "SELECT status FROM orders WHERE id=?", orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	if order.Status == "CANCELLED" {
		c.JSON(http.StatusOK, gin.H{"status": "CANCELLED"})
		return
	}
	if order.Status == "PAID" {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot cancel a paid order"})
		return
	}

	// Release stock
	var items []struct {
		ProductID string `db:"product_id"`
		Quantity  int    `db:"quantity"`
	}
	database.Select(&items, "SELECT product_id, quantity FROM order_items WHERE order_id=?", orderID)
	for _, item := range items {
		doRequest("POST", cfg.ProductServiceURL+"/products/"+item.ProductID+"/release",
			map[string]int{"quantity": item.Quantity}, headers)
	}

	database.Exec("UPDATE orders SET status='CANCELLED' WHERE id=?", orderID)

	emitEvent("order_cancelled", map[string]interface{}{"orderId": orderID})

	c.JSON(http.StatusOK, gin.H{"id": orderID, "status": "CANCELLED"})
}

func handlePayOrder(c *gin.Context) {
	orderID := c.Param("id")

	var order struct {
		Status      string  `db:"status"`
		UserID      string  `db:"user_id"`
		TotalAmount float64 `db:"total_amount"`
	}
	err := database.Get(&order, "SELECT status, user_id, total_amount FROM orders WHERE id=?", orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	if order.Status == "CANCELLED" {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot pay a cancelled order"})
		return
	}
	if order.Status == "PAID" {
		c.JSON(http.StatusOK, gin.H{"id": orderID, "status": "PAID"})
		return
	}

	database.Exec("UPDATE orders SET status='PAID' WHERE id=?", orderID)

	emitEvent("order_paid", map[string]interface{}{
		"orderId":     orderID,
		"userId":      order.UserID,
		"totalAmount": order.TotalAmount,
	})

	c.JSON(http.StatusOK, gin.H{"id": orderID, "status": "PAID"})
}

