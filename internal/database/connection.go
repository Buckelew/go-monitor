package database

import (
	"database/sql"
	"fmt"

	"github.com/buckelew/go-monitor/config"
	_ "github.com/lib/pq"
)

func Connect(cfg config.Database) (*Queries, *sql.DB, error) {
	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, nil, err
	}

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("failed to ping db: %w", err)
	}

	queries := New(db)

	return queries, db, nil
}
