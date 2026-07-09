// Package server implements Hush's HTTP API, auth middleware, and the
// embedded web UI. The same handler tree serves the TCP listener and the
// local unix socket (which is implicitly admin).
package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

type Server struct {
	st      *store.Store
	version string
	mux     *http.ServeMux
}

func New(st *store.Store, version string) (*Server, error) {
	s := &Server{st: st, version: version}
	if err := s.bootstrap(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}

// bootstrap creates the first admin account on an empty vault. The
// password comes from HUSH_ADMIN_PASSWORD or is generated and printed to
// the container logs exactly once.
func (s *Server) bootstrap() error {
	n, err := s.st.CountUsers()
	if err != nil || n > 0 {
		return err
	}
	password := os.Getenv("HUSH_ADMIN_PASSWORD")
	generated := password == ""
	if generated {
		if password, err = auth.GeneratePassword(20); err != nil {
			return err
		}
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.st.CreateUser("admin", hash, store.RoleAdmin); err != nil {
		return err
	}
	s.st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush", Action: "user.create", Detail: "bootstrap admin"})
	if generated {
		log.Printf("=========================================================")
		log.Printf("  first boot: created web user 'admin'")
		log.Printf("  password: %s", password)
		log.Printf("  change it after logging in. this is printed only once.")
		log.Printf("=========================================================")
	} else {
		log.Printf("first boot: created web user 'admin' with password from HUSH_ADMIN_PASSWORD")
	}
	return nil
}

func (s *Server) routes() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	if ui := uiHandler(); ui != nil {
		mux.Handle("GET /", ui)
	} else {
		mux.HandleFunc("GET /", s.handleIndex) // placeholder before a UI build
	}

	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.auth(s.handleLogout))
	mux.HandleFunc("GET /api/v1/auth/me", s.auth(s.handleMe))

	mux.HandleFunc("GET /api/v1/tree/{path...}", s.auth(s.handleTree))
	mux.HandleFunc("POST /api/v1/folders", s.auth(s.adminOnly(s.handleFolderCreate)))
	mux.HandleFunc("DELETE /api/v1/folders/{path...}", s.auth(s.adminOnly(s.handleFolderDelete)))

	mux.HandleFunc("GET /api/v1/secrets/{path...}", s.auth(s.handleSecretGet))
	mux.HandleFunc("PUT /api/v1/secrets/{path...}", s.auth(s.handleSecretPut))
	mux.HandleFunc("POST /api/v1/secrets/{path...}", s.auth(s.handleSecretPut)) // devices and simple clients POST
	mux.HandleFunc("POST /api/v1/rotate/{path...}", s.auth(s.adminOnly(s.handleRotate)))
	mux.HandleFunc("POST /api/v1/move", s.auth(s.adminOnly(s.handleSecretMove)))
	mux.HandleFunc("PATCH /api/v1/secrets/{path...}", s.auth(s.adminOnly(s.handleSecretMeta)))
	mux.HandleFunc("DELETE /api/v1/secrets/{path...}", s.auth(s.adminOnly(s.handleSecretDelete)))

	mux.HandleFunc("GET /api/v1/tokens", s.auth(s.adminOnly(s.handleTokenList)))
	mux.HandleFunc("POST /api/v1/tokens", s.auth(s.handleTokenCreate))
	mux.HandleFunc("DELETE /api/v1/tokens/{name}", s.auth(s.handleTokenDelete))

	mux.HandleFunc("GET /api/v1/users", s.auth(s.adminOnly(s.handleUserList)))
	mux.HandleFunc("POST /api/v1/users", s.auth(s.adminOnly(s.handleUserCreate)))
	mux.HandleFunc("DELETE /api/v1/users/{name}", s.auth(s.adminOnly(s.handleUserDelete)))
	mux.HandleFunc("POST /api/v1/users/{name}/password", s.auth(s.handleUserPassword))
	mux.HandleFunc("POST /api/v1/users/{name}/grants", s.auth(s.adminOnly(s.handleGrantAdd)))
	mux.HandleFunc("DELETE /api/v1/users/{name}/grants/{path...}", s.auth(s.adminOnly(s.handleGrantRemove)))

	mux.HandleFunc("GET /api/v1/devices", s.auth(s.adminOnly(s.handleDeviceList)))
	mux.HandleFunc("GET /api/v1/grants/{path...}", s.auth(s.adminOnly(s.handlePathGrants)))
	mux.HandleFunc("POST /api/v1/grants/{path...}", s.auth(s.adminOnly(s.handlePathGrantAdd)))
	mux.HandleFunc("DELETE /api/v1/grants/{path...}", s.auth(s.adminOnly(s.handlePathGrantRemove)))
	mux.HandleFunc("PATCH /api/v1/devices/{hostname}", s.auth(s.adminOnly(s.handleDeviceName)))
	mux.HandleFunc("POST /api/v1/devices/{hostname}/trust", s.auth(s.adminOnly(s.handleDeviceTrust)))
	mux.HandleFunc("POST /api/v1/devices/{hostname}/block", s.auth(s.adminOnly(s.handleDeviceBlock)))
	mux.HandleFunc("DELETE /api/v1/devices/{hostname}", s.auth(s.adminOnly(s.handleDeviceDelete)))

	mux.HandleFunc("GET /api/v1/audit", s.auth(s.adminOnly(s.handleAudit)))
	s.mux = mux
}

type ctxKey int

const identityKey ctxKey = 1

// socketHandler wraps the mux marking every request as local-admin. It is
// only attached to the unix socket listener.
func (s *Server) socketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := identity{actorType: "socket", username: "local-admin", role: store.RoleAdmin}
		s.mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey, id)))
	})
}

// Run serves the API on addr and, when socketPath is non-empty, on a unix
// socket with implicit admin identity. When certFile and keyFile are set it
// serves HTTPS. Blocks until the TCP listener stops.
func (s *Server) Run(addr, socketPath, certFile, keyFile string) error {
	if socketPath != "" {
		if err := s.serveSocket(socketPath); err != nil {
			log.Printf("warning: local admin socket unavailable: %v", err)
		}
	}
	if certFile != "" && keyFile != "" {
		log.Printf("hush %s listening on %s (https)", s.version, addr)
		return http.ListenAndServeTLS(addr, certFile, keyFile, s.mux)
	}
	log.Printf("hush %s listening on %s", s.version, addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) serveSocket(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return err
	}
	go func() {
		log.Printf("local admin socket on %s", path)
		if err := http.Serve(ln, s.socketHandler()); err != nil {
			log.Printf("socket listener stopped: %v", err)
		}
	}()
	return nil
}

// Handler exposes the full handler tree (used by tests).
func (s *Server) Handler() http.Handler { return s.mux }

// clientIP extracts the caller address, honoring the first X-Forwarded-For
// hop when a reverse proxy is in front.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if r.RemoteAddr == "" || r.RemoteAddr == "@" {
			return "local" // unix socket
		}
		return r.RemoteAddr
	}
	return host
}
