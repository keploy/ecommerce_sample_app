package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for services
type Config struct {
	// Database
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string

	// JWT
	JWTSecret     string
	JWTAlgorithm  string
	JWTTTLSeconds int

	// Service URLs (for inter-service communication)
	UserServiceURL    string
	ProductServiceURL string
	OrderServiceURL   string

	// AWS/SQS
	AWSRegion   string
	AWSEndpoint string
	SQSQueueURL string

	// Server
	Port int

	// Admin seed
	AdminUsername string
	AdminEmail    string
	AdminPassword string
	ResetAdminPwd bool
}

// Load loads configuration from environment variables
func Load() *Config {
	jwtTTL, _ := strconv.Atoi(getEnv("JWT_TTL_SECONDS", "10")) // 30 days default
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	resetAdmin := getEnv("RESET_ADMIN_PASSWORD", "false")

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBUser:     getEnv("DB_USER", "user"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", ""),

		JWTSecret:     getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTAlgorithm:  "HS256",
		JWTTTLSeconds: jwtTTL,

		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8082/api/v1"),
		ProductServiceURL: getEnv("PRODUCT_SERVICE_URL", "http://localhost:8081/api/v1"),
		OrderServiceURL:   getEnv("ORDER_SERVICE_URL", "http://localhost:8080/api/v1"),

		AWSRegion:   getEnv("AWS_REGION", "us-east-1"),
		AWSEndpoint: getEnv("AWS_ENDPOINT", ""),
		SQSQueueURL: getEnv("SQS_QUEUE_URL", ""),

		Port: port,

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		ResetAdminPwd: resetAdmin == "1" || resetAdmin == "true" || resetAdmin == "yes",
	}
}

// JWTExpiry returns the JWT expiry duration
func (c *Config) JWTExpiry() time.Duration {
	return time.Duration(c.JWTTTLSeconds) * time.Second
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
