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
	DeviceDiscovered = "discovered"
	DeviceTrusted    = "trusted"
	DeviceBlocked    = "blocked"
)

type Device struct {
	Hostname   string   `json:"hostname"`
	Label      string   `json:"label"`
	IP         string   `json:"ip"`
	FirstSeen  int64    `json:"firstSeen"`
	LastSeen   int64    `json:"lastSeen"`
	Status     string   `json:"status"`
	Scopes     []string `json:"scopes"`
	Grants     []string `json:"grants"`
	AllowWrite bool     `json:"allowWrite"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
}

// DeviceAccess is one device's access to a path, for the resource-side view.
// Via is empty for a grant made directly on the path, else the ancestor
// folder the access is inherited from.
type DeviceAccess struct {
	Hostname string `json:"hostname"`
	Label    string `json:"label"`
	IP       string `json:"ip"`
	Via      string `json:"via,omitempty"`
}

const deviceCols = `hostname, ip, first_seen, last_seen, status, scopes, allow_write, expires_at, label`

// UpsertDevice records a poller sighting: new devices start as
// discovered; known devices refresh their ip and last_seen.
func (s *Store) UpsertDevice(hostname, ip string) error {
	hostname = normalizeHostname(hostname)
	if hostname == "" || ip == "" {
		return errors.New("empty hostname or ip")
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`INSERT INTO devices (hostname, ip, first_seen, last_seen)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(hostname) DO UPDATE SET ip = excluded.ip, last_seen = excluded.last_seen`,
		hostname, ip, now, now)
	return err
}

// GetDevice finds a device by claimed hostname. The claim matches either
// the stored name (usually a FQDN from reverse DNS) or its first label.
func (s *Store) GetDevice(claimed string) (Device, error) {
	claimed = normalizeHostname(claimed)
	rows, err := s.db.Query(`SELECT ` + deviceCols + ` FROM devices`)
	if err != nil {
		return Device{}, err
	}
	defer rows.Close()
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return Device{}, err
		}
		// A claim matches the discovered hostname, its first label, or the
		// admin-assigned friendly name.
		if d.Hostname == claimed || strings.SplitN(d.Hostname, ".", 2)[0] == claimed ||
			(d.Label != "" && strings.EqualFold(d.Label, claimed)) {
			return d, nil
		}
	}
	if err := rows.Err(); err != nil {
		return Device{}, err
	}
	return Device{}, ErrNotFound
}

// ListDevices returns the device inventory, most recently seen first.
func (s *Store) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`SELECT ` + deviceCols + ` FROM devices ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Grants, err = s.ListDeviceGrants(out[i].Hostname); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SetDeviceTrust updates a device's status, scopes, write flag, and expiry.
func (s *Store) SetDeviceTrust(hostname, status string, scopes []string, allowWrite bool, expiresAt int64) error {
	if status != DeviceDiscovered && status != DeviceTrusted && status != DeviceBlocked {
		return errors.New("invalid device status")
	}
	scopesJSON, _ := json.Marshal(scopes)
	var exp any
	if expiresAt > 0 {
		exp = expiresAt
	}
	res, err := s.db.Exec(
		`UPDATE devices SET status = ?, scopes = ?, allow_write = ?, expires_at = ?
		 WHERE hostname = ?`, status, string(scopesJSON), allowWrite, exp, normalizeHostname(hostname))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDeviceLabel assigns (or clears, with "") a device's friendly name.
func (s *Store) SetDeviceLabel(hostname, label string) error {
	res, err := s.db.Exec(`UPDATE devices SET label = ? WHERE hostname = ?`,
		strings.TrimSpace(label), normalizeHostname(hostname))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDevice forgets a device entirely.
func (s *Store) DeleteDevice(hostname string) error {
	res, err := s.db.Exec(`DELETE FROM devices WHERE hostname = ?`, normalizeHostname(hostname))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- device grants ---

// GrantDevice allows a device to read a folder (cascading to everything
// beneath) or a single secret. Granting also marks a discovered device
// trusted.
func (s *Store) GrantDevice(hostname, path string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("%w: a grant needs a folder or secret path", ErrInvalidPath)
	}
	if ok, _ := s.FolderExists(path); !ok {
		if _, err := s.GetSecretMeta(path); err != nil {
			return fmt.Errorf("%w: no folder or secret at %q", ErrNotFound, path)
		}
	}
	var deviceID int64
	err = s.db.QueryRow(`SELECT id FROM devices WHERE hostname = ?`, normalizeHostname(hostname)).Scan(&deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: device %q", ErrNotFound, hostname)
	}
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO device_grants (device_id, path) VALUES (?, ?)`, deviceID, path); err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE devices SET status = ? WHERE id = ? AND status = ?`,
		DeviceTrusted, deviceID, DeviceDiscovered)
	return err
}

// RevokeDeviceGrant removes a device's grant on a path.
func (s *Store) RevokeDeviceGrant(hostname, path string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`DELETE FROM device_grants WHERE path = ?
		 AND device_id = (SELECT id FROM devices WHERE hostname = ?)`,
		path, normalizeHostname(hostname))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeviceGrants returns the paths granted to a device.
func (s *Store) ListDeviceGrants(hostname string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT dg.path FROM device_grants dg JOIN devices d ON dg.device_id = d.id
		 WHERE d.hostname = ? ORDER BY dg.path`, normalizeHostname(hostname))
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

// DevicesForPath returns devices with access to path: those granted on it
// directly (Via empty) and those inheriting it from an ancestor folder.
func (s *Store) DevicesForPath(path string) ([]DeviceAccess, error) {
	path, err := NormalizePath(path)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	covers := coveringPaths(path)
	args := make([]any, len(covers))
	for i, c := range covers {
		args[i] = c
	}
	rows, err := s.db.Query(
		`SELECT d.hostname, d.label, d.ip, dg.path
		 FROM device_grants dg JOIN devices d ON dg.device_id = d.id
		 WHERE dg.path IN (`+placeholders(len(covers))+`) ORDER BY d.hostname`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceAccess
	for rows.Next() {
		var a DeviceAccess
		var grantPath string
		if err := rows.Scan(&a.Hostname, &a.Label, &a.IP, &grantPath); err != nil {
			return nil, err
		}
		if grantPath != path {
			a.Via = grantPath
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// coveringPaths returns path plus its ancestor folder paths:
// "a/b/c" -> ["a", "a/b", "a/b/c"].
func coveringPaths(path string) []string {
	segs := strings.Split(path, "/")
	out := make([]string, 0, len(segs))
	for i := 1; i <= len(segs); i++ {
		out = append(out, strings.Join(segs[:i], "/"))
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func scanDevice(rows *sql.Rows) (Device, error) {
	var d Device
	var scopesJSON string
	var exp sql.NullInt64
	if err := rows.Scan(&d.Hostname, &d.IP, &d.FirstSeen, &d.LastSeen,
		&d.Status, &scopesJSON, &d.AllowWrite, &exp, &d.Label); err != nil {
		return Device{}, err
	}
	d.ExpiresAt = exp.Int64
	json.Unmarshal([]byte(scopesJSON), &d.Scopes)
	return d, nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
}
