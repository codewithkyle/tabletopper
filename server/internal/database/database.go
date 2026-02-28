package db

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func Connect() (*sql.DB, error) {
	dsn := os.Getenv("DSN")
	if len(dsn) == 0 {
		slog.Error("Failed to connect to DB", "error", "envrionment variable DSN cannot be empty")
		return nil, errors.New("envrionment variable DSN cannot be empty")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("Failed to connect to DB", "error", err)
		return nil, err
	}

	return db, nil
}
