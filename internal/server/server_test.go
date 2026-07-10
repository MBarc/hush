package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/MBarc/hush/internal/crypto"
	"github.com/MBarc/hush/internal/store"
)

type testEnv struct {
	t   *testing.T
	ts  *httptest.Server
	st  *store.Store
	jar map[string]string // username -> session cookie
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	os.Setenv("HUSH_ADMIN_PASSWORD", "test-admin-password")
	t.Cleanup(func() { os.Unsetenv("HUSH_ADMIN_PASSWORD") })
	key := make([]byte, crypto.KeySize)
	rand.Read(key)
	st, err := store.Open(t.TempDir(), key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv, err := New(st, "test")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &testEnv{t: t, ts: ts, st: st, jar: map[string]string{}}
}

// callWithHeader is call with an arbitrary auth header instead of
// cookie/bearer credentials.
func (e *testEnv) callWithHeader(method, path, header, value string, body any) (int, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, &buf)
	req.Header.Set(header, value)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// call makes a request. cred is "" (anon), "cookie:<user>" for a session,
// or a bearer token.
func (e *testEnv) call(method, path, cred string, body any) (int, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, &buf)
	if strings.HasPrefix(cred, "cookie:") {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.jar[strings.TrimPrefix(cred, "cookie:")]})
	} else if cred != "" {
		req.Header.Set("Authorization", "Bearer "+cred)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e *testEnv) login(username, password string) {
	e.t.Helper()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"username": username, "password": password})
	resp, err := http.Post(e.ts.URL+"/api/v1/auth/login", "application/json", &buf)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		e.t.Fatalf("login %s: status %d", username, resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			e.jar[username] = c.Value
		}
	}
}

func TestBootstrapAndLogin(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	code, me := e.call("GET", "/api/v1/auth/me", "cookie:admin", nil)
	if code != 200 || me["username"] != "admin" || me["admin"] != true {
		t.Fatalf("me: %d %+v", code, me)
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"username": "admin", "password": "wrong"})
	resp, _ := http.Post(e.ts.URL+"/api/v1/auth/login", "application/json", &buf)
	if resp.StatusCode != 401 {
		t.Fatalf("bad login: %d", resp.StatusCode)
	}
}

func TestSecretLifecycle(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")

	code, out := e.call("PUT", "/api/v1/secrets/infra/proxmox/root", "cookie:admin",
		map[string]any{"value": "hunter2"})
	if code != 200 || out["version"].(float64) != 1 {
		t.Fatalf("put: %d %+v", code, out)
	}
	code, out = e.call("GET", "/api/v1/secrets/infra/proxmox/root", "cookie:admin", nil)
	if code != 200 || out["value"] != "hunter2" {
		t.Fatalf("get: %d %+v", code, out)
	}
	e.call("PUT", "/api/v1/secrets/infra/proxmox/root", "cookie:admin", map[string]any{"value": "hunter3"})
	code, out = e.call("GET", "/api/v1/secrets/infra/proxmox/root?version=1", "cookie:admin", nil)
	if code != 200 || out["value"] != "hunter2" {
		t.Fatalf("old version: %d %+v", code, out)
	}
	code, out = e.call("GET", "/api/v1/secrets/infra/proxmox/root?versions=1", "cookie:admin", nil)
	if code != 200 || len(out["versions"].([]any)) != 2 {
		t.Fatalf("versions: %d %+v", code, out)
	}
	code, _ = e.call("DELETE", "/api/v1/secrets/infra/proxmox/root", "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("delete: %d", code)
	}
	code, _ = e.call("GET", "/api/v1/secrets/infra/proxmox/root", "cookie:admin", nil)
	if code != 404 {
		t.Fatalf("after delete expected 404, got %d", code)
	}
}

func TestSecretMoveEndpoint(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/dns/old", "cookie:admin", map[string]any{"value": "secret-val"})

	code, _ := e.call("POST", "/api/v1/move", "cookie:admin",
		map[string]any{"from": "infra/dns/old", "to": "media/plex/moved"})
	if code != 200 {
		t.Fatalf("move: %d", code)
	}
	code, _ = e.call("GET", "/api/v1/secrets/infra/dns/old", "cookie:admin", nil)
	if code != 404 {
		t.Fatalf("old path should be 404, got %d", code)
	}
	code, out := e.call("GET", "/api/v1/secrets/media/plex/moved", "cookie:admin", nil)
	if code != 200 || out["value"] != "secret-val" {
		t.Fatalf("moved secret: %d %+v", code, out)
	}

	// Readonly users cannot move.
	e.call("POST", "/api/v1/users", "cookie:admin",
		map[string]any{"username": "movero", "password": "movero-pass-1", "role": "readonly"})
	e.login("movero", "movero-pass-1")
	code, _ = e.call("POST", "/api/v1/move", "cookie:movero",
		map[string]any{"from": "media/plex/moved", "to": "media/plex/x"})
	if code != 403 {
		t.Fatalf("readonly move expected 403, got %d", code)
	}
}

