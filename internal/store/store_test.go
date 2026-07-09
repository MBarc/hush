package store

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/MBarc/hush/internal/crypto"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.TempDir(), key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSetGetSecret(t *testing.T) {
	s := testStore(t)
	v, err := s.SetSecret("infra/proxmox/root", []byte("hunter2"), "test")
	if err != nil || v != 1 {
		t.Fatalf("set: v=%d err=%v", v, err)
	}
	meta, value, err := s.GetSecret("infra/proxmox/root")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(value, []byte("hunter2")) {
		t.Fatalf("value mismatch: %q", value)
	}
	if meta.CurrentVersion != 1 || meta.Name != "root" {
		t.Fatalf("meta wrong: %+v", meta)
	}
}

func TestVersioning(t *testing.T) {
	s := testStore(t)
	s.SetSecret("infra/db/admin", []byte("one"), "test")
	v, _ := s.SetSecret("infra/db/admin", []byte("two"), "rotator")
	if v != 2 {
		t.Fatalf("expected version 2, got %d", v)
	}
	old, err := s.GetSecretVersion("infra/db/admin", 1)
	if err != nil || string(old) != "one" {
		t.Fatalf("old version: %q err=%v", old, err)
	}
	_, cur, _ := s.GetSecret("infra/db/admin")
	if string(cur) != "two" {
		t.Fatalf("current version: %q", cur)
	}
	versions, err := s.ListVersions("infra/db/admin")
	if err != nil || len(versions) != 2 || versions[0].Version != 2 || versions[0].CreatedBy != "rotator" {
		t.Fatalf("versions: %+v err=%v", versions, err)
	}
}

func TestListFolder(t *testing.T) {
	s := testStore(t)
	s.SetSecret("infra/proxmox/root", []byte("a"), "test")
	s.SetSecret("infra/proxmox/api", []byte("b"), "test")
	s.SetSecret("media/jellyfin/admin", []byte("c"), "test")
	s.CreateFolder("infra/dns")

	roots, _, err := s.ListFolder("")
	if err != nil || len(roots) != 2 {
		t.Fatalf("root: %+v err=%v", roots, err)
	}
	subs, secs, err := s.ListFolder("infra")
	if err != nil || len(subs) != 2 || len(secs) != 0 {
		t.Fatalf("infra: subs=%+v secs=%+v err=%v", subs, secs, err)
	}
	_, secs, err = s.ListFolder("infra/proxmox")
	if err != nil || len(secs) != 2 {
		t.Fatalf("proxmox secrets: %+v err=%v", secs, err)
	}
	if _, _, err := s.ListFolder("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := testStore(t)
	s.SetSecret("infra/proxmox/root", []byte("a"), "test")
	if err := s.DeleteFolder("infra", false); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("expected ErrNotEmpty, got %v", err)
	}
	if err := s.DeleteSecret("infra/proxmox/root"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, _, err := s.GetSecret("infra/proxmox/root"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := s.DeleteFolder("infra", true); err != nil {
		t.Fatalf("recursive delete: %v", err)
	}
	if ok, _ := s.FolderExists("infra/proxmox"); ok {
		t.Fatal("cascade should have removed child folders")
	}
}

func TestMoveSecret(t *testing.T) {
	s := testStore(t)
	s.SetSecret("infra/old/name", []byte("v1"), "test")
	s.SetSecret("infra/old/name", []byte("v2"), "test")

	// Rename within the same folder, preserving history.
	if err := s.MoveSecret("infra/old/name", "infra/old/renamed"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, _, err := s.GetSecret("infra/old/name"); !errors.Is(err, ErrNotFound) {
		t.Fatal("old path should be gone")
	}
	meta, val, err := s.GetSecret("infra/old/renamed")
	if err != nil || string(val) != "v2" || meta.CurrentVersion != 2 {
		t.Fatalf("moved secret wrong: %+v %q err=%v", meta, val, err)
	}
	if versions, _ := s.ListVersions("infra/old/renamed"); len(versions) != 2 {
		t.Fatalf("history not preserved: %d versions", len(versions))
	}

	// Move into a brand-new folder tree.
	if err := s.MoveSecret("infra/old/renamed", "archive/deep/here"); err != nil {
		t.Fatalf("cross-folder move: %v", err)
	}
	if ok, _ := s.FolderExists("archive/deep"); !ok {
		t.Fatal("destination folder should be created")
	}

	// Moving onto an existing secret fails.
	s.SetSecret("a/b/c", []byte("x"), "test")
	s.SetSecret("a/b/d", []byte("y"), "test")
	if err := s.MoveSecret("a/b/c", "a/b/d"); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}
	// Moving a missing secret fails.
	if err := s.MoveSecret("no/such/secret", "a/b/z"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPathValidation(t *testing.T) {
	s := testStore(t)
	bad := []string{"has space/x", "../etc/passwd", "a//b", "trailing/.", "-lead/x"}
	for _, p := range bad {
		if _, err := s.SetSecret(p+"/name", []byte("v"), "t"); err == nil {
			t.Fatalf("path %q should be rejected", p)
		}
	}
	if _, err := s.SetSecret("top-level-secret", []byte("v"), "t"); err == nil {
		t.Fatal("secrets outside folders should be rejected")
	}
}

func TestAudit(t *testing.T) {
	s := testStore(t)
	s.Audit(AuditEntry{ActorType: "user", Actor: "admin", Action: "login", IP: "10.0.0.5"})
	s.Audit(AuditEntry{ActorType: "device", Actor: "nas01", Action: "secret.read", Path: "infra/nas/key"})
	entries, err := s.AuditList(10, 0)
	if err != nil || len(entries) != 2 {
		t.Fatalf("audit list: %+v err=%v", entries, err)
	}
	if entries[0].Actor != "nas01" || entries[0].TS == 0 {
		t.Fatalf("newest first expected: %+v", entries[0])
	}
}
