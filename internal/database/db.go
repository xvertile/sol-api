package database

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}

	if err := db.init(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return db, nil
}

func (db *DB) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		api_key TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		testKeys := []string{
			"550e8400-e29b-41d4-a716-446655440000",
			"550e8400-e29b-41d4-a716-446655440001",
			"550e8400-e29b-41d4-a716-446655440002",
		}

		for _, key := range testKeys {
			_, err := db.conn.Exec("INSERT INTO api_keys (api_key) VALUES (?)", key)
			if err != nil {
				slog.Warn("failed to insert test key", "key", key, "error", err)
			} else {
				slog.Info("inserted test API key", "key", key)
			}
		}
	}

	return nil
}

func (db *DB) ValidateAPIKey(apiKey string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM api_keys WHERE api_key = ? AND is_active = 1)"
	err := db.conn.QueryRow(query, apiKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to validate API key: %w", err)
	}
	return exists, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}
