package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrInvalidPath):
		httpError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrExists):
		httpError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrNotEmpty):
		httpError(w, http.StatusConflict, "folder not empty (use recursive)")
	default:
		httpError(w, http.StatusInternalServerError, "internal error")
	}
}

func (s *Server) audit(r *http.Request, action, path, detail string) {
	id := mustIdentity(r)
	s.st.Audit(store.AuditEntry{
		ActorType: id.auditType(), Actor: id.label(), Action: action,
		Path: path, IP: clientIP(r), Detail: detail,
	})
}

// --- health and index ---

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// --- auth ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	u, hash, err := s.st.GetUser(req.Username)
	if err != nil || !auth.VerifyPassword(req.Password, hash) {
		s.st.Audit(store.AuditEntry{ActorType: "user", Actor: req.Username,
			Action: "login.failed", IP: clientIP(r)})
		httpError(w, http.StatusUnauthorized, errBadLogin.Error())
		return
	}
	sid, err := auth.NewSessionID()
	if err != nil {
		storeError(w, err)
		return
	}
	if err := s.st.CreateSession(sid, u.ID, sessionTTL); err != nil {
		storeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sid, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds()),
	})
	s.st.Audit(store.AuditEntry{ActorType: "user", Actor: u.Username, Action: "login", IP: clientIP(r)})
	writeJSON(w, http.StatusOK, map[string]any{
		"username": u.Username, "role": u.Role, "mustChangePassword": u.MustChangePassword,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.st.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	s.audit(r, "logout", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"username": id.username, "role": id.role, "actorType": id.actorType,
		"tokenType": id.tokenType, "grants": id.grants, "admin": id.isAdmin(),
	})
}

// --- tree and folders ---

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	if id.isMachine() {
		httpError(w, http.StatusForbidden, "machine identities fetch secrets directly")
		return
	}
	path, err := store.NormalizePath(r.PathValue("path"))
	if err != nil {
		storeError(w, err)
		return
	}
	folders, secrets, err := s.st.ListFolder(path)
	if err != nil {
		storeError(w, err)
		return
	}
	if !id.isAdmin() {
		full, partial := folderVisibility(id.grants, path)
		if !full && !partial {
			httpError(w, http.StatusNotFound, "not found")
			return
		}
		if !full {
			secrets = nil
			visible := folders[:0]
			for _, f := range folders {
				if fFull, fPartial := folderVisibility(id.grants, f.Path); fFull || fPartial {
					visible = append(visible, f)
				}
			}
			folders = visible
		}
	}
	if folders == nil {
		folders = []store.FolderInfo{}
	}
	if secrets == nil {
		secrets = []store.SecretMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "folders": folders, "secrets": secrets})
}

func (s *Server) handleFolderCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if err := s.st.CreateFolder(req.Path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "folder.create", req.Path, "")
	writeJSON(w, http.StatusCreated, map[string]string{"path": req.Path})
}

func (s *Server) handleFolderDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	recursive := r.URL.Query().Get("recursive") == "1" || r.URL.Query().Get("recursive") == "true"
	if err := s.st.DeleteFolder(path, recursive); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "folder.delete", path, fmt.Sprintf("recursive=%v", recursive))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- secrets ---

func (s *Server) handleSecretGet(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	path := r.PathValue("path")
	meta, err := s.st.GetSecretMeta(path)
	if err != nil {
		storeError(w, err)
		return
	}
	if !canReadSecret(id, meta.Path, meta.AgentAccess) {
		// 404 for unauthorized reads so paths are not probeable.
		s.audit(r, "secret.read.denied", meta.Path, "")
		httpError(w, http.StatusNotFound, "not found")
		return
	}
	q := r.URL.Query()
	if q.Get("versions") != "" {
		versions, err := s.st.ListVersions(meta.Path)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": meta.Path, "versions": versions})
		return
	}
	version := meta.CurrentVersion
	if v := q.Get("version"); v != "" {
		if version, err = strconv.Atoi(v); err != nil {
			httpError(w, http.StatusBadRequest, "bad version")
			return
		}
	}
	value, err := s.st.GetSecretVersion(meta.Path, version)
	if err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "secret.read", meta.Path, fmt.Sprintf("version=%d", version))
	resp := map[string]any{"path": meta.Path, "meta": meta, "version": version}
	if meta.Type == store.SecretTypeCredential {
		var c store.Credential
		json.Unmarshal(value, &c)
		resp["credential"] = c
	} else {
		resp["value"] = string(value)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSecretPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value       string            `json:"value"`
		Credential  *store.Credential `json:"credential"`
		AgentAccess *bool             `json:"agentAccess"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	id := mustIdentity(r)
	path := r.PathValue("path")
	if !id.isAdmin() {
		// Only devices with write access may mutate, and only within a
		// folder or secret they are granted.
		if !id.device || !id.allowWrite {
			httpError(w, http.StatusForbidden, "admin access required")
			return
		}
		normalized, err := store.NormalizePath(path)
		if err != nil {
			storeError(w, err)
			return
		}
		if !grantCovers(id.deviceGrants, normalized) {
			s.audit(r, "secret.write.denied", normalized, "device not granted")
			httpError(w, http.StatusNotFound, "not found")
			return
		}
		req.AgentAccess = nil // devices do not touch the agent-token flag
		req.Credential = nil  // devices write plain values only
	}
	var version int
	var err error
	if req.Credential != nil {
		if !id.isAdmin() {
			httpError(w, http.StatusForbidden, "credentials require admin access")
			return
		}
		version, err = s.st.SetCredential(path, *req.Credential, id.label())
	} else {
		version, err = s.st.SetSecret(path, []byte(req.Value), id.label())
	}
	if err != nil {
		storeError(w, err)
		return
	}
	if req.AgentAccess != nil {
		if err := s.st.SetSecretMeta(path, req.AgentAccess, nil); err != nil {
			storeError(w, err)
			return
		}
	}
	s.audit(r, "secret.write", path, fmt.Sprintf("version=%d", version))
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "version": version})
}

func (s *Server) handleSecretMeta(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentAccess *bool           `json:"agentAccess"`
		Rotation    json.RawMessage `json:"rotation"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	path := r.PathValue("path")
	var rotation *string
	if len(req.Rotation) > 0 {
		rs := string(req.Rotation)
		rotation = &rs
	}
	if err := s.st.SetSecretMeta(path, req.AgentAccess, rotation); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "secret.meta", path, "")
	meta, err := s.st.GetSecretMeta(path)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleSecretMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if err := s.st.MoveSecret(req.From, req.To); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "secret.move", req.To, "from="+req.From)
	writeJSON(w, http.StatusOK, map[string]string{"from": req.From, "to": req.To})
}

