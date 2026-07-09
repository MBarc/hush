package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

const sessionCookie = "hush_session"
const sessionTTL = 7 * 24 * time.Hour

type identity struct {
	actorType    string // user | token | device | socket
	username     string
	role         string
	tokenName    string
	tokenType    string   // user | agent (when actorType == token)
	grants       []string // readonly user's folder grants
	scopes       []string // agent token's path-glob scopes
	device       bool
	deviceGrants []string // folder/secret paths this device may read
	allowWrite   bool     // device may write within its grants
}

func (id identity) isAdmin() bool {
	return id.role == store.RoleAdmin && id.tokenType != store.TokenTypeAgent
}

// label is the actor string recorded in the audit log.
func (id identity) label() string {
	switch id.actorType {
	case "token":
		return "token:" + id.tokenName
	case "device":
		return "device:" + id.username
	case "socket":
		return "local-admin"
	default:
		return id.username
	}
}

// isMachine reports whether the caller is automation (agent token or
// device) rather than a person.
func (id identity) isMachine() bool {
	return id.device || id.tokenType == store.TokenTypeAgent
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
		if claim := r.Header.Get("X-Hush-Device"); claim != "" {
			if id, ok := s.deviceIdentity(r, claim); ok {
				next(w, r.WithContext(withIdentity(r.Context(), id)))
				return
			}
			httpError(w, http.StatusUnauthorized, "device not authorized")
			return
		}
		httpError(w, http.StatusUnauthorized, "authentication required")
	}
}

// deviceIdentity validates a hostname claim. The device must be known,
// trusted, unexpired, and the request must come from the IP the poller
// last saw that hostname at. The raw connection address is used on
// purpose: X-Forwarded-For is client-controlled and spoofable.
func (s *Server) deviceIdentity(r *http.Request, claim string) (identity, bool) {
	deny := func(reason string) (identity, bool) {
		s.st.Audit(store.AuditEntry{ActorType: "device", Actor: "device:" + claim,
			Action: "device.denied", IP: connIP(r), Detail: reason})
		return identity{}, false
	}
	d, err := s.st.GetDevice(claim)
	if err != nil {
		return deny("unknown device")
	}
	if d.Status == store.DeviceBlocked {
		return deny("device blocked")
	}
	if d.ExpiresAt > 0 && d.ExpiresAt <= time.Now().Unix() {
		return deny("device access expired")
	}
	if ip := connIP(r); ip != d.IP {
		return deny(fmt.Sprintf("claimed %s but connected from %s (expected %s)", d.Hostname, ip, d.IP))
	}
	grants, err := s.st.ListDeviceGrants(d.Hostname)
	if err != nil {
		return deny("could not load device grants")
	}
	return identity{
		actorType: "device", username: d.Hostname, device: true,
		deviceGrants: grants, allowWrite: d.AllowWrite,
	}, true
}

// connIP is the raw connection peer address, never X-Forwarded-For.
func connIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

// canReadSecret decides whether id may read the secret at path.
//   - a device may read a path it is granted (directly or via an ancestor
//     folder); the per-secret agent flag does not apply to devices.
//   - an agent token needs the per-secret agent flag on and a matching scope.
//   - admins read anything; readonly users read within their folder grants.
func canReadSecret(id identity, path string, agentAccess bool) bool {
	if id.device {
		return grantCovers(id.deviceGrants, path)
	}
	if id.tokenType == store.TokenTypeAgent {
		return agentAccess && auth.MatchAnyScope(id.scopes, path)
	}
	if id.isAdmin() {
		return true
	}
	return grantCovers(id.grants, path)
}

// grantCovers reports whether any grant covers path: an empty grant is the
// vault root and covers everything, otherwise the grant must equal path or
// be an ancestor folder of it.
func grantCovers(grants []string, path string) bool {
	for _, g := range grants {
		if g == "" || path == g || strings.HasPrefix(path, g+"/") {
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
