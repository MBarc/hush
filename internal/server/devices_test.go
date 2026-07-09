package server

import (
	"testing"
)

// deviceEnv seeds a device the way the poller would, then trusts it.
func deviceEnv(t *testing.T, ip string, allowWrite bool) *testEnv {
	t.Helper()
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/nas/backup-key", "cookie:admin",
		map[string]any{"value": "backup-secret", "agentAccess": true})
	e.call("PUT", "/api/v1/secrets/infra/nas/private", "cookie:admin",
		map[string]any{"value": "humans-only"}) // agent access off
	e.call("PUT", "/api/v1/secrets/media/plex/token", "cookie:admin",
		map[string]any{"value": "plex", "agentAccess": true})

	// Seed the inventory as the poller would (reverse DNS gave a FQDN).
	if err := e.st.UpsertDevice("nas01.lan", ip); err != nil {
		t.Fatal(err)
	}
	code, out := e.call("POST", "/api/v1/devices/nas01.lan/trust", "cookie:admin",
		map[string]any{"scopes": []string{"infra/nas/*"}, "allowWrite": allowWrite})
	if code != 200 {
		t.Fatalf("trust: %d %+v", code, out)
	}
	return e
}

func (e *testEnv) deviceCall(method, path, hostname string, body any) (int, map[string]any) {
	e.t.Helper()
	return e.callWithHeader(method, path, "X-Hush-Device", hostname, body)
}

func TestDeviceAccess(t *testing.T) {
	// httptest connections come from 127.0.0.1, so a device registered at
	// 127.0.0.1 passes the source-IP check.
	e := deviceEnv(t, "127.0.0.1", false)

	// Short-name claim for a FQDN inventory entry, in scope, flagged: allowed.
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 200 || out["value"] != "backup-secret" {
		t.Fatalf("device read: %d %+v", code, out)
	}
	// In scope but agent-access off: hidden.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/nas/private", "nas01", nil)
	if code != 404 {
		t.Fatalf("flag-off read expected 404, got %d", code)
	}
	// Out of scope: hidden.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/media/plex/token", "nas01", nil)
	if code != 404 {
		t.Fatalf("out-of-scope read expected 404, got %d", code)
	}
	// Read-only device cannot write.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/infra/nas/backup-key", "nas01",
		map[string]any{"value": "overwrite"})
	if code != 403 {
		t.Fatalf("read-only device write expected 403, got %d", code)
	}
	// Unknown hostname: unauthorized.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "impostor", nil)
	if code != 401 {
		t.Fatalf("unknown device expected 401, got %d", code)
	}
	// Devices cannot browse or use admin endpoints.
	code, _ = e.deviceCall("GET", "/api/v1/tree/", "nas01", nil)
	if code != 403 {
		t.Fatalf("device tree expected 403, got %d", code)
	}
	code, _ = e.deviceCall("GET", "/api/v1/devices", "nas01", nil)
	if code != 403 {
		t.Fatalf("device admin expected 403, got %d", code)
	}
}

func TestDeviceNaming(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)

	// Admin gives the device a friendly name.
	code, _ := e.call("PATCH", "/api/v1/devices/nas01.lan", "cookie:admin",
		map[string]any{"label": "backup-box"})
	if code != 200 {
		t.Fatalf("name device: %d", code)
	}
	d, err := e.st.GetDevice("nas01.lan")
	if err != nil || d.Label != "backup-box" {
		t.Fatalf("label not persisted: %+v err=%v", d, err)
	}

	// The device can now authenticate by that friendly name.
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "backup-box", nil)
	if code != 200 || out["value"] != "backup-secret" {
		t.Fatalf("auth by label: %d %+v", code, out)
	}

	// Readonly users cannot rename devices.
	e.call("POST", "/api/v1/users", "cookie:admin",
		map[string]any{"username": "ro2", "password": "ro2-pass-xyz", "role": "readonly"})
	e.login("ro2", "ro2-pass-xyz")
	code, _ = e.call("PATCH", "/api/v1/devices/nas01.lan", "cookie:ro2", map[string]any{"label": "x"})
	if code != 403 {
		t.Fatalf("readonly rename expected 403, got %d", code)
	}
}

func TestDeviceSpoofedHostnameDenied(t *testing.T) {
	// Device inventory says nas01 lives at 10.9.9.9; the request will come
	// from 127.0.0.1, so the claim must be rejected.
	e := deviceEnv(t, "10.9.9.9", false)
	code, _ := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 401 {
		t.Fatalf("spoofed claim expected 401, got %d", code)
	}
	// And the denial is audited with the mismatch.
	entries, err := e.st.AuditList(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, en := range entries {
		if en.Action == "device.denied" && en.Actor == "device:nas01" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected device.denied audit entry, got %+v", entries)
	}
}

func TestDeviceWrite(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", true)

	// Write to an existing agent-accessible secret in scope: allowed.
	code, out := e.deviceCall("PUT", "/api/v1/secrets/infra/nas/backup-key", "nas01",
		map[string]any{"value": "rotated-by-device"})
	if code != 200 {
		t.Fatalf("device write: %d %+v", code, out)
	}
	// New secret in scope: allowed and auto-flagged agent-accessible.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/infra/nas/new-cert", "nas01",
		map[string]any{"value": "cert"})
	if code != 200 {
		t.Fatalf("device create: %d", code)
	}
	code, out = e.deviceCall("GET", "/api/v1/secrets/infra/nas/new-cert", "nas01", nil)
	if code != 200 || out["value"] != "cert" {
		t.Fatalf("device read-back: %d %+v", code, out)
	}
	// Existing human-only secret stays hidden even for writes.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/infra/nas/private", "nas01",
		map[string]any{"value": "x"})
	if code != 404 {
		t.Fatalf("device write to flag-off expected 404, got %d", code)
	}
	// Out-of-scope write: hidden.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/media/plex/token", "nas01",
		map[string]any{"value": "x"})
	if code != 404 {
		t.Fatalf("device write out of scope expected 404, got %d", code)
	}
}

func TestBlockedDeviceDenied(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)
	code, _ := e.call("POST", "/api/v1/devices/nas01.lan/block", "cookie:admin", map[string]any{})
	if code != 200 {
		t.Fatalf("block: %d", code)
	}
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 401 {
		t.Fatalf("blocked device expected 401, got %d", code)
	}
}
