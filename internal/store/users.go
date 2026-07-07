package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RoleAdmin    = "admin"
	RoleReadonly = "readonly"
)

var ErrExists = errors.New("already exists")

type User struct {
	ID                 int64    `json:"-"`
	Username           string   `json:"username"`
	Role               string   `json:"role"`
	CreatedAt          int64    `json:"createdAt"`
	MustChangePassword bool     `json:"mustChangePassword"`
	Grants             []string `json:"grants,omitempty"`
}

// CreateUser adds a local account. passwordHash must already be encoded.
func (s *Store) CreateUser(username, passwordHash, role string) error {
	if role != RoleAdmin && role != RoleReadonly {
		return fmt.Errorf("invalid role %q", role)
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("empty username")
	}
	_, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		username, passwordHash, role, time.Now().Unix())
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return fmt.Errorf("user %s: %w", username, ErrExists)
	}
	return err
}

// GetUser returns a user plus their folder grants.
func (s *Store) GetUser(username string) (User, string, error) {
	var u User
	var hash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, must_change_password
		 FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &hash, &u.Role, &u.CreatedAt, &u.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	u.Grants, err = s.ListGrants(u.ID)
	return u, hash, err
}

// ListUsers returns all users with their grants.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, role, created_at, must_change_password FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.MustChangePassword); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Grants, err = s.ListGrants(out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CountUsers returns the number of local accounts.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// DeleteUser removes a user (sessions, tokens, and grants cascade).
func (s *Store) DeleteUser(username string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPassword replaces a user's password hash.
func (s *Store) SetPassword(username, passwordHash string, mustChange bool) error {
	res, err := s.db.Exec(
		`UPDATE users SET password_hash = ?, must_change_password = ? WHERE username = ?`,
		passwordHash, mustChange, username)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GrantFolder gives a user access to a folder subtree (grants cascade).
func (s *Store) GrantFolder(username, folderPath string) error {
	folderPath, err := NormalizePath(folderPath)
	if err != nil {
		return err
	}
	u, _, err := s.GetUser(username)
	if err != nil {
		return err
	}
	if err := s.CreateFolder(folderPath); err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO user_grants (user_id, folder_id)
		 SELECT ?, id FROM folders WHERE path = ?`, u.ID, folderPath)
	return err
}

// RevokeGrant removes a user's grant on a folder.
func (s *Store) RevokeGrant(username, folderPath string) error {
	folderPath, err := NormalizePath(folderPath)
	if err != nil {
		return err
	}
	u, _, err := s.GetUser(username)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`DELETE FROM user_grants WHERE user_id = ?
		 AND folder_id = (SELECT id FROM folders WHERE path = ?)`, u.ID, folderPath)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGrants returns the folder paths granted to a user.
func (s *Store) ListGrants(userID int64) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT f.path FROM user_grants g JOIN folders f ON g.folder_id = f.id
		 WHERE g.user_id = ? ORDER BY f.path`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- sessions ---

// CreateSession stores a web session.
func (s *Store) CreateSession(id string, userID int64, ttl time.Duration) error {
	now := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		id, userID, now.Unix(), now.Add(ttl).Unix())
	return err
}

// SessionUser resolves a session id to its user, if valid.
func (s *Store) SessionUser(id string) (User, error) {
	var username string
	err := s.db.QueryRow(
		`SELECT u.username FROM sessions se JOIN users u ON se.user_id = u.id
		 WHERE se.id = ? AND se.expires_at > ?`, id, time.Now().Unix()).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u, _, err := s.GetUser(username)
	return u, err
}

// DeleteSession logs a session out; expired sessions are also swept.
func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(
		`DELETE FROM sessions WHERE id = ? OR expires_at <= ?`, id, time.Now().Unix())
	return err
}
