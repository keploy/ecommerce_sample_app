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

	// Kafka
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string

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
	jwtTTL, _ := strconv.Atoi(getEnv("JWT_TTL_SECONDS", "3600")) // 1 hour default
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	resetAdmin := getEnv("RESET_ADMIN_PASSWORD", "false")

	// Parse Kafka brokers (comma-separated)
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "kafka:9092")
	kafkaBrokers := parseBrokers(kafkaBrokersStr)

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

		KafkaBrokers: kafkaBrokers,
		KafkaTopic:   getEnv("KAFKA_TOPIC", "order-events"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "order-service-group"),

		Port: port,

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		ResetAdminPwd: resetAdmin == "1" || resetAdmin == "true" || resetAdmin == "yes",
	}
}

// parseBrokers splits a comma-separated broker string into a slice
func parseBrokers(brokers string) []string {
	if brokers == "" {
		return []string{"kafka:9092"}
	}
	var result []string
	for _, b := range splitAndTrim(brokers, ",") {
		if b != "" {
			result = append(result, b)
		}
	}
	if len(result) == 0 {
		return []string{"kafka:9092"}
	}
	return result
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString is a simple string split implementation
func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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
