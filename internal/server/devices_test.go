package server

import (
	"strings"
	"testing"
)

// deviceEnv seeds secrets, a device, trusts it, and grants it the
// "infra/nas" folder (which cascades to everything beneath).
func deviceEnv(t *testing.T, ip string, allowWrite bool) *testEnv {
	t.Helper()
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/nas/backup-key", "cookie:admin", map[string]any{"value": "backup-secret"})
	e.call("PUT", "/api/v1/secrets/infra/nas/private", "cookie:admin", map[string]any{"value": "was-humans-only"})
	e.call("PUT", "/api/v1/secrets/media/plex/token", "cookie:admin", map[string]any{"value": "plex"})

	if err := e.st.UpsertDevice("nas01.lan", ip); err != nil {
		t.Fatal(err)
	}
	// Granting a folder both trusts the device and cascades to everything
	// beneath it; there is no separate trust step.
	code, out := e.call("POST", "/api/v1/grants/infra/nas", "cookie:admin", map[string]any{"hostname": "nas01.lan"})
	if code != 201 {
		t.Fatalf("grant folder: %d %+v", code, out)
	}
	if allowWrite {
		code, out = e.call("PATCH", "/api/v1/devices/nas01.lan", "cookie:admin", map[string]any{"allowWrite": true})
		if code != 200 {
			t.Fatalf("allow write: %d %+v", code, out)
		}
	}
	return e
}

func (e *testEnv) deviceCall(method, path, hostname string, body any) (int, map[string]any) {
	e.t.Helper()
	return e.callWithHeader(method, path, "X-Hush-Device", hostname, body)
}

func TestDeviceAccessByGrant(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)

	// Folder grant cascades: both secrets under infra/nas are readable,
	// including the one with no agent flag (the flag no longer gates devices).
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 200 || out["value"] != "backup-secret" {
		t.Fatalf("granted read: %d %+v", code, out)
	}
	code, out = e.deviceCall("GET", "/api/v1/secrets/infra/nas/private", "nas01", nil)
	if code != 200 || out["value"] != "was-humans-only" {
		t.Fatalf("cascade read: %d %+v", code, out)
	}
	// Not granted: hidden.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/media/plex/token", "nas01", nil)
	if code != 404 {
		t.Fatalf("ungranted read expected 404, got %d", code)
	}
	// Read-only device cannot write.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/infra/nas/backup-key", "nas01", map[string]any{"value": "x"})
	if code != 403 {
		t.Fatalf("read-only write expected 403, got %d", code)
	}
	// Unknown device, browsing, and admin endpoints are all rejected.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "impostor", nil)
	if code != 401 {
		t.Fatalf("unknown device expected 401, got %d", code)
	}
	code, _ = e.deviceCall("GET", "/api/v1/tree/", "nas01", nil)
	if code != 403 {
		t.Fatalf("device tree expected 403, got %d", code)
	}
	code, _ = e.deviceCall("GET", "/api/v1/devices", "nas01", nil)
	if code != 403 {
		t.Fatalf("device admin expected 403, got %d", code)
	}
}

func TestDeviceIndividualSecretGrant(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/dns/cf", "cookie:admin", map[string]any{"value": "cf"})
	e.call("PUT", "/api/v1/secrets/infra/dns/hz", "cookie:admin", map[string]any{"value": "hz"})
	e.st.UpsertDevice("box.lan", "127.0.0.1")

	// Grant a single secret, not the folder (the grant is what trusts it).
	code, _ := e.call("POST", "/api/v1/grants/infra/dns/cf", "cookie:admin", map[string]any{"hostname": "box.lan"})
	if code != 201 {
		t.Fatalf("grant secret: %d", code)
	}
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/dns/cf", "box", nil)
	if code != 200 || out["value"] != "cf" {
		t.Fatalf("granted secret read: %d %+v", code, out)
	}
	// Sibling in the same folder is NOT covered by a single-secret grant.
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/dns/hz", "box", nil)
	if code != 404 {
		t.Fatalf("sibling read expected 404, got %d", code)
	}

	// Resource view lists the device, and revoke removes access.
	code, list := e.call("GET", "/api/v1/grants/infra/dns/cf", "cookie:admin", nil)
	_ = list
	if code != 200 {
		t.Fatalf("grants list: %d", code)
	}
	code, _ = e.call("DELETE", "/api/v1/grants/infra/dns/cf?hostname=box.lan", "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("revoke: %d", code)
	}
	code, _ = e.deviceCall("GET", "/api/v1/secrets/infra/dns/cf", "box", nil)
	if code != 404 {
		t.Fatalf("after revoke expected 404, got %d", code)
	}
}

