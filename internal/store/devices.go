package store

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	IP         string   `json:"ip"`
	FirstSeen  int64    `json:"firstSeen"`
	LastSeen   int64    `json:"lastSeen"`
	Status     string   `json:"status"`
	Scopes     []string `json:"scopes"`
	AllowWrite bool     `json:"allowWrite"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
}

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
	rows, err := s.db.Query(
		`SELECT hostname, ip, first_seen, last_seen, status, scopes, allow_write, expires_at
		 FROM devices`)
	if err != nil {
		return Device{}, err
	}
	defer rows.Close()
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return Device{}, err
		}
		if d.Hostname == claimed || strings.SplitN(d.Hostname, ".", 2)[0] == claimed {
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
	rows, err := s.db.Query(
		`SELECT hostname, ip, first_seen, last_seen, status, scopes, allow_write, expires_at
		 FROM devices ORDER BY last_seen DESC`)
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
	return out, rows.Err()
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

func scanDevice(rows *sql.Rows) (Device, error) {
	var d Device
	var scopesJSON string
	var exp sql.NullInt64
	if err := rows.Scan(&d.Hostname, &d.IP, &d.FirstSeen, &d.LastSeen,
		&d.Status, &scopesJSON, &d.AllowWrite, &exp); err != nil {
		return Device{}, err
	}
	d.ExpiresAt = exp.Int64
	json.Unmarshal([]byte(scopesJSON), &d.Scopes)
	return d, nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
}
