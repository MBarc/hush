package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

const sessionCookie = "hush_session"
const sessionTTL = 7 * 24 * time.Hour

type identity struct {
	actorType string // user | token | socket
	username  string
	role      string
	tokenName string
	tokenType string // user | agent (when actorType == token)
	grants    []string
	scopes    []string
}

func (id identity) isAdmin() bool {
	return id.role == store.RoleAdmin && id.tokenType != store.TokenTypeAgent
}

// label is the actor string recorded in the audit log.
func (id identity) label() string {
	switch id.actorType {
	case "token":
		return "token:" + id.tokenName
	case "socket":
		return "local-admin"
	default:
		return id.username
	}
}

func (id identity) auditType() string {
	if id.actorType == "socket" {
		return "user"
	}
	return id.actorType
}

// auth resolves the caller identity: unix-socket admin (set upstream),
// session cookie, or bearer token. Unauthenticated requests get 401.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(identityKey).(identity); ok {
			next(w, r) // socket identity already attached
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil {
			if u, err := s.st.SessionUser(c.Value); err == nil {
				id := identity{actorType: "user", username: u.Username, role: u.Role, grants: u.Grants}
				next(w, r.WithContext(withIdentity(r.Context(), id)))
				return
			}
		}
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			raw := strings.TrimPrefix(h, "Bearer ")
			if auth.LooksLikeToken(raw) {
				tok, owner, err := s.st.TokenByHash(auth.HashToken(raw))
				if err == nil {
					id := identity{
						actorType: "token", username: owner.Username, role: owner.Role,
						tokenName: tok.Name, tokenType: tok.Type, grants: owner.Grants, scopes: tok.Scopes,
					}
					next(w, r.WithContext(withIdentity(r.Context(), id)))
					return
				}
			}
		}
		httpError(w, http.StatusUnauthorized, "authentication required")
	}
}

// adminOnly rejects non-admin identities. Combined with the router only
// mapping mutating methods through it, this enforces "readonly users and
// user-tokens may only GET; agent tokens may only GET".
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mustIdentity(r)
		if !id.isAdmin() {
			httpError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

// canReadSecret decides whether id may read the secret at path with the
// given agent-access flag.
func canReadSecret(id identity, path string, agentAccess bool) bool {
	if id.tokenType == store.TokenTypeAgent {
		return agentAccess && auth.MatchAnyScope(id.scopes, path)
	}
	if id.isAdmin() {
		return true
	}
	return grantCovers(id.grants, path)
}

// grantCovers reports whether any granted folder is an ancestor of path.
func grantCovers(grants []string, path string) bool {
	for _, g := range grants {
		if path == g || strings.HasPrefix(path, g+"/") {
			return true
		}
	}
	return false
}

// folderVisibility classifies a folder for tree listings: full (inside a
// grant), partial (an ancestor on the way down to a grant), or hidden.
func folderVisibility(grants []string, folder string) (full, partial bool) {
	if grantCovers(grants, folder) {
		return true, false
	}
	for _, g := range grants {
		if folder == "" || g == folder || strings.HasPrefix(g, folder+"/") {
			return false, true
		}
	}
	return false, false
}

func withIdentity(ctx context.Context, id identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func mustIdentity(r *http.Request) identity {
	id, _ := r.Context().Value(identityKey).(identity)
	return id
}

var errBadLogin = errors.New("invalid username or password")
