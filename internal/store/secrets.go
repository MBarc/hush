package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/MBarc/hush/internal/crypto"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrInvalidPath = errors.New("invalid path")
	ErrNotEmpty    = errors.New("folder not empty")
)

// NormalizePath validates and canonicalizes a secret or folder path like
// "HomeLab/Raspberry Pis/hush-server". Segments may contain spaces and most
// printable characters and are trimmed of surrounding whitespace. Empty
// segments, "." and "..", path separators, and control characters are
// rejected so a name can never escape its folder.
func NormalizePath(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return "", nil // root
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		ns, err := normalizeSegment(s)
		if err != nil {
			return "", err
		}
		segs[i] = ns
	}
	return strings.Join(segs, "/"), nil
}

func normalizeSegment(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return "", fmt.Errorf("%w: segment %q", ErrInvalidPath, s)
	}
	for _, r := range s {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return "", fmt.Errorf("%w: segment %q has an illegal character", ErrInvalidPath, s)
		}
	}
	return s, nil
}

type FolderInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

const (
	SecretTypeValue      = "value"
	SecretTypeCredential = "credential"
)

// Credential is the structured form of a secret: a login with optional
// fields. Stored as encrypted JSON in the version blob.
type Credential struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	URL      string `json:"url,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type SecretMeta struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	AgentAccess    bool   `json:"agentAccess"`
	Rotation       string `json:"rotation"`
	CurrentVersion int    `json:"currentVersion"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// secretMetaCols is the column list backing a SecretMeta scan.
const secretMetaCols = `path, name, type, agent_access, rotation, current_version, created_at, updated_at`

