package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func openDB(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := applyMigrations(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	stmts := []struct {
		query string
		args  []any
	}{
		{
			query: `CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS invites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			used_by_user_id INTEGER,
			used_at TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY (used_by_user_id) REFERENCES users(id)
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS refresh_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT NOT NULL,
			revoked_at TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS contact_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			requester_id INTEGER NOT NULL,
			recipient_id INTEGER NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'rejected')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (requester_id, recipient_id),
			FOREIGN KEY (requester_id) REFERENCES users(id),
			FOREIGN KEY (recipient_id) REFERENCES users(id)
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS contacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_low_id INTEGER NOT NULL,
			user_high_id INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE (user_low_id, user_high_id),
			FOREIGN KEY (user_low_id) REFERENCES users(id),
			FOREIGN KEY (user_high_id) REFERENCES users(id)
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS e2ee_accounts (
			user_id INTEGER PRIMARY KEY,
			protocol_version INTEGER NOT NULL,
			identity_keys TEXT NOT NULL,
			wrapped_master_key TEXT NOT NULL,
			kdf_salt TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS e2ee_devices (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			public_keys TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			revoked_at TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS e2ee_one_time_keys (
			user_id INTEGER NOT NULL,
			device_id TEXT NOT NULL,
			key_id TEXT NOT NULL,
			key_json TEXT NOT NULL,
			PRIMARY KEY (user_id, device_id, key_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (device_id) REFERENCES e2ee_devices(id) ON DELETE CASCADE
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS e2ee_to_device_events (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			recipient_user_id INTEGER NOT NULL,
			recipient_device_id TEXT NOT NULL,
			sender TEXT NOT NULL,
			event_type TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (recipient_user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (recipient_device_id) REFERENCES e2ee_devices(id) ON DELETE CASCADE
		)`,
		},
		{
			query: `CREATE TABLE IF NOT EXISTS encrypted_messages (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id TEXT NOT NULL UNIQUE,
			sender_id INTEGER NOT NULL,
			recipient_id INTEGER NOT NULL,
			ciphertext TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (recipient_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (1, ?)`,
			args: []any{now},
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (2, ?)`,
			args: []any{now},
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (3, ?)`,
			args: []any{now},
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (4, ?)`,
			args: []any{now},
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (5, ?)`,
			args: []any{now},
		},
		{
			query: `INSERT OR IGNORE INTO schema_migrations (version, applied_at)
		 VALUES (6, ?)`,
			args: []any{now},
		},
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}
