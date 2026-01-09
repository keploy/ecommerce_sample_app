package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates JWT tokens from Authorization header
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		tokenString = strings.TrimSpace(tokenString)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Extract claims and store in context
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if sub, exists := claims["sub"]; exists {
				c.Set("user_id", sub)
			}
			if username, exists := claims["username"]; exists {
				c.Set("username", username)
			}
		}

		c.Next()
	}
}

// GetUserID extracts user ID from context (set by AuthMiddleware)
func GetUserID(c *gin.Context) string {
	if id, exists := c.Get("user_id"); exists {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}
