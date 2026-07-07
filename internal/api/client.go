// Package api is the Go client for the Hush HTTP API, used by the CLI. It
// speaks either TCP (with a bearer token) or the local unix socket (implicit
// admin).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/MBarc/hush/internal/store"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

// New returns a client for addr like "http://host:4874" with an optional
// bearer token.
func New(addr, token string) *Client {
	return &Client{
		base:  strings.TrimRight(addr, "/"),
		token: token,
		http:  &http.Client{},
	}
}

// NewSocket returns a client that talks over the local unix socket, which
// the server treats as admin.
func NewSocket(socketPath string) *Client {
	return &Client{
		base: "http://hush.sock",
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("%s (http %d)", e.Msg, e.Status)
	}
	return fmt.Sprintf("http %d", e.Status)
}

// IsNotFound reports whether err is a 404 from the server.
func IsNotFound(err error) bool {
	ae, ok := err.(*apiError)
	return ok && ae.Status == http.StatusNotFound
}

func (c *Client) do(method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return err
		}
		rdr = buf
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		return &apiError{Status: resp.StatusCode, Msg: e.Error}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Health pings /healthz.
func (c *Client) Health() error {
	return c.do("GET", "/healthz", nil, nil)
}

// LoginToken logs in with username/password and mints a named user token
// in one step, returning the plaintext token.
func (c *Client) LoginToken(username, password, tokenName string) (string, error) {
	// Login to get a session cookie on a throwaway jar, then mint a token.
	buf := &bytes.Buffer{}
	json.NewEncoder(buf).Encode(map[string]string{"username": username, "password": password})
	resp, err := c.http.Post(c.base+"/api/v1/auth/login", "application/json", buf)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &apiError{Status: resp.StatusCode, Msg: "invalid username or password"}
	}
	var cookie string
	for _, ck := range resp.Cookies() {
		if ck.Name == "hush_session" {
			cookie = ck.Value
		}
	}
	if cookie == "" {
		return "", fmt.Errorf("no session cookie in login response")
	}
	body := &bytes.Buffer{}
	json.NewEncoder(body).Encode(map[string]any{"name": tokenName, "type": "user"})
	req, _ := http.NewRequest("POST", c.base+"/api/v1/tokens", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "hush_session", Value: cookie})
	tresp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer tresp.Body.Close()
	if tresp.StatusCode != http.StatusCreated {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(tresp.Body).Decode(&e)
		return "", &apiError{Status: tresp.StatusCode, Msg: e.Error}
	}
	var tout struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(tresp.Body).Decode(&tout); err != nil {
		return "", err
	}
	return tout.Token, nil
}

type Me struct {
	Username  string   `json:"username"`
	Role      string   `json:"role"`
	ActorType string   `json:"actorType"`
	TokenType string   `json:"tokenType"`
	Grants    []string `json:"grants"`
	Admin     bool     `json:"admin"`
}

func (c *Client) Me() (Me, error) {
	var m Me
	err := c.do("GET", "/api/v1/auth/me", nil, &m)
	return m, err
}

type Tree struct {
	Path    string             `json:"path"`
	Folders []store.FolderInfo `json:"folders"`
	Secrets []store.SecretMeta `json:"secrets"`
}

func (c *Client) Tree(path string) (Tree, error) {
	var t Tree
	err := c.do("GET", "/api/v1/tree/"+escapePath(path), nil, &t)
	return t, err
}

type SecretValue struct {
	Path    string           `json:"path"`
	Meta    store.SecretMeta `json:"meta"`
	Version int              `json:"version"`
	Value   string           `json:"value"`
}

func (c *Client) GetSecret(path string, version int) (SecretValue, error) {
	url := "/api/v1/secrets/" + escapePath(path)
	if version > 0 {
		url += fmt.Sprintf("?version=%d", version)
	}
	var v SecretValue
	err := c.do("GET", url, nil, &v)
	return v, err
}

