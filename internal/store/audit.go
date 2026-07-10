package store

import "time"

type AuditEntry struct {
	ID        int64  `json:"id"`
	TS        int64  `json:"ts"`
	ActorType string `json:"actorType"` // user | token | device | system
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Path      string `json:"path,omitempty"`
	IP        string `json:"ip,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Audit appends an entry to the audit log. Failures are returned but
// callers on hot read paths may choose to log rather than fail the request.
func (s *Store) Audit(e AuditEntry) error {
	if e.TS == 0 {
		e.TS = time.Now().Unix()
	}
	_, err := s.db.Exec(
		`INSERT INTO audit_log (ts, actor_type, actor, action, path, ip, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.TS, e.ActorType, e.Actor, e.Action, e.Path, e.IP, e.Detail)
	return err
}

// AuditList returns the newest entries, most recent first.
func (s *Store) AuditList(limit, offset int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return s.scanAudit(
		`SELECT id, ts, actor_type, actor, action, path, ip, detail
		 FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
}

// AuditAll returns every entry, newest first, for a full export.
func (s *Store) AuditAll() ([]AuditEntry, error) {
	return s.scanAudit(
		`SELECT id, ts, actor_type, actor, action, path, ip, detail
		 FROM audit_log ORDER BY id DESC`)
}

// PruneAudit deletes entries older than the given unix timestamp and returns
// how many were removed.
func (s *Store) PruneAudit(before int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM audit_log WHERE ts < ?`, before)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) scanAudit(query string, args ...any) ([]AuditEntry, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.ActorType, &e.Actor, &e.Action,
			&e.Path, &e.IP, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
