package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/keploy/ecommerce-sample-go/internal/auth"
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
	cfg.DBName = "user_db"
	cfg.Port = 8082

	database = db.MustConnect(cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	defer database.Close()

	// Seed admin user
	ensureSeedUser()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Public routes
	r.POST("/api/v1/login", handleLogin)

	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		protected.POST("/users", handleCreateUser)
		protected.GET("/users/:id", handleGetUser)
		protected.DELETE("/users/:id", handleDeleteUser)

		protected.POST("/users/:id/addresses", handleCreateAddress)
		protected.GET("/users/:id/addresses", handleListAddresses)
		protected.PUT("/users/:id/addresses/:addrId", handleUpdateAddress)
		protected.DELETE("/users/:id/addresses/:addrId", handleDeleteAddress)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// Graceful shutdown
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
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}

func ensureSeedUser() {
	var exists bool
	err := database.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE username=? OR email=?)", cfg.AdminUsername, cfg.AdminEmail)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking admin user: %v", err)
		return
	}

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)

	if !exists {
		uid := uuid.New().String()
		_, err = database.Exec(
			"INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)",
			uid, cfg.AdminUsername, cfg.AdminEmail, string(hashedPwd),
		)
		if err != nil {
			log.Printf("Error creating admin user: %v", err)
		}
	} else if cfg.ResetAdminPwd {
		_, err = database.Exec(
			"UPDATE users SET password_hash=? WHERE username=? OR email=?",
			string(hashedPwd), cfg.AdminUsername, cfg.AdminEmail,
		)
		if err != nil {
			log.Printf("Error resetting admin password: %v", err)
		}
	}
}

// ===================== HANDLERS =====================

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	var user struct {
		ID           string `db:"id"`
		Username     string `db:"username"`
		Email        string `db:"email"`
		PasswordHash string `db:"password_hash"`
	}

	err := database.Get(&user, "SELECT id, username, email, password_hash FROM users WHERE username=?", req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, cfg.JWTSecret, cfg.JWTExpiry())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"token":    token,
	})
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Phone    string `json:"phone"`
}

func handleCreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)

	if len(username) < 3 || len(username) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3-50 chars"})
		return
	}
	if !strings.Contains(email, "@") || len(email) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password too short"})
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	userID := uuid.New().String()
	var phone *string
	if req.Phone != "" {
		phone = &req.Phone
	}

	_, err = database.Exec(
		"INSERT INTO users (id, username, email, password_hash, phone) VALUES (?, ?, ?, ?, ?)",
		userID, username, email, string(hashedPwd), phone,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       userID,
		"username": username,
		"email":    email,
		"phone":    phone,
	})
}

func handleGetUser(c *gin.Context) {
	userID := c.Param("id")

	var user struct {
		ID        string    `db:"id" json:"id"`
		Username  string    `db:"username" json:"username"`
		Email     string    `db:"email" json:"email"`
		Phone     *string   `db:"phone" json:"phone"`
		CreatedAt time.Time `db:"created_at" json:"created_at"`
	}

	err := database.Get(&user, "SELECT id, username, email, phone, created_at FROM users WHERE id=?", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var addresses []struct {
		ID         string  `db:"id" json:"id"`
		Line1      string  `db:"line1" json:"line1"`
		Line2      *string `db:"line2" json:"line2"`
		City       string  `db:"city" json:"city"`
		State      string  `db:"state" json:"state"`
		PostalCode string  `db:"postal_code" json:"postal_code"`
		Country    string  `db:"country" json:"country"`
		Phone      *string `db:"phone" json:"phone"`
		IsDefault  int     `db:"is_default" json:"is_default"`
	}
	database.Select(&addresses, "SELECT id, line1, line2, city, state, postal_code, country, phone, is_default FROM addresses WHERE user_id=? ORDER BY is_default DESC, created_at DESC", userID)

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"phone":      user.Phone,
		"created_at": user.CreatedAt,
		"addresses":  addresses,
	})
}

func handleDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// Check if user exists
	var exists bool
	database.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE id=?)", userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	tx, err := database.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Delete addresses first (FK constraint)
	tx.Exec("DELETE FROM addresses WHERE user_id=?", userID)
	// Delete user
	tx.Exec("DELETE FROM users WHERE id=?", userID)

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

type CreateAddressRequest struct {
	Line1      string `json:"line1" binding:"required"`
	Line2      string `json:"line2"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state" binding:"required"`
	PostalCode string `json:"postal_code" binding:"required"`
	Country    string `json:"country" binding:"required"`
	Phone      string `json:"phone"`
	IsDefault  bool   `json:"is_default"`
}

func handleCreateAddress(c *gin.Context) {
	userID := c.Param("id")

	var req CreateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	// Check user exists
	var exists bool
	database.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE id=?)", userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	addrID := uuid.New().String()
	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}

	var line2, phone *string
	if req.Line2 != "" {
		line2 = &req.Line2
	}
	if req.Phone != "" {
		phone = &req.Phone
	}

	tx, _ := database.Beginx()
	_, err := tx.Exec(
		"INSERT INTO addresses (id, user_id, line1, line2, city, state, postal_code, country, phone, is_default) VALUES (?,?,?,?,?,?,?,?,?,?)",
		addrID, userID, req.Line1, line2, req.City, req.State, req.PostalCode, req.Country, phone, isDefault,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create address: %v", err)})
		return
	}

	if isDefault == 1 {
		tx.Exec("UPDATE addresses SET is_default=0 WHERE user_id=? AND id<>?", userID, addrID)
	}
	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{"id": addrID})
}

func handleListAddresses(c *gin.Context) {
	userID := c.Param("id")

	var exists bool
	database.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE id=?)", userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var addresses []struct {
		ID         string  `db:"id" json:"id"`
		Line1      string  `db:"line1" json:"line1"`
		Line2      *string `db:"line2" json:"line2"`
		City       string  `db:"city" json:"city"`
		State      string  `db:"state" json:"state"`
		PostalCode string  `db:"postal_code" json:"postal_code"`
		Country    string  `db:"country" json:"country"`
		Phone      *string `db:"phone" json:"phone"`
		IsDefault  int     `db:"is_default" json:"is_default"`
	}
	database.Select(&addresses, "SELECT id, line1, line2, city, state, postal_code, country, phone, is_default FROM addresses WHERE user_id=? ORDER BY is_default DESC, created_at DESC", userID)

	c.JSON(http.StatusOK, addresses)
}

func handleUpdateAddress(c *gin.Context) {
	userID := c.Param("id")
	addrID := c.Param("addrId")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if len(req) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	// Build update query dynamically
	var sets []string
	var args []interface{}

	fields := []string{"line1", "line2", "city", "state", "postal_code", "country", "phone"}
	for _, f := range fields {
		if val, ok := req[f]; ok {
			sets = append(sets, f+"=?")
			args = append(args, val)
		}
	}
	if val, ok := req["is_default"]; ok {
		isDefault := 0
		if b, ok := val.(bool); ok && b {
			isDefault = 1
		}
		sets = append(sets, "is_default=?")
		args = append(args, isDefault)
	}

	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	args = append(args, userID, addrID)
	result, err := database.Exec(
		fmt.Sprintf("UPDATE addresses SET %s WHERE user_id=? AND id=?", strings.Join(sets, ", ")),
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update address: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
		return
	}

	// Handle is_default update
	if val, ok := req["is_default"]; ok {
		if b, ok := val.(bool); ok && b {
			database.Exec("UPDATE addresses SET is_default=0 WHERE user_id=? AND id<>?", userID, addrID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func handleDeleteAddress(c *gin.Context) {
	userID := c.Param("id")
	addrID := c.Param("addrId")

	result, err := database.Exec("DELETE FROM addresses WHERE user_id=? AND id=?", userID, addrID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete address: %v", err)})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
