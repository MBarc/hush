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
	rows, err := s.db.Query(
		`SELECT id, ts, actor_type, actor, action, path, ip, detail
		 FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
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
