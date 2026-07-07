// Package store is Hush's persistence layer: a single SQLite file on the
// data volume, with secret values envelope-encrypted before they touch disk.
package store

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DBFile is the database file name inside the data dir.
const DBFile = "hush.db"

type Store struct {
	db  *sql.DB
	key []byte
}

// Open opens (creating if needed) the database in dataDir and runs
// migrations. masterKey is used to envelope-encrypt secret values.
func Open(dataDir string, masterKey []byte) (*Store, error) {
	path := filepath.ToSlash(filepath.Join(dataDir, DBFile))
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// The pure Go driver is happiest with a single connection; a homelab
	// vault does not need write concurrency.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return &Store{db: db, key: masterKey}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= 1 {
		return nil
	}
	if _, err := db.Exec(schemaV1); err != nil {
		return err
	}
	_, err := db.Exec("PRAGMA user_version = 1")
	return err
}

const schemaV1 = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'readonly')),
    created_at    INTEGER NOT NULL,
    must_change_password INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tokens (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    type         TEXT NOT NULL CHECK (type IN ('user', 'agent')),
    token_hash   TEXT NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scopes       TEXT NOT NULL DEFAULT '[]',
    expires_at   INTEGER,
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE TABLE IF NOT EXISTS folders (
    id        INTEGER PRIMARY KEY,
    parent_id INTEGER REFERENCES folders(id) ON DELETE CASCADE,
    name      TEXT NOT NULL,
    path      TEXT NOT NULL UNIQUE,
    UNIQUE (parent_id, name)
);

CREATE TABLE IF NOT EXISTS user_grants (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    UNIQUE (user_id, folder_id)
);

CREATE TABLE IF NOT EXISTS secrets (
    id              INTEGER PRIMARY KEY,
    folder_id       INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    path            TEXT NOT NULL UNIQUE,
    agent_access    INTEGER NOT NULL DEFAULT 0,
    rotation        TEXT NOT NULL DEFAULT '{}',
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    UNIQUE (folder_id, name)
);

CREATE TABLE IF NOT EXISTS secret_versions (
    id         INTEGER PRIMARY KEY,
    secret_id  INTEGER NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    version    INTEGER NOT NULL,
    blob       BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    created_by TEXT NOT NULL,
    UNIQUE (secret_id, version)
);

CREATE TABLE IF NOT EXISTS devices (
    id          INTEGER PRIMARY KEY,
    hostname    TEXT NOT NULL UNIQUE COLLATE NOCASE,
    ip          TEXT NOT NULL,
    first_seen  INTEGER NOT NULL,
    last_seen   INTEGER NOT NULL,
    status      TEXT NOT NULL DEFAULT 'discovered'
                CHECK (status IN ('discovered', 'trusted', 'blocked')),
    scopes      TEXT NOT NULL DEFAULT '[]',
    allow_write INTEGER NOT NULL DEFAULT 0,
    expires_at  INTEGER
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY,
    ts         INTEGER NOT NULL,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'token', 'device', 'system')),
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    path       TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    detail     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts DESC);
CREATE INDEX IF NOT EXISTS idx_secret_versions ON secret_versions(secret_id, version DESC);
`