func TestDeviceRootGrant(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/dns/cf", "cookie:admin", map[string]any{"value": "cf"})
	e.call("PUT", "/api/v1/secrets/media/plex/tok", "cookie:admin", map[string]any{"value": "plex"})
	e.st.UpsertDevice("super.lan", "127.0.0.1")

	// Grant at the vault root (empty path) covers everything and trusts it.
	code, out := e.call("POST", "/api/v1/grants/", "cookie:admin", map[string]any{"hostname": "super.lan"})
	if code != 201 {
		t.Fatalf("root grant: %d %+v", code, out)
	}
	for _, p := range []string{"infra/dns/cf", "media/plex/tok"} {
		code, _ := e.deviceCall("GET", "/api/v1/secrets/"+p, "super", nil)
		if code != 200 {
			t.Fatalf("root-granted read of %s: %d", p, code)
		}
	}

	// A nested secret shows the device as inherited from the root ("/").
	acc, err := e.st.DevicesForPath("infra/dns/cf")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range acc {
		if a.Hostname == "super.lan" && a.Via == "/" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected super.lan inherited via /, got %+v", acc)
	}
}

func TestDeviceQueryMissLogged(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)

	// A device asking for a path that does not exist still gets a 404, but
	// the query is recorded so the audit log shows everything it probed for.
	code, _ := e.deviceCall("GET", "/api/v1/secrets/infra/nas/ghost", "nas01", nil)
	if code != 404 {
		t.Fatalf("missing secret expected 404, got %d", code)
	}
	entries, err := e.st.AuditList(20, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, en := range entries {
		if en.Action == "secret.read.denied" && en.Path == "infra/nas/ghost" &&
			en.Detail == "no such secret" && strings.HasPrefix(en.Actor, "device:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a logged miss for the device, got %+v", entries)
	}
}

func TestDeviceNaming(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)

	code, _ := e.call("PATCH", "/api/v1/devices/nas01.lan", "cookie:admin", map[string]any{"label": "backup-box"})
	if code != 200 {
		t.Fatalf("name device: %d", code)
	}
	// The device authenticates by its friendly name and reads its grant.
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "backup-box", nil)
	if code != 200 || out["value"] != "backup-secret" {
		t.Fatalf("auth by label: %d %+v", code, out)
	}
	// Readonly users cannot rename.
	e.call("POST", "/api/v1/users", "cookie:admin",
		map[string]any{"username": "ro2", "password": "ro2-pass-xyz", "role": "readonly"})
	e.login("ro2", "ro2-pass-xyz")
	code, _ = e.call("PATCH", "/api/v1/devices/nas01.lan", "cookie:ro2", map[string]any{"label": "x"})
	if code != 403 {
		t.Fatalf("readonly rename expected 403, got %d", code)
	}
}

func TestDeviceSpoofedHostnameDenied(t *testing.T) {
	// Inventory says nas01 lives at 10.9.9.9; the request comes from
	// 127.0.0.1, so the claim is rejected before any grant check.
	e := deviceEnv(t, "10.9.9.9", false)
	code, _ := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 401 {
		t.Fatalf("spoofed claim expected 401, got %d", code)
	}
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

func TestDeviceWriteWithinGrant(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", true)

	// Write and create within the granted folder.
	code, _ := e.deviceCall("PUT", "/api/v1/secrets/infra/nas/backup-key", "nas01", map[string]any{"value": "rotated"})
	if code != 200 {
		t.Fatalf("device write: %d", code)
	}
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/infra/nas/new-cert", "nas01", map[string]any{"value": "cert"})
	if code != 200 {
		t.Fatalf("device create: %d", code)
	}
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/new-cert", "nas01", nil)
	if code != 200 || out["value"] != "cert" {
		t.Fatalf("device read-back: %d %+v", code, out)
	}
	// Write outside the grant is hidden.
	code, _ = e.deviceCall("PUT", "/api/v1/secrets/media/plex/token", "nas01", map[string]any{"value": "x"})
	if code != 404 {
		t.Fatalf("ungranted write expected 404, got %d", code)
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

func TestDeviceUnblockRestoresAccess(t *testing.T) {
	e := deviceEnv(t, "127.0.0.1", false)
	e.call("POST", "/api/v1/devices/nas01.lan/block", "cookie:admin", map[string]any{})
	if code, _ := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil); code != 401 {
		t.Fatalf("blocked device expected 401, got %d", code)
	}
	// Unblocking a device that still has grants restores its access.
	code, _ := e.call("POST", "/api/v1/devices/nas01.lan/unblock", "cookie:admin", map[string]any{})
	if code != 200 {
		t.Fatalf("unblock: %d", code)
	}
	code, out := e.deviceCall("GET", "/api/v1/secrets/infra/nas/backup-key", "nas01", nil)
	if code != 200 || out["value"] != "backup-secret" {
		t.Fatalf("unblocked read: %d %+v", code, out)
	}
}
