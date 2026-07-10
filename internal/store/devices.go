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
	IP         string   `json:"ip"`      // primary address, usually IPv4
	MAC        string   `json:"mac,omitempty"`
	IPs        []string `json:"ips"`     // every address the device may connect from
	FirstSeen  int64    `json:"firstSeen"`
	LastSeen   int64    `json:"lastSeen"`
	Status     string   `json:"status"`
	Scopes     []string `json:"scopes"`
	Grants     []string `json:"grants"`
	AllowWrite bool     `json:"allowWrite"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
}

// HasIP reports whether ip (zone-stripped) is a known address for the device.
func (d Device) HasIP(ip string) bool {
	ip = stripZone(ip)
	if ip == d.IP {
		return true
	}
	for _, a := range d.IPs {
		if a == ip {
			return true
		}
	}
	return false
}

func stripZone(ip string) string {
	if i := strings.IndexByte(ip, '%'); i >= 0 {
		return ip[:i]
	}
	return ip
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

const deviceCols = `hostname, ip, first_seen, last_seen, status, scopes, allow_write, expires_at, label, mac, ips`

// UpsertDevice records a poller sighting with no known MAC.
func (s *Store) UpsertDevice(hostname, ip string) error {
	return s.UpsertDeviceSeen(hostname, ip, "")
}

// UpsertDeviceSeen records a poller sighting: new devices start as discovered;
// known devices refresh their primary ip and last_seen, learn their mac, and
// add the seen ip to their address set.
func (s *Store) UpsertDeviceSeen(hostname, ip, mac string) error {
	hostname = normalizeHostname(hostname)
	ip = stripZone(ip)
	if hostname == "" || ip == "" {
		return errors.New("empty hostname or ip")
	}
	mac = strings.ToLower(strings.TrimSpace(mac))
	now := time.Now().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int64
	var ipsJSON, curMAC string
	err = tx.QueryRow(`SELECT id, ips, mac FROM devices WHERE hostname = ?`, hostname).Scan(&id, &ipsJSON, &curMAC)
	if errors.Is(err, sql.ErrNoRows) {
		ips, _ := json.Marshal([]string{ip})
		if _, err := tx.Exec(
			`INSERT INTO devices (hostname, ip, mac, ips, first_seen, last_seen)
			 VALUES (?, ?, ?, ?, ?, ?)`, hostname, ip, mac, string(ips), now, now); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var ips []string
	json.Unmarshal([]byte(ipsJSON), &ips)
	ips = addUnique(ips, ip)
	if mac == "" {
		mac = curMAC // never clear a known MAC
	}
	ipsB, _ := json.Marshal(ips)
	if _, err := tx.Exec(
		`UPDATE devices SET ip = ?, mac = ?, ips = ?, last_seen = ? WHERE id = ?`,
		ip, mac, string(ipsB), now, id); err != nil {
		return err
	}
	return tx.Commit()
}

// AddDeviceIP adds an address to a device's set (used when a request is
// verified by MAC to come from a device at a new address).
func (s *Store) AddDeviceIP(hostname, ip string) error {
	ip = stripZone(ip)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var ipsJSON string
	err = tx.QueryRow(`SELECT id, ips FROM devices WHERE hostname = ?`, normalizeHostname(hostname)).Scan(&id, &ipsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var ips []string
	json.Unmarshal([]byte(ipsJSON), &ips)
	ips = addUnique(ips, ip)
	ipsB, _ := json.Marshal(ips)
	if _, err := tx.Exec(`UPDATE devices SET ips = ? WHERE id = ?`, string(ipsB), id); err != nil {
		return err
	}
	return tx.Commit()
}

func addUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
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

// SetDeviceWrite sets whether a device may write within its granted paths.
func (s *Store) SetDeviceWrite(hostname string, allow bool) error {
	res, err := s.db.Exec(`UPDATE devices SET allow_write = ? WHERE hostname = ?`,
		allow, normalizeHostname(hostname))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UnblockDevice lifts a block, restoring the device to trusted if it still
// has grants, else to discovered. It is a no-op on a device that is not
// blocked.
func (s *Store) UnblockDevice(hostname string) error {
	hostname = normalizeHostname(hostname)
	var id int64
	var status string
	err := s.db.QueryRow(`SELECT id, status FROM devices WHERE hostname = ?`, hostname).Scan(&id, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != DeviceBlocked {
		return nil
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM device_grants WHERE device_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	next := DeviceDiscovered
	if n > 0 {
		next = DeviceTrusted
	}
	_, err = s.db.Exec(`UPDATE devices SET status = ? WHERE id = ?`, next, id)
	return err
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
	// An empty path is the vault root: a grant there covers everything.
	// Any other path must be an existing folder or secret.
	if path != "" {
		if ok, _ := s.FolderExists(path); !ok {
			if _, err := s.GetSecretMeta(path); err != nil {
				return fmt.Errorf("%w: no folder or secret at %q", ErrNotFound, path)
			}
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

// RevokeDeviceGrant removes a device's grant on a path. Trust follows grants:
// when the last grant is removed, a trusted device drops back to discovered
// (a blocked device stays blocked).
func (s *Store) RevokeDeviceGrant(hostname, path string) error {
	path, err := NormalizePath(path)
	if err != nil {
		return err
	}
	var deviceID int64
	err = s.db.QueryRow(`SELECT id FROM devices WHERE hostname = ?`, normalizeHostname(hostname)).Scan(&deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM device_grants WHERE path = ? AND device_id = ?`, path, deviceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	var remaining int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM device_grants WHERE device_id = ?`, deviceID).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err := s.db.Exec(`UPDATE devices SET status = ? WHERE id = ? AND status = ?`,
			DeviceDiscovered, deviceID, DeviceTrusted); err != nil {
			return err
		}
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
			// Inherited from an ancestor; "/" marks a vault-root grant.
			a.Via = grantPath
			if a.Via == "" {
				a.Via = "/"
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// coveringPaths returns the vault root plus path and all its ancestor
// folders: "a/b/c" -> ["", "a", "a/b", "a/b/c"]. A root ("") grant covers
// everything.
func coveringPaths(path string) []string {
	out := []string{""}
	if path == "" {
		return out
	}
	segs := strings.Split(path, "/")
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
	var scopesJSON, ipsJSON string
	var exp sql.NullInt64
	if err := rows.Scan(&d.Hostname, &d.IP, &d.FirstSeen, &d.LastSeen,
		&d.Status, &scopesJSON, &d.AllowWrite, &exp, &d.Label, &d.MAC, &ipsJSON); err != nil {
		return Device{}, err
	}
	d.ExpiresAt = exp.Int64
	json.Unmarshal([]byte(scopesJSON), &d.Scopes)
	json.Unmarshal([]byte(ipsJSON), &d.IPs)
	return d, nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
}
