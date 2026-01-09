package db

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Connect creates a MySQL connection with retry logic
func Connect(host, user, password, dbName string, retries int, delay time.Duration) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", user, password, host, dbName)

	var db *sqlx.DB
	var lastErr error

	for i := 0; i < retries; i++ {
		db, lastErr = sqlx.Connect("mysql", dsn)
		if lastErr == nil {
			// Configure connection pool
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			return db, nil
		}
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("failed to connect to database after %d retries: %w", retries, lastErr)
}

// MustConnect connects or panics
func MustConnect(host, user, password, dbName string) *sqlx.DB {
	db, err := Connect(host, user, password, dbName, 30, time.Second)
	if err != nil {
		panic(err)
	}
	return db
}