func TestCredentialEndpoint(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	p := "/api/v1/secrets/HomeLab/Pi/Hush%20Server"

	code, _ := e.call("PUT", p, "cookie:admin", map[string]any{
		"credential": map[string]any{"username": "admin", "password": "sekret", "url": "http://hush.local:4874"},
	})
	if code != 200 {
		t.Fatalf("put credential: %d", code)
	}
	code, out := e.call("GET", p, "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("get: %d", code)
	}
	if out["value"] != nil {
		t.Fatal("credential should not return a plain value")
	}
	cred, ok := out["credential"].(map[string]any)
	if !ok || cred["username"] != "admin" || cred["password"] != "sekret" {
		t.Fatalf("credential response: %+v", out)
	}
	if out["meta"].(map[string]any)["type"] != "credential" {
		t.Fatalf("type: %+v", out["meta"])
	}

	// Rotation replaces the password but keeps the username.
	e.call("POST", "/api/v1/rotate/HomeLab/Pi/Hush%20Server", "cookie:admin", map[string]any{})
	_, out = e.call("GET", p, "cookie:admin", nil)
	cred = out["credential"].(map[string]any)
	if cred["username"] != "admin" {
		t.Fatal("rotation should preserve username")
	}
	if cred["password"] == "sekret" || cred["password"] == "" {
		t.Fatalf("rotation should set a new password, got %q", cred["password"])
	}
}

func TestSpacedPaths(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")

	// Create and read a secret at a spaced path (URL-encoded like the clients do).
	code, _ := e.call("PUT", "/api/v1/secrets/HomeLab/Raspberry%20Pis/hush-server", "cookie:admin",
		map[string]any{"value": "pi-cred"})
	if code != 200 {
		t.Fatalf("put spaced path: %d", code)
	}
	code, out := e.call("GET", "/api/v1/secrets/HomeLab/Raspberry%20Pis/hush-server", "cookie:admin", nil)
	if code != 200 || out["value"] != "pi-cred" {
		t.Fatalf("get spaced path: %d %+v", code, out)
	}

	// The spaced folder appears (decoded) in the tree.
	code, tree := e.call("GET", "/api/v1/tree/HomeLab", "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("tree: %d", code)
	}
	found := false
	for _, f := range tree["folders"].([]any) {
		if f.(map[string]any)["name"] == "Raspberry Pis" {
			found = true
		}
	}
	if !found {
		t.Fatalf("spaced folder not listed: %+v", tree["folders"])
	}
}

func TestReadonlyGrantsAndMethodEnforcement(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/dns/cloudflare", "cookie:admin", map[string]any{"value": "cf"})
	e.call("PUT", "/api/v1/secrets/media/jellyfin/admin", "cookie:admin", map[string]any{"value": "jf"})

	code, _ := e.call("POST", "/api/v1/users", "cookie:admin",
		map[string]any{"username": "viewer", "password": "viewer-pass-1", "role": "readonly"})
	if code != 201 {
		t.Fatalf("create viewer: %d", code)
	}
	code, _ = e.call("POST", "/api/v1/users/viewer/grants", "cookie:admin", map[string]any{"path": "infra"})
	if code != 201 {
		t.Fatalf("grant: %d", code)
	}
	e.login("viewer", "viewer-pass-1")

	// Granted subtree readable.
	code, out := e.call("GET", "/api/v1/secrets/infra/dns/cloudflare", "cookie:viewer", nil)
	if code != 200 || out["value"] != "cf" {
		t.Fatalf("viewer read granted: %d %+v", code, out)
	}
	// Ungranted subtree hidden.
	code, _ = e.call("GET", "/api/v1/secrets/media/jellyfin/admin", "cookie:viewer", nil)
	if code != 404 {
		t.Fatalf("viewer read ungranted expected 404, got %d", code)
	}
	// Any mutation is 403.
	code, _ = e.call("PUT", "/api/v1/secrets/infra/dns/new", "cookie:viewer", map[string]any{"value": "x"})
	if code != 403 {
		t.Fatalf("viewer write expected 403, got %d", code)
	}
	code, _ = e.call("DELETE", "/api/v1/secrets/infra/dns/cloudflare", "cookie:viewer", nil)
	if code != 403 {
		t.Fatalf("viewer delete expected 403, got %d", code)
	}
	// Tree filtered: root shows only infra.
	code, tree := e.call("GET", "/api/v1/tree/", "cookie:viewer", nil)
	if code != 200 {
		t.Fatalf("tree: %d", code)
	}
	folders := tree["folders"].([]any)
	if len(folders) != 1 || folders[0].(map[string]any)["path"] != "infra" {
		t.Fatalf("viewer root tree should only show infra: %+v", folders)
	}
}