func (c *Client) SetSecret(path, value string, agentAccess *bool) (int, error) {
	var out struct {
		Version int `json:"version"`
	}
	body := map[string]any{"value": value}
	if agentAccess != nil {
		body["agentAccess"] = *agentAccess
	}
	err := c.do("PUT", "/api/v1/secrets/"+escapePath(path), body, &out)
	return out.Version, err
}

func (c *Client) SetSecretMeta(path string, agentAccess *bool, rotation json.RawMessage) (store.SecretMeta, error) {
	body := map[string]any{}
	if agentAccess != nil {
		body["agentAccess"] = *agentAccess
	}
	if len(rotation) > 0 {
		body["rotation"] = rotation
	}
	var meta store.SecretMeta
	err := c.do("PATCH", "/api/v1/secrets/"+escapePath(path), body, &meta)
	return meta, err
}

func (c *Client) DeleteSecret(path string) error {
	return c.do("DELETE", "/api/v1/secrets/"+escapePath(path), nil, nil)
}

func (c *Client) Versions(path string) ([]store.VersionMeta, error) {
	var out struct {
		Versions []store.VersionMeta `json:"versions"`
	}
	err := c.do("GET", "/api/v1/secrets/"+escapePath(path)+"?versions=1", nil, &out)
	return out.Versions, err
}

func (c *Client) CreateFolder(path string) error {
	return c.do("POST", "/api/v1/folders", map[string]string{"path": path}, nil)
}

func (c *Client) DeleteFolder(path string, recursive bool) error {
	url := "/api/v1/folders/" + escapePath(path)
	if recursive {
		url += "?recursive=1"
	}
	return c.do("DELETE", url, nil, nil)
}

type CreatedToken struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Scopes    []string `json:"scopes"`
	ExpiresAt int64    `json:"expiresAt"`
	Token     string   `json:"token"`
}

func (c *Client) CreateToken(name, typ string, scopes []string, ttlDays int) (CreatedToken, error) {
	var out CreatedToken
	err := c.do("POST", "/api/v1/tokens", map[string]any{
		"name": name, "type": typ, "scopes": scopes, "ttlDays": ttlDays,
	}, &out)
	return out, err
}

func (c *Client) ListTokens() ([]store.Token, error) {
	var out []store.Token
	err := c.do("GET", "/api/v1/tokens", nil, &out)
	return out, err
}

func (c *Client) DeleteToken(name string) error {
	return c.do("DELETE", "/api/v1/tokens/"+url.PathEscape(name), nil, nil)
}

type CreatedUser struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
}

func (c *Client) CreateUser(username, password, role string) (CreatedUser, error) {
	var out CreatedUser
	err := c.do("POST", "/api/v1/users", map[string]string{
		"username": username, "password": password, "role": role,
	}, &out)
	return out, err
}

func (c *Client) ListUsers() ([]store.User, error) {
	var out []store.User
	err := c.do("GET", "/api/v1/users", nil, &out)
	return out, err
}

func (c *Client) DeleteUser(username string) error {
	return c.do("DELETE", "/api/v1/users/"+url.PathEscape(username), nil, nil)
}

func (c *Client) SetPassword(username, password string) (string, error) {
	var out struct {
		Password string `json:"password"`
	}
	err := c.do("POST", "/api/v1/users/"+url.PathEscape(username)+"/password",
		map[string]string{"password": password}, &out)
	return out.Password, err
}

func (c *Client) Grant(username, path string) error {
	return c.do("POST", "/api/v1/users/"+url.PathEscape(username)+"/grants",
		map[string]string{"path": path}, nil)
}

func (c *Client) Revoke(username, path string) error {
	return c.do("DELETE", "/api/v1/users/"+url.PathEscape(username)+"/grants/"+escapePath(path), nil, nil)
}

func (c *Client) Audit(limit, offset int) ([]store.AuditEntry, error) {
	var out []store.AuditEntry
	err := c.do("GET", fmt.Sprintf("/api/v1/audit?limit=%d&offset=%d", limit, offset), nil, &out)
	return out, err
}

// escapePath escapes each segment but keeps the slashes.
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}
