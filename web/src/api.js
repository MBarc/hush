// Thin fetch wrapper for the Hush API. Same origin in production (the UI is
// served from the Go binary); proxied in dev via vite.config.js.

async function req(method, path, body) {
  const opts = {
    method,
    credentials: 'same-origin',
    headers: {},
  }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const err = new Error((data && data.error) || `request failed (${res.status})`)
    err.status = res.status
    throw err
  }
  return data
}

const enc = (p) =>
  p.split('/').map(encodeURIComponent).join('/')

export const api = {
  login: (username, password) => req('POST', '/api/v1/auth/login', { username, password }),
  logout: () => req('POST', '/api/v1/auth/logout'),
  me: () => req('GET', '/api/v1/auth/me'),

  tree: (path = '') => req('GET', '/api/v1/tree/' + enc(path)),
  createFolder: (path) => req('POST', '/api/v1/folders', { path }),
  deleteFolder: (path, recursive) =>
    req('DELETE', '/api/v1/folders/' + enc(path) + (recursive ? '?recursive=1' : '')),

  getSecret: (path, version) =>
    req('GET', '/api/v1/secrets/' + enc(path) + (version ? `?version=${version}` : '')),
  setSecret: (path, value, agentAccess) =>
    req('PUT', '/api/v1/secrets/' + enc(path), agentAccess === undefined ? { value } : { value, agentAccess }),
  patchSecret: (path, patch) => req('PATCH', '/api/v1/secrets/' + enc(path), patch),
  deleteSecret: (path) => req('DELETE', '/api/v1/secrets/' + enc(path)),
  versions: (path) => req('GET', '/api/v1/secrets/' + enc(path) + '?versions=1'),
  rotate: (path) => req('POST', '/api/v1/rotate/' + enc(path), {}),

  tokens: () => req('GET', '/api/v1/tokens'),
  createToken: (body) => req('POST', '/api/v1/tokens', body),
  deleteToken: (name) => req('DELETE', '/api/v1/tokens/' + encodeURIComponent(name)),

  users: () => req('GET', '/api/v1/users'),
  createUser: (body) => req('POST', '/api/v1/users', body),
  deleteUser: (name) => req('DELETE', '/api/v1/users/' + encodeURIComponent(name)),
  setPassword: (name, password) =>
    req('POST', '/api/v1/users/' + encodeURIComponent(name) + '/password', { password }),
  grant: (name, path) =>
    req('POST', '/api/v1/users/' + encodeURIComponent(name) + '/grants', { path }),
  revoke: (name, path) =>
    req('DELETE', '/api/v1/users/' + encodeURIComponent(name) + '/grants/' + enc(path)),

  devices: () => req('GET', '/api/v1/devices'),
  trustDevice: (hostname, body) =>
    req('POST', '/api/v1/devices/' + encodeURIComponent(hostname) + '/trust', body),
  blockDevice: (hostname) =>
    req('POST', '/api/v1/devices/' + encodeURIComponent(hostname) + '/block', {}),
  deleteDevice: (hostname) =>
    req('DELETE', '/api/v1/devices/' + encodeURIComponent(hostname)),

  audit: (limit = 100) => req('GET', `/api/v1/audit?limit=${limit}`),
}

export function fmtTime(unix) {
  if (!unix) return '-'
  return new Date(unix * 1000).toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}