func (s *Server) handleSecretDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if err := s.st.DeleteSecret(path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "secret.delete", path, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- tokens ---

func (s *Server) handleTokenList(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.st.ListTokens()
	if err != nil {
		storeError(w, err)
		return
	}
	if tokens == nil {
		tokens = []store.Token{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Scopes  []string `json:"scopes"`
		TTLDays int      `json:"ttlDays"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	id := mustIdentity(r)
	if req.Type == store.TokenTypeAgent {
		if !id.isAdmin() {
			httpError(w, http.StatusForbidden, "agent tokens require admin access")
			return
		}
		if len(req.Scopes) == 0 {
			httpError(w, http.StatusBadRequest, "agent tokens require explicit scopes")
			return
		}
	} else if id.tokenType == store.TokenTypeAgent {
		httpError(w, http.StatusForbidden, "agent tokens cannot mint tokens")
		return
	}
	token, hash, err := auth.NewToken()
	if err != nil {
		storeError(w, err)
		return
	}
	var expiresAt int64
	if req.TTLDays > 0 {
		expiresAt = time.Now().Add(time.Duration(req.TTLDays) * 24 * time.Hour).Unix()
	}
	// User tokens always belong to their creator.
	owner := id.username
	if owner == "" || id.actorType == "socket" {
		owner = "admin"
	}
	if err := s.st.CreateToken(req.Name, req.Type, hash, owner, req.Scopes, expiresAt); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "token.create", "", fmt.Sprintf("name=%s type=%s scopes=%v ttlDays=%d",
		req.Name, req.Type, req.Scopes, req.TTLDays))
	writeJSON(w, http.StatusCreated, map[string]any{
		"name": req.Name, "type": req.Type, "scopes": req.Scopes,
		"expiresAt": expiresAt, "token": token,
		"note": "store this token now, it is never shown again",
	})
}

func (s *Server) handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := mustIdentity(r)
	if !id.isAdmin() {
		// Non-admins may only revoke their own tokens.
		tokens, err := s.st.ListTokens()
		if err != nil {
			storeError(w, err)
			return
		}
		owned := false
		for _, t := range tokens {
			if t.Name == name && t.Owner == id.username && t.Type == store.TokenTypeUser {
				owned = true
				break
			}
		}
		if !owned || id.tokenType == store.TokenTypeAgent {
			httpError(w, http.StatusForbidden, "you may only revoke your own tokens")
			return
		}
	}
	if err := s.st.DeleteToken(name); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "token.delete", "", "name="+name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- users and grants ---

func (s *Server) handleUserList(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers()
	if err != nil {
		storeError(w, err)
		return
	}
	if users == nil {
		users = []store.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	generated := req.Password == ""
	if generated {
		var err error
		if req.Password, err = auth.GeneratePassword(20); err != nil {
			storeError(w, err)
			return
		}
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		storeError(w, err)
		return
	}
	if err := s.st.CreateUser(req.Username, hash, req.Role); err != nil {
		if errors.Is(err, store.ErrExists) {
			storeError(w, err)
		} else {
			httpError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if generated {
		s.st.SetPassword(req.Username, hash, true)
	}
	s.audit(r, "user.create", "", fmt.Sprintf("username=%s role=%s", req.Username, req.Role))
	resp := map[string]any{"username": req.Username, "role": req.Role}
	if generated {
		resp["password"] = req.Password
		resp["note"] = "share this password now, it is never shown again"
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := mustIdentity(r)
	if id.username == name {
		httpError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	if err := s.st.DeleteUser(name); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "user.delete", "", "username="+name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleUserPassword(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := mustIdentity(r)
	if !id.isAdmin() && !(id.actorType == "user" && id.username == name) {
		httpError(w, http.StatusForbidden, "you may only change your own password")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	generated := req.Password == ""
	if generated {
		var err error
		if req.Password, err = auth.GeneratePassword(20); err != nil {
			storeError(w, err)
			return
		}
	}
	if len(req.Password) < 8 {
		httpError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		storeError(w, err)
		return
	}
	mustChange := generated && id.username != name
	if err := s.st.SetPassword(name, hash, mustChange); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "user.password", "", "username="+name)
	resp := map[string]any{"username": name}
	if generated {
		resp["password"] = req.Password
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGrantAdd(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if err := s.st.GrantFolder(name, req.Path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "grant.add", req.Path, "username="+name)
	writeJSON(w, http.StatusCreated, map[string]string{"username": name, "path": req.Path})
}

func (s *Server) handleGrantRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path := r.PathValue("path")
	if err := s.st.RevokeGrant(name, path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "grant.remove", path, "username="+name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// --- audit ---

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, err := s.st.AuditList(limit, offset)
	if err != nil {
		storeError(w, err)
		return
	}
	if entries == nil {
		entries = []store.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
