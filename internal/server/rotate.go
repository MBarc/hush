package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

// RotationPolicy is the per-secret rotation configuration, stored as JSON
// in the secret's rotation column.
type RotationPolicy struct {
	// Length of generated values (default 32).
	Length int `json:"length,omitempty"`
	// Charset: "full" (default), "alnum", "hex", "digits", or a literal
	// set of characters to draw from.
	Charset string `json:"charset,omitempty"`
	// IntervalDays > 0 enables scheduled auto-rotation.
	IntervalDays int `json:"intervalDays,omitempty"`
	// WebhookURL is called after each rotation so automation can push the
	// new value into the real service.
	WebhookURL string `json:"webhookUrl,omitempty"`
	// WebhookSecret keys the HMAC-SHA256 signature header.
	WebhookSecret string `json:"webhookSecret,omitempty"`
	// IncludeValue puts the new value in the webhook payload.
	IncludeValue bool `json:"includeValue,omitempty"`
}

func parsePolicy(raw string) RotationPolicy {
	var p RotationPolicy
	json.Unmarshal([]byte(raw), &p)
	if p.Length <= 0 {
		p.Length = 32
	}
	return p
}

func (p RotationPolicy) charset() string {
	switch p.Charset {
	case "", "full":
		return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%^*"
	case "alnum":
		return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	case "hex":
		return "0123456789abcdef"
	case "digits":
		return "0123456789"
	default:
		return p.Charset
	}
}

// rotate generates a fresh value for the secret at path per its policy,
// writes it as a new version, and fires the webhook if configured.
func (s *Server) rotate(path, actor, ip string) (int, error) {
	meta, err := s.st.GetSecretMeta(path)
	if err != nil {
		return 0, err
	}
	policy := parsePolicy(meta.Rotation)
	value, err := auth.GenerateFromCharset(policy.Length, policy.charset())
	if err != nil {
		return 0, err
	}
	version, err := s.st.SetSecret(meta.Path, []byte(value), actor)
	if err != nil {
		return 0, err
	}
	s.st.Audit(store.AuditEntry{ActorType: "system", Actor: actor, Action: "secret.rotate",
		Path: meta.Path, IP: ip, Detail: fmt.Sprintf("version=%d", version)})
	if policy.WebhookURL != "" {
		go s.fireWebhook(meta.Path, version, value, policy)
	}
	return version, nil
}

// fireWebhook delivers the rotation event with an HMAC signature, retrying
// a few times before giving up.
func (s *Server) fireWebhook(path string, version int, value string, policy RotationPolicy) {
	payload := map[string]any{
		"event":   "rotation",
		"path":    path,
		"version": version,
		"ts":      time.Now().Unix(),
	}
	if policy.IncludeValue {
		payload["value"] = value
	}
	body, _ := json.Marshal(payload)
	sig := ""
	if policy.WebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(policy.WebhookSecret))
		mac.Write(body)
		sig = hex.EncodeToString(mac.Sum(nil))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest("POST", policy.WebhookURL, bytes.NewReader(body))
		if err != nil {
			break
		}
		req.Header.Set("Content-Type", "application/json")
		if sig != "" {
			req.Header.Set("X-Hush-Signature", sig)
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 300 {
				s.st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush",
					Action: "webhook.sent", Path: path, Detail: policy.WebhookURL})
				return
			}
			err = fmt.Errorf("status %d", resp.StatusCode)
		}
		if attempt == 3 {
			s.st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush",
				Action: "webhook.failed", Path: path,
				Detail: fmt.Sprintf("%s: %v", policy.WebhookURL, err)})
			return
		}
		time.Sleep(time.Duration(attempt*attempt) * time.Second)
	}
}

func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	path := r.PathValue("path")
	version, err := s.rotate(path, id.label(), clientIP(r))
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "version": version})
}

// StartRotationLoop checks for due secrets on start and then periodically.
func (s *Server) StartRotationLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 15 * time.Minute
	}
	go func() {
		s.rotateDue()
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.rotateDue()
			}
		}
	}()
}

// rotationDue reports whether a secret updated at updatedAt is due under
// policy at time now.
func rotationDue(updatedAt int64, policy RotationPolicy, now int64) bool {
	return policy.IntervalDays > 0 && updatedAt+int64(policy.IntervalDays)*86400 <= now
}

func (s *Server) rotateDue() {
	secrets, err := s.st.ListSecretsWithRotation()
	if err != nil {
		log.Printf("rotation sweep: %v", err)
		return
	}
	now := time.Now().Unix()
	for _, meta := range secrets {
		policy := parsePolicy(meta.Rotation)
		if !rotationDue(meta.UpdatedAt, policy, now) {
			continue
		}
		if _, err := s.rotate(meta.Path, "rotation-schedule", "local"); err != nil {
			log.Printf("scheduled rotation of %s: %v", meta.Path, err)
		} else {
			log.Printf("rotated %s on schedule", meta.Path)
		}
	}
}
