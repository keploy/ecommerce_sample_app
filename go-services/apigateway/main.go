package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/keploy/ecommerce-sample-go/internal/config"
)

var cfg *config.Config

func main() {
	cfg = config.Load()
	cfg.Port = 8083

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Login (no auth needed - public endpoint)
	r.POST("/api/v1/login", proxyHandler(cfg.UserServiceURL, "login"))

	// Users - proxy to user service
	r.Any("/api/v1/users", proxyHandler(cfg.UserServiceURL, "users"))
	r.Any("/api/v1/users/*path", func(c *gin.Context) {
		subpath := c.Param("path")
		proxy(c, cfg.UserServiceURL, "users"+subpath)
	})

	// Products - proxy to product service
	r.Any("/api/v1/products", proxyHandler(cfg.ProductServiceURL, "products"))
	r.Any("/api/v1/products/*path", func(c *gin.Context) {
		subpath := c.Param("path")
		proxy(c, cfg.ProductServiceURL, "products"+subpath)
	})

	// Orders - proxy to order service
	r.Any("/api/v1/orders", proxyHandler(cfg.OrderServiceURL, "orders"))
	r.Any("/api/v1/orders/*path", func(c *gin.Context) {
		subpath := c.Param("path")
		proxy(c, cfg.OrderServiceURL, "orders"+subpath)
	})

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

func proxyHandler(baseURL, subpath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxy(c, baseURL, subpath)
	}
}

func proxy(c *gin.Context, baseURL, subpath string) {
	targetURL := fmt.Sprintf("%s/%s", baseURL, subpath)

	// Forward query params
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// Create request
	var body io.Reader
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
		body = c.Request.Body
	}

	req, err := http.NewRequest(c.Request.Method, targetURL, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Forward safe headers
	forwardHeaders := []string{"Authorization", "Content-Type", "Accept", "Idempotency-Key"}
	for _, h := range forwardHeaders {
		if v := c.GetHeader(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	// Make request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Upstream unavailable: %v", err)})
		return
	}
	defer resp.Body.Close()

	// Copy response
	respBody, _ := io.ReadAll(resp.Body)

	// Forward content-type
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

