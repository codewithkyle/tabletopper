package db

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var pool *sql.DB

// Init opens the shared connection pool. It must be called once during startup,
// before any call to Get.
func Init() error {
	dsn := os.Getenv("DSN")
	if len(dsn) == 0 {
		slog.Error("Failed to connect to DB", "error", "envrionment variable DSN cannot be empty")
		return errors.New("envrionment variable DSN cannot be empty")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("Failed to connect to DB", "error", err)
		return err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	// NOTE: sql.Open is lazy, so ping to fail fast on a bad DSN
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		slog.Error("Failed to ping DB", "error", err)
		_ = db.Close()
		return err
	}

	pool = db
	return nil
}

// Get returns the shared connection pool opened by Init.
func Get() *sql.DB {
	return pool
}

// Close shuts down the shared connection pool.
func Close() error {
	if pool == nil {
		return nil
	}
	return pool.Close()
}
