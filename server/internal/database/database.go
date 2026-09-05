// Package database opens the MySQL connection pool. It owns the pool
// settings and the fail-fast ping; the pool itself is handed to whoever
// needs it rather than kept here.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Open returns a connection pool for dsn that has answered a ping. sql.Open
// is lazy, so without the ping a bad DSN would surface on the first query
// instead of at startup.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse DSN: %w", err)
	}

	// RowsAffected reports the rows an UPDATE matched rather than the rows it
	// changed. Handlers use it to tell "not yours" from "nothing to change":
	// a save that found its row but altered no column still counts as found.
	// Set here rather than in the DSN so it cannot be forgotten.
	cfg.ClientFoundRows = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return db, nil
}
