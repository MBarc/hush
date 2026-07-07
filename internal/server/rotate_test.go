package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestRotateEndpoint(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/db/root", "cookie:admin", map[string]any{"value": "initial"})
	code, _ := e.call("PATCH", "/api/v1/secrets/infra/db/root", "cookie:admin",
		map[string]any{"rotation": map[string]any{"length": 16, "charset": "hex"}})
	if code != 200 {
		t.Fatalf("set policy: %d", code)
	}
	code, out := e.call("POST", "/api/v1/rotate/infra/db/root", "cookie:admin", map[string]any{})
	if code != 200 || out["version"].(float64) != 2 {
		t.Fatalf("rotate: %d %+v", code, out)
	}
	code, out = e.call("GET", "/api/v1/secrets/infra/db/root", "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("get: %d", code)
	}
	value := out["value"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(value) {
		t.Fatalf("rotated value should be 16 hex chars, got %q", value)
	}
	// Readonly users cannot rotate.
	e.call("POST", "/api/v1/users", "cookie:admin",
		map[string]any{"username": "viewer", "password": "viewer-pass-1", "role": "readonly"})
	e.login("viewer", "viewer-pass-1")
	code, _ = e.call("POST", "/api/v1/rotate/infra/db/root", "cookie:viewer", map[string]any{})
	if code != 403 {
		t.Fatalf("readonly rotate expected 403, got %d", code)
	}
}

func TestRotationWebhook(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")

	got := make(chan *http.Request, 1)
	body := make(chan []byte, 1)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- r
		body <- b
	}))
	defer hook.Close()

	e.call("PUT", "/api/v1/secrets/infra/db/admin", "cookie:admin", map[string]any{"value": "one"})
	e.call("PATCH", "/api/v1/secrets/infra/db/admin", "cookie:admin",
		map[string]any{"rotation": map[string]any{
			"length": 20, "webhookUrl": hook.URL, "webhookSecret": "whsec", "includeValue": true,
		}})
	code, _ := e.call("POST", "/api/v1/rotate/infra/db/admin", "cookie:admin", map[string]any{})
	if code != 200 {
		t.Fatalf("rotate: %d", code)
	}

	select {
	case r := <-got:
		b := <-body
		var payload map[string]any
		json.Unmarshal(b, &payload)
		if payload["event"] != "rotation" || payload["path"] != "infra/db/admin" {
			t.Fatalf("payload: %+v", payload)
		}
		// Value included and matches the stored secret.
		_, out := e.call("GET", "/api/v1/secrets/infra/db/admin", "cookie:admin", nil)
		if payload["value"] != out["value"] {
			t.Fatal("webhook value must match the rotated secret")
		}
		// Signature verifies.
		mac := hmac.New(sha256.New, []byte("whsec"))
		mac.Write(b)
		want := hex.EncodeToString(mac.Sum(nil))
		if r.Header.Get("X-Hush-Signature") != want {
			t.Fatalf("bad signature: %s != %s", r.Header.Get("X-Hush-Signature"), want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook never arrived")
	}
}

func TestRotationDue(t *testing.T) {
	now := time.Now().Unix()
	if rotationDue(now, RotationPolicy{IntervalDays: 0}, now) {
		t.Fatal("no interval means never due")
	}
	if rotationDue(now-86400, RotationPolicy{IntervalDays: 7}, now) {
		t.Fatal("1 day old with 7 day interval is not due")
	}
	if !rotationDue(now-8*86400, RotationPolicy{IntervalDays: 7}, now) {
		t.Fatal("8 days old with 7 day interval is due")
	}
}