func TestAgentTokenScoping(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/dns/cloudflare", "cookie:admin",
		map[string]any{"value": "cf-token"})
	e.call("PUT", "/api/v1/secrets/infra/dns/hetzner", "cookie:admin",
		map[string]any{"value": "hz-token"})
	e.call("PUT", "/api/v1/secrets/media/jellyfin/admin", "cookie:admin",
		map[string]any{"value": "jf"})

	// An agent token lives in infra/dns and reads that folder and everything
	// beneath it, with no per-secret flag to set.
	code, out := e.call("POST", "/api/v1/tokens", "cookie:admin",
		map[string]any{"name": "claude", "type": "agent", "path": "infra/dns"})
	if code != 201 {
		t.Fatalf("token create: %d %+v", code, out)
	}
	token := out["token"].(string)

	// Inside the folder: both siblings are readable, no flag needed.
	code, out = e.call("GET", "/api/v1/secrets/infra/dns/cloudflare", token, nil)
	if code != 200 || out["value"] != "cf-token" {
		t.Fatalf("agent read allowed: %d %+v", code, out)
	}
	code, out = e.call("GET", "/api/v1/secrets/infra/dns/hetzner", token, nil)
	if code != 200 || out["value"] != "hz-token" {
		t.Fatalf("agent cascade read: %d %+v", code, out)
	}
	// Outside the folder: denied.
	code, _ = e.call("GET", "/api/v1/secrets/media/jellyfin/admin", token, nil)
	if code != 404 {
		t.Fatalf("agent read out of folder expected 404, got %d", code)
	}
	// An agent token must name a folder.
	code, _ = e.call("POST", "/api/v1/tokens", "cookie:admin",
		map[string]any{"name": "rootless", "type": "agent"})
	if code != 400 {
		t.Fatalf("agent token without folder expected 400, got %d", code)
	}
	// Agents cannot write, browse, or admin.
	code, _ = e.call("PUT", "/api/v1/secrets/infra/dns/cloudflare", token, map[string]any{"value": "x"})
	if code != 403 {
		t.Fatalf("agent write expected 403, got %d", code)
	}
	code, _ = e.call("GET", "/api/v1/tree/", token, nil)
	if code != 403 {
		t.Fatalf("agent tree expected 403, got %d", code)
	}
	code, _ = e.call("GET", "/api/v1/audit", token, nil)
	if code != 403 {
		t.Fatalf("agent audit expected 403, got %d", code)
	}
}

func TestAuditTrail(t *testing.T) {
	e := newTestEnv(t)
	e.login("admin", "test-admin-password")
	e.call("PUT", "/api/v1/secrets/infra/x/y", "cookie:admin", map[string]any{"value": "v"})
	e.call("GET", "/api/v1/secrets/infra/x/y", "cookie:admin", nil)
	code, _ := e.call("GET", "/api/v1/audit?limit=50", "cookie:admin", nil)
	if code != 200 {
		t.Fatalf("audit: %d", code)
	}
	// audit returns a JSON array; re-fetch raw to assert contents
	req, _ := http.NewRequest("GET", e.ts.URL+"/api/v1/audit?limit=50", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.jar["admin"]})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	json.NewDecoder(resp.Body).Decode(&entries)
	var actions []string
	for _, en := range entries {
		actions = append(actions, fmt.Sprint(en["action"]))
	}
	joined := strings.Join(actions, ",")
	for _, want := range []string{"secret.read", "secret.write", "login", "user.create"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("audit missing %s: %s", want, joined)
		}
	}
}

func TestUnauthenticated(t *testing.T) {
	e := newTestEnv(t)
	code, _ := e.call("GET", "/api/v1/secrets/infra/x/y", "", nil)
	if code != 401 {
		t.Fatalf("expected 401, got %d", code)
	}
	resp, err := http.Get(e.ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz should be public: %d", resp.StatusCode)
	}
}
