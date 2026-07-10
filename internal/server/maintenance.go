package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MBarc/hush/internal/store"
)

// StartAuditRetentionLoop prunes audit entries older than retention, on start
// and then every day. A retention of zero or less disables pruning (keep
// forever).
func (s *Server) StartAuditRetentionLoop(ctx context.Context, retention, every time.Duration) {
	if retention <= 0 {
		log.Printf("audit retention off (keeping entries forever)")
		return
	}
	if every <= 0 {
		every = 24 * time.Hour
	}
	go func() {
		s.pruneAudit(retention)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.pruneAudit(retention)
			}
		}
	}()
}

func (s *Server) pruneAudit(retention time.Duration) {
	cutoff := time.Now().Add(-retention).Unix()
	n, err := s.st.PruneAudit(cutoff)
	if err != nil {
		log.Printf("audit prune: %v", err)
		return
	}
	if n > 0 {
		days := int(retention.Hours() / 24)
		s.st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush",
			Action: "audit.prune", Detail: fmt.Sprintf("%d entries older than %d days", n, days)})
		log.Printf("pruned %d audit entries older than %s", n, retention)
	}
}
