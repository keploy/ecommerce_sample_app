package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/keploy/ecommerce-sample-go/internal/config"
	"github.com/keploy/ecommerce-sample-go/internal/db"
	"github.com/keploy/ecommerce-sample-go/internal/middleware"
)

var (
	cfg      *config.Config
	database *sqlx.DB
)

func main() {
	cfg = config.Load()
	cfg.DBName = "product_db"
	cfg.Port = 8081

	database = db.MustConnect(cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	defer database.Close()

	// Seed products
	ensureSeedProducts()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// All routes require auth
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		api.GET("/products", handleGetProducts)
		api.GET("/products/search", handleSearchProducts)
		api.GET("/products/:id", handleGetProduct)
		api.POST("/products", handleCreateProduct)
		api.PUT("/products/:id", handleUpdateProduct)
		api.DELETE("/products/:id", handleDeleteProduct)
		api.POST("/products/:id/reserve", handleReserveStock)
		api.POST("/products/:id/release", handleReleaseStock)
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

func ensureSeedProducts() {
	var count int
	database.Get(&count, "SELECT COUNT(*) FROM products")
	if count == 0 {
		database.Exec(
			"INSERT INTO products (id, name, description, price, stock) VALUES (?, ?, ?, ?, ?)",
			uuid.New().String(), "Laptop", "A powerful and portable laptop.", 1200.00, 50,
		)
		database.Exec(
			"INSERT INTO products (id, name, description, price, stock) VALUES (?, ?, ?, ?, ?)",
			uuid.New().String(), "Mouse", "An ergonomic wireless mouse.", 25.50, 200,
		)
	}
}

// ===================== HANDLERS =====================

type Product struct {
	ID          string  `db:"id" json:"id"`
	Name        string  `db:"name" json:"name"`
	Description *string `db:"description" json:"description"`
	Price       float64 `db:"price" json:"price"`
	Stock       int     `db:"stock" json:"stock"`
}

func handleGetProducts(c *gin.Context) {
	var products []Product
	err := database.Select(&products, "SELECT id, name, description, price, stock FROM products")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	c.JSON(http.StatusOK, products)
}

func handleGetProduct(c *gin.Context) {
	productID := c.Param("id")

	var product Product
	err := database.Get(&product, "SELECT id, name, description, price, stock FROM products WHERE id=?", productID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required"`
	Stock       int     `json:"stock" binding:"required"`
}

func handleCreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	if req.Price < 0 || req.Stock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price and stock must be non-negative"})
		return
	}

	productID := uuid.New().String()
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	_, err := database.Exec(
		"INSERT INTO products (id, name, description, price, stock) VALUES (?, ?, ?, ?, ?)",
		productID, strings.TrimSpace(req.Name), desc, req.Price, req.Stock,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create product: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": productID})
}

func handleUpdateProduct(c *gin.Context) {
	productID := c.Param("id")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var sets []string
	var args []interface{}

	if name, ok := req["name"].(string); ok {
		sets = append(sets, "name=?")
		args = append(args, strings.TrimSpace(name))
	}
	if desc, ok := req["description"]; ok {
		sets = append(sets, "description=?")
		args = append(args, desc)
	}
	if price, ok := req["price"].(float64); ok {
		if price < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "price must be non-negative"})
			return
		}
		sets = append(sets, "price=?")
		args = append(args, price)
	}
	if stock, ok := req["stock"].(float64); ok {
		if stock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stock must be non-negative"})
			return
		}
		sets = append(sets, "stock=?")
		args = append(args, int(stock))
	}

	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	args = append(args, productID)
	result, err := database.Exec(
		fmt.Sprintf("UPDATE products SET %s WHERE id=?", strings.Join(sets, ", ")),
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update product: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func handleDeleteProduct(c *gin.Context) {
	productID := c.Param("id")

	result, err := database.Exec("DELETE FROM products WHERE id=?", productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete product: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

type StockRequest struct {
	Quantity int `json:"quantity" binding:"required"`
}

func handleReserveStock(c *gin.Context) {
	productID := c.Param("id")

	var req StockRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be > 0"})
		return
	}

	result, err := database.Exec(
		"UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?",
		req.Quantity, productID, req.Quantity,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reserve stock: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Insufficient stock or product not found"})
		return
	}

	// Get new stock
	var newStock int
	database.Get(&newStock, "SELECT stock FROM products WHERE id=?", productID)

	c.JSON(http.StatusOK, gin.H{"reserved": req.Quantity, "stock": newStock})
}

func handleReleaseStock(c *gin.Context) {
	productID := c.Param("id")

	var req StockRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be > 0"})
		return
	}

	result, err := database.Exec(
		"UPDATE products SET stock = stock + ? WHERE id = ?",
		req.Quantity, productID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to release stock: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var newStock int
	database.Get(&newStock, "SELECT stock FROM products WHERE id=?", productID)

	c.JSON(http.StatusOK, gin.H{"released": req.Quantity, "stock": newStock})
}

func handleSearchProducts(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	minPriceStr := c.Query("minPrice")
	maxPriceStr := c.Query("maxPrice")

	var clauses []string
	var params []interface{}

	if q != "" {
		clauses = append(clauses, "name LIKE ?")
		params = append(params, "%"+q+"%")
	}
	if minPriceStr != "" {
		minPrice, err := strconv.ParseFloat(minPriceStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid minPrice"})
			return
		}
		clauses = append(clauses, "price >= ?")
		params = append(params, minPrice)
	}
	if maxPriceStr != "" {
		maxPrice, err := strconv.ParseFloat(maxPriceStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid maxPrice"})
			return
		}
		clauses = append(clauses, "price <= ?")
		params = append(params, maxPrice)
	}

	query := "SELECT id, name, description, price, stock FROM products"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var products []Product
	err := database.Select(&products, query, params...)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, products)
}
