package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TokenTypeUser  = "user"
	TokenTypeAgent = "agent"
)

type Token struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Owner      string `json:"owner"`
	Path       string `json:"path,omitempty"` // folder an agent token lives in and may read (cascading); empty for a personal token
	ExpiresAt  int64  `json:"expiresAt,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
}

// CreateToken stores a token's hash and metadata. An agent token lives in a
// folder (path): it may read that folder and everything beneath it. A user
// (personal) token has no folder and acts as its owning account.
func (s *Store) CreateToken(name, typ, hash, ownerUsername, path string, expiresAt int64) error {
	if typ != TokenTypeUser && typ != TokenTypeAgent {
		return fmt.Errorf("invalid token type %q", typ)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty token name")
	}
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	if typ == TokenTypeAgent {
		if path == "" {
			return fmt.Errorf("%w: an agent token lives in a folder", ErrInvalidPath)
		}
		ok, err := s.FolderExists(path)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: folder %s", ErrNotFound, path)
		}
	} else {
		path = "" // personal tokens are not folder items
	}
	owner, _, err := s.GetUser(ownerUsername)
	if err != nil {
		return fmt.Errorf("token owner: %w", err)
	}
	var pathVal any
	if path != "" {
		pathVal = path
	}
	var exp any
	if expiresAt > 0 {
		exp = expiresAt
	}
	_, err = s.db.Exec(
		`INSERT INTO tokens (name, type, token_hash, user_id, path, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, typ, hash, owner.ID, pathVal, exp, time.Now().Unix())
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return fmt.Errorf("token %s: %w", name, ErrExists)
	}
	return err
}

// TokenByHash resolves a presented token hash, enforcing expiry, and stamps
// last_used_at. Also returns the owning user.
func (s *Store) TokenByHash(hash string) (Token, User, error) {
	var t Token
	var owner string
	var path sql.NullString
	var exp, lastUsed sql.NullInt64
	err := s.db.QueryRow(
		`SELECT t.name, t.type, u.username, t.path, t.expires_at, t.created_at, t.last_used_at
		 FROM tokens t JOIN users u ON t.user_id = u.id WHERE t.token_hash = ?`, hash).
		Scan(&t.Name, &t.Type, &owner, &path, &exp, &t.CreatedAt, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, User{}, ErrNotFound
	}
	if err != nil {
		return Token{}, User{}, err
	}
	t.Owner = owner
	t.Path = path.String
	t.ExpiresAt = exp.Int64
	t.LastUsedAt = lastUsed.Int64
	if t.ExpiresAt > 0 && t.ExpiresAt <= time.Now().Unix() {
		return Token{}, User{}, ErrNotFound
	}
	s.db.Exec(`UPDATE tokens SET last_used_at = ? WHERE token_hash = ?`, time.Now().Unix(), hash)
	u, _, err := s.GetUser(owner)
	return t, u, err
}

// ListTokens returns token metadata (never hashes).
func (s *Store) ListTokens() ([]Token, error) {
	return s.queryTokens(
		`SELECT t.name, t.type, u.username, t.path, t.expires_at, t.created_at, t.last_used_at
		 FROM tokens t JOIN users u ON t.user_id = u.id ORDER BY t.name`)
}

// ListFolderTokens returns the agent tokens that live directly in a folder.
// The root ("") holds no tokens: an agent token always names a folder.
func (s *Store) ListFolderTokens(folder string) ([]Token, error) {
	folder, err := NormalizePath(folder)
	if err != nil {
		return nil, err
	}
	if folder == "" {
		return nil, nil
	}
	return s.queryTokens(
		`SELECT t.name, t.type, u.username, t.path, t.expires_at, t.created_at, t.last_used_at
		 FROM tokens t JOIN users u ON t.user_id = u.id
		 WHERE t.path = ? ORDER BY t.name`, folder)
}

func (s *Store) queryTokens(query string, args ...any) ([]Token, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		var path sql.NullString
		var exp, lastUsed sql.NullInt64
		if err := rows.Scan(&t.Name, &t.Type, &t.Owner, &path, &exp, &t.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		t.Path = path.String
		t.ExpiresAt = exp.Int64
		t.LastUsedAt = lastUsed.Int64
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteToken revokes a token by name.
func (s *Store) DeleteToken(name string) error {
	res, err := s.db.Exec(`DELETE FROM tokens WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSecretsWithRotation returns secrets that have a rotation policy.
func (s *Store) ListSecretsWithRotation() ([]SecretMeta, error) {
	rows, err := s.db.Query(
		`SELECT path, name, agent_access, rotation, current_version, created_at, updated_at
		 FROM secrets WHERE rotation != '{}' AND rotation != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SecretMeta
	for rows.Next() {
		var m SecretMeta
		if err := rows.Scan(&m.Path, &m.Name, &m.AgentAccess, &m.Rotation,
			&m.CurrentVersion, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetSecretMeta updates the agent-access flag and/or rotation policy.
func (s *Store) SetSecretMeta(path string, agentAccess *bool, rotation *string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	if _, err := s.GetSecretMeta(path); err != nil {
		return err
	}
	if agentAccess != nil {
		if _, err := s.db.Exec(
			`UPDATE secrets SET agent_access = ?, updated_at = ? WHERE path = ?`,
			*agentAccess, time.Now().Unix(), path); err != nil {
			return err
		}
	}
	if rotation != nil {
		if !json.Valid([]byte(*rotation)) {
			return errors.New("rotation policy must be JSON")
		}
		if _, err := s.db.Exec(
			`UPDATE secrets SET rotation = ?, updated_at = ? WHERE path = ?`,
			*rotation, time.Now().Unix(), path); err != nil {
			return err
		}
	}
	return nil
}