type VersionMeta struct {
	Version   int    `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	CreatedBy string `json:"createdBy"`
}

// ensureFolder walks path segments, creating missing folders, and returns
// the id of the deepest folder. Empty path returns 0 (the implicit root).
func ensureFolder(tx *sql.Tx, path string) (int64, error) {
	if path == "" {
		return 0, nil
	}
	var parent sql.NullInt64
	var id int64
	walked := ""
	for _, seg := range strings.Split(path, "/") {
		if walked == "" {
			walked = seg
		} else {
			walked += "/" + seg
		}
		err := tx.QueryRow(
			`SELECT id FROM folders WHERE path = ?`, walked,
		).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			res, ierr := tx.Exec(
				`INSERT INTO folders (parent_id, name, path) VALUES (?, ?, ?)`,
				parent, seg, walked,
			)
			if ierr != nil {
				return 0, ierr
			}
			id, _ = res.LastInsertId()
		} else if err != nil {
			return 0, err
		}
		parent = sql.NullInt64{Int64: id, Valid: true}
	}
	return id, nil
}

// CreateFolder creates the folder (and any missing parents).
func (s *Store) CreateFolder(path string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("%w: empty folder path", ErrInvalidPath)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := ensureFolder(tx, path); err != nil {
		return err
	}
	return tx.Commit()
}

// FolderExists reports whether the folder path exists ("" is always true).
func (s *Store) FolderExists(path string) (bool, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return false, err
	}
	if path == "" {
		return true, nil
	}
	var one int
	err = s.db.QueryRow(`SELECT 1 FROM folders WHERE path = ?`, path).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ListFolder returns the immediate subfolders and secrets of a folder path
// ("" lists the root).
func (s *Store) ListFolder(path string) ([]FolderInfo, []SecretMeta, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return nil, nil, err
	}
	var folderRows *sql.Rows
	if path == "" {
		folderRows, err = s.db.Query(
			`SELECT path, name FROM folders WHERE parent_id IS NULL ORDER BY name`)
	} else {
		if ok, ferr := s.FolderExists(path); ferr != nil {
			return nil, nil, ferr
		} else if !ok {
			return nil, nil, ErrNotFound
		}
		folderRows, err = s.db.Query(
			`SELECT f.path, f.name FROM folders f
			 JOIN folders p ON f.parent_id = p.id WHERE p.path = ? ORDER BY f.name`, path)
	}
	if err != nil {
		return nil, nil, err
	}
	defer folderRows.Close()
	var folders []FolderInfo
	for folderRows.Next() {
		var f FolderInfo
		if err := folderRows.Scan(&f.Path, &f.Name); err != nil {
			return nil, nil, err
		}
		folders = append(folders, f)
	}
	if err := folderRows.Err(); err != nil {
		return nil, nil, err
	}

	var secretRows *sql.Rows
	if path == "" {
		return folders, nil, nil // secrets always live inside a folder
	}
	secretRows, err = s.db.Query(
		`SELECT s.path, s.name, s.type, s.agent_access, s.rotation, s.current_version,
		        s.created_at, s.updated_at
		 FROM secrets s JOIN folders f ON s.folder_id = f.id
		 WHERE f.path = ? ORDER BY s.name`, path)
	if err != nil {
		return nil, nil, err
	}
	defer secretRows.Close()
	var secrets []SecretMeta
	for secretRows.Next() {
		m, err := scanSecretMeta(secretRows)
		if err != nil {
			return nil, nil, err
		}
		secrets = append(secrets, m)
	}
	if err := secretRows.Err(); err != nil {
		return nil, nil, err
	}
	return folders, secrets, nil
}

// SetSecret writes a new version of a value secret at path, creating the
// secret and any missing folders. Returns the new version number.
func (s *Store) SetSecret(path string, value []byte, actor string) (int, error) {
	return s.setBlob(path, value, actor, SecretTypeValue)
}

// SetCredential writes a new version of a credential secret (its fields are
// stored as encrypted JSON).
func (s *Store) SetCredential(path string, c Credential, actor string) (int, error) {
	blob, err := json.Marshal(c)
	if err != nil {
		return 0, err
	}
	return s.setBlob(path, blob, actor, SecretTypeCredential)
}

// setBlob writes a new encrypted version. A secret keeps the type it was
// created with; writing the wrong type is rejected.
func (s *Store) setBlob(path string, plaintext []byte, actor, secretType string) (int, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return 0, err
	}
	dir, name := splitPath(path)
	if name == "" || dir == "" {
		return 0, fmt.Errorf("%w: secrets live inside a folder, like infra/proxmox/root", ErrInvalidPath)
	}
	blob, err := crypto.Encrypt(s.key, plaintext)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	folderID, err := ensureFolder(tx, dir)
	if err != nil {
		return 0, err
	}
	var secretID int64
	var version int
	var existingType string
	err = tx.QueryRow(`SELECT id, current_version, type FROM secrets WHERE path = ?`, path).
		Scan(&secretID, &version, &existingType)
	if errors.Is(err, sql.ErrNoRows) {
		res, ierr := tx.Exec(
			`INSERT INTO secrets (folder_id, name, path, type, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`, folderID, name, path, secretType, now, now)
		if ierr != nil {
			return 0, ierr
		}
		secretID, _ = res.LastInsertId()
		version = 0
	} else if err != nil {
		return 0, err
	} else if existingType != secretType {
		return 0, fmt.Errorf("%w: %s is a %s, not a %s", ErrInvalidPath, path, existingType, secretType)
	}
	version++
	if _, err := tx.Exec(
		`INSERT INTO secret_versions (secret_id, version, blob, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?)`, secretID, version, blob, now, actor); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(
		`UPDATE secrets SET current_version = ?, updated_at = ? WHERE id = ?`,
		version, now, secretID); err != nil {
		return 0, err
	}
	return version, tx.Commit()
}

// GetCredential returns the decrypted credential fields at path.
func (s *Store) GetCredential(path string) (SecretMeta, Credential, error) {
	meta, val, err := s.GetSecret(path)
	if err != nil {
		return SecretMeta{}, Credential{}, err
	}
	if meta.Type != SecretTypeCredential {
		return meta, Credential{}, fmt.Errorf("%w: %s is a %s, not a credential", ErrInvalidPath, path, meta.Type)
	}
	var c Credential
	if err := json.Unmarshal(val, &c); err != nil {
		return meta, Credential{}, err
	}
	return meta, c, nil
}

// GetSecret returns the metadata and decrypted current value at path.
func (s *Store) GetSecret(path string) (SecretMeta, []byte, error) {
	meta, err := s.GetSecretMeta(path)
	if err != nil {
		return SecretMeta{}, nil, err
	}
	value, err := s.GetSecretVersion(meta.Path, meta.CurrentVersion)
	return meta, value, err
}

// GetSecretMeta returns the metadata of the secret at path.
func (s *Store) GetSecretMeta(path string) (SecretMeta, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return SecretMeta{}, err
	}
	m, err := scanSecretMeta(s.db.QueryRow(
		`SELECT `+secretMetaCols+` FROM secrets WHERE path = ?`, path))
	if errors.Is(err, sql.ErrNoRows) {
		return SecretMeta{}, ErrNotFound
	}
	return m, err
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(...any) error }

func scanSecretMeta(row scanner) (SecretMeta, error) {
	var m SecretMeta
	err := row.Scan(&m.Path, &m.Name, &m.Type, &m.AgentAccess, &m.Rotation,
		&m.CurrentVersion, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// GetSecretVersion decrypts and returns a specific version's value.
func (s *Store) GetSecretVersion(path string, version int) ([]byte, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return nil, err
	}
	var blob []byte
	err = s.db.QueryRow(
		`SELECT v.blob FROM secret_versions v
		 JOIN secrets s ON v.secret_id = s.id
		 WHERE s.path = ? AND v.version = ?`, path, version).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return crypto.Decrypt(s.key, blob)
}

// ListVersions lists version metadata for the secret at path, newest first.
func (s *Store) ListVersions(path string) ([]VersionMeta, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetSecretMeta(path); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT v.version, v.created_at, v.created_by FROM secret_versions v
		 JOIN secrets s ON v.secret_id = s.id
		 WHERE s.path = ? ORDER BY v.version DESC`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VersionMeta
	for rows.Next() {
		var v VersionMeta
		if err := rows.Scan(&v.Version, &v.CreatedAt, &v.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteSecret removes the secret and all its versions.
func (s *Store) DeleteSecret(path string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM secrets WHERE path = ?`, path)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveSecret renames a secret or moves it to another folder, creating
// destination folders as needed. The version history follows the secret.
func (s *Store) MoveSecret(from, to string) error {
	from, err := NormalizePath(from)
	if err != nil {
		return err
	}
	to, err = NormalizePath(to)
	if err != nil {
		return err
	}
	dir, name := splitPath(to)
	if name == "" || dir == "" {
		return fmt.Errorf("%w: secrets live inside a folder, like infra/proxmox/root", ErrInvalidPath)
	}
	if from == to {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var secretID int64
	err = tx.QueryRow(`SELECT id FROM secrets WHERE path = ?`, from).Scan(&secretID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var one int
	err = tx.QueryRow(`SELECT 1 FROM secrets WHERE path = ?`, to).Scan(&one)
	if err == nil {
		return fmt.Errorf("%s: %w", to, ErrExists)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	folderID, err := ensureFolder(tx, dir)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE secrets SET folder_id = ?, name = ?, path = ?, updated_at = ? WHERE id = ?`,
		folderID, name, to, time.Now().Unix(), secretID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteFolder removes a folder. Without recursive it refuses when the
// folder still contains subfolders or secrets.
func (s *Store) DeleteFolder(path string, recursive bool) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("%w: cannot delete the root", ErrInvalidPath)
	}
	if !recursive {
		subs, secs, lerr := s.ListFolder(path)
		if lerr != nil {
			return lerr
		}
		if len(subs) > 0 || len(secs) > 0 {
			return ErrNotEmpty
		}
	}
	// Agent tokens live in a folder but have no foreign key to it, so drop
	// any that lived in this folder or its subtree along with it.
	if _, err := s.db.Exec(
		`DELETE FROM tokens WHERE path = ? OR path LIKE ? ESCAPE '\'`,
		path, likePrefix(path)); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM folders WHERE path = ?`, path)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// likePrefix builds a LIKE pattern matching path's descendants ("path/..."),
// escaping LIKE metacharacters in path so names with % or _ are literal.
func likePrefix(path string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(path) + "/%"
}

// CountSecrets returns the number of secrets in the vault.
func (s *Store) CountSecrets() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&n)
	return n, err
}

func splitPath(path string) (dir, name string) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
