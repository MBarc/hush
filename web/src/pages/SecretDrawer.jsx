import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { AgentPill, CopyButton, useToast } from '../components/ui'
import { DeviceAccess } from '../components/DeviceAccess'

// The secret detail slides in from the right. Value stays masked until
// revealed, and every mutating control is hidden for non-admins.
export default function SecretDrawer({ path, canEdit, onClose, onChanged }) {
  const toast = useToast()
  const [data, setData] = useState(null)
  const [versions, setVersions] = useState([])
  const [revealed, setRevealed] = useState(false)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [error, setError] = useState('')

  const load = () => {
    setError('')
    api
      .getSecret(path)
      .then((d) => {
        setData(d)
        setDraft(d.value)
      })
      .catch((e) => setError(e.message))
    api.versions(path).then((v) => setVersions(v.versions || [])).catch(() => {})
  }
  useEffect(() => {
    load()
  }, [path])

  useEffect(() => {
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const rotation = data ? JSON.parse(data.meta.rotation || '{}') : {}

  const save = async () => {
    try {
      await api.setSecret(path, draft)
      setEditing(false)
      setRevealed(true)
      load()
      onChanged()
      toast('New version saved', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }
  const rotate = async () => {
    try {
      await api.rotate(path)
      setRevealed(true)
      load()
      onChanged()
      toast('Rotated', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }
  const toggleAgent = async () => {
    try {
      await api.patchSecret(path, { agentAccess: !data.meta.agentAccess })
      load()
      onChanged()
    } catch (e) {
      toast(e.message, 'error')
    }
  }
  const del = async () => {
    if (!confirm(`Delete ${path} and all versions? This cannot be undone.`)) return
    try {
      await api.deleteSecret(path)
      onChanged()
      onClose()
      toast('Secret deleted', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  const name = path.split('/').pop()
  const folder = path.split('/').slice(0, -1).join('/')

  return (
    <div className="fixed inset-0 z-30 flex justify-end bg-base/60 backdrop-blur-sm" onMouseDown={onClose}>
      <div
        className="flex h-full w-full max-w-lg flex-col border-l border-border bg-surface"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between border-b border-border p-6">
          <div>
            <p className="mono text-xs text-muted">{folder}/</p>
            <h2 className="mono text-xl font-semibold text-primary">{name}</h2>
          </div>
          <button onClick={onClose} className="text-muted hover:text-primary" aria-label="Close">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>

        {error && <p className="m-6 text-sm text-danger">{error}</p>}

        {data && (
          <div className="flex-1 space-y-6 overflow-y-auto p-6">
            {data.meta.type === 'credential' ? (
              <CredentialSection
                path={path}
                cred={data.credential || {}}
                version={data.version}
                canEdit={canEdit}
                onSaved={() => {
                  load()
                  onChanged()
                }}
                onRotate={rotate}
              />
            ) : (
            <section>
              <div className="mb-2 flex items-center justify-between">
                <label className="text-xs font-medium uppercase tracking-wide text-muted">
                  Value {data.version ? `(v${data.version})` : ''}
                </label>
                <div className="flex gap-2">
                  {!editing && (
                    <button className="btn-ghost !px-2 !py-1" onClick={() => setRevealed((r) => !r)}>
                      {revealed ? 'Hide' : 'Reveal'}
                    </button>
                  )}
                  {!editing && <CopyButton value={data.value} className="!px-2 !py-1" />}
                </div>
              </div>
              {editing ? (
                <textarea
                  className="input mono min-h-[90px] resize-y"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  autoFocus
                />
              ) : (
                <div className="rounded-control border border-border bg-raised px-3 py-2.5 mono text-sm text-primary break-all">
                  {revealed ? data.value : <MaskedDots />}
                </div>
              )}
              {canEdit && (
                <div className="mt-2 flex gap-2">
                  {editing ? (
                    <>
                      <button className="btn-primary" onClick={save}>
                        Save version
                      </button>
                      <button className="btn-ghost" onClick={() => { setEditing(false); setDraft(data.value) }}>
                        Cancel
                      </button>
                    </>
                  ) : (
                    <>
                      <button className="btn-ghost" onClick={() => setEditing(true)}>
                        Edit
                      </button>
                      <button className="btn-ghost" onClick={rotate}>
                        Rotate now
                      </button>
                    </>
                  )}
                </div>
              )}
            </section>
            )}

            {/* Device access (grants, cascading) */}
            {canEdit && <DeviceAccess path={path} />}

            {/* Agent token access */}
            <section className="card p-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-primary">Agent token access</p>
                  <p className="text-xs text-muted">
                    When on, scoped agent tokens (for CI and AI) can read this secret. Devices use the grants above.
                  </p>
                </div>
                <AgentPill on={data.meta.agentAccess} />
              </div>
              {canEdit && (
                <button
                  className={`mt-3 w-full rounded-control border px-3 py-2 text-sm transition-colors ${
                    data.meta.agentAccess
                      ? 'border-border text-secondary hover:border-border-strong'
                      : 'border-agent/40 bg-agent/10 text-agent hover:bg-agent/20'
                  }`}
                  onClick={toggleAgent}
                >
                  {data.meta.agentAccess ? 'Turn agent token access off' : 'Turn agent token access on'}
                </button>
              )}
            </section>

            {/* Rotation policy */}
            {canEdit && <RotationPanel path={path} rotation={rotation} onSaved={() => { load(); onChanged() }} />}

            {/* Versions */}
            <section>
              <label className="mb-2 block text-xs font-medium uppercase tracking-wide text-muted">
                History
              </label>
              <div className="card divide-y divide-border overflow-hidden">
                {versions.map((v) => (
                  <div key={v.version} className="flex items-center justify-between px-4 py-2.5 text-sm">
                    <span className="mono text-primary">v{v.version}</span>
                    <span className="text-muted">{fmtTime(v.createdAt)}</span>
                    <span className="mono text-xs text-secondary">{v.createdBy}</span>
                  </div>
                ))}
              </div>
            </section>

            {canEdit && (
              <MoveControl
                path={path}
                onMoved={(to) => {
                  onChanged()
                  onClose()
                  toast(`Moved to ${to}`, 'success')
                }}
              />
            )}

            {canEdit && (
              <button className="btn-danger w-full" onClick={del}>
                Delete secret
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function MoveControl({ path, onMoved }) {
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [to, setTo] = useState(path)

  const save = async () => {
    const dest = to.trim()
    if (!dest || dest === path) {
      setOpen(false)
      return
    }
    try {
      await api.moveSecret(path, dest)
      onMoved(dest)
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  return (
    <section className="card p-4">
      <button
        className="flex w-full items-center justify-between"
        onClick={() => {
          setTo(path)
          setOpen((o) => !o)
        }}
      >
        <div className="text-left">
          <p className="text-sm font-medium text-primary">Rename or move</p>
          <p className="text-xs text-muted">Change the path; version history follows.</p>
        </div>
        <span className="text-muted">{open ? 'Close' : 'Edit'}</span>
      </button>
      {open && (
        <div className="mt-3 flex gap-2">
          <input
            className="input mono"
            value={to}
            autoFocus
            onChange={(e) => setTo(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && save()}
          />
          <button className="btn-primary shrink-0" onClick={save}>
            Move
          </button>
        </div>
      )}
    </section>
  )
}

function CredentialSection({ path, cred, version, canEdit, onSaved, onRotate }) {
  const toast = useToast()
  const [editing, setEditing] = useState(false)
  const [reveal, setReveal] = useState(false)
  const [draft, setDraft] = useState(cred)
  useEffect(() => setDraft(cred), [cred])

  const save = async () => {
    try {
      await api.setCredential(path, draft)
      setEditing(false)
      setReveal(true)
      onSaved()
      toast('Credential saved', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  if (editing) {
    const set = (k) => (e) => setDraft({ ...draft, [k]: e.target.value })
    return (
      <section className="space-y-3">
        <label className="text-xs font-medium uppercase tracking-wide text-muted">
          Credential (v{version})
        </label>
        {['username', 'password', 'url'].map((k) => (
          <div key={k}>
            <label className="mb-1 block text-xs capitalize text-secondary">{k}</label>
            <input className="input mono" value={draft[k] || ''} onChange={set(k)} />
          </div>
        ))}
        <div>
          <label className="mb-1 block text-xs text-secondary">Notes</label>
          <textarea
            className="input mono min-h-[60px] resize-y"
            value={draft.notes || ''}
            onChange={set('notes')}
          />
        </div>
        <div className="flex gap-2">
          <button className="btn-primary" onClick={save}>
            Save
          </button>
          <button
            className="btn-ghost"
            onClick={() => {
              setDraft(cred)
              setEditing(false)
            }}
          >
            Cancel
          </button>
        </div>
      </section>
    )
  }

  return (
    <section className="space-y-2">
      <div className="mb-1 flex items-center justify-between">
        <label className="text-xs font-medium uppercase tracking-wide text-muted">
          Credential (v{version})
        </label>
        <button className="btn-ghost !px-2 !py-1" onClick={() => setReveal((r) => !r)}>
          {reveal ? 'Hide' : 'Reveal'}
        </button>
      </div>
      <CredRow label="Username" value={cred.username} />
      <CredRow label="Password" value={cred.password} secret reveal={reveal} />
      {cred.url && <CredRow label="URL" value={cred.url} isUrl />}
      {cred.notes && <CredRow label="Notes" value={cred.notes} />}
      {canEdit && (
        <div className="mt-2 flex gap-2">
          <button
            className="btn-ghost"
            onClick={() => {
              setDraft(cred)
              setEditing(true)
            }}
          >
            Edit
          </button>
          <button className="btn-ghost" onClick={onRotate}>
            Rotate password
          </button>
        </div>
      )}
    </section>
  )
}

function CredRow({ label, value, secret, reveal, isUrl }) {
  const hidden = secret && !reveal
  return (
    <div className="flex items-center gap-2">
      <span className="w-20 shrink-0 text-xs text-muted">{label}</span>
      <div className="flex-1 overflow-hidden rounded-control border border-border bg-raised px-3 py-2 mono text-sm text-primary">
        {hidden ? (
          <MaskedDots />
        ) : isUrl ? (
          <a href={value} target="_blank" rel="noreferrer" className="break-all text-accent-hover hover:underline">
            {value}
          </a>
        ) : (
          <span className="break-all">{value || <span className="text-muted">-</span>}</span>
        )}
      </div>
      {value && <CopyButton value={value} className="!px-2 !py-1 shrink-0" />}
    </div>
  )
}

function MaskedDots() {
  return (
    <span className="inline-flex gap-1 align-middle">
      {Array.from({ length: 12 }).map((_, i) => (
        <span key={i} className="inline-block h-1.5 w-1.5 rounded-full bg-muted" />
      ))}
    </span>
  )
}

function RotationPanel({ path, rotation, onSaved }) {
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [intervalDays, setIntervalDays] = useState(rotation.intervalDays || 0)
  const [length, setLength] = useState(rotation.length || 32)
  const [charset, setCharset] = useState(rotation.charset || 'full')
  const [webhookUrl, setWebhookUrl] = useState(rotation.webhookUrl || '')

  const save = async () => {
    const policy = {}
    if (Number(intervalDays) > 0) policy.intervalDays = Number(intervalDays)
    policy.length = Number(length)
    if (charset !== 'full') policy.charset = charset
    if (webhookUrl) policy.webhookUrl = webhookUrl
    try {
      await api.patchSecret(path, { rotation: policy })
      setOpen(false)
      onSaved()
      toast('Rotation policy saved', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  return (
    <section className="card p-4">
      <button className="flex w-full items-center justify-between" onClick={() => setOpen((o) => !o)}>
        <div className="text-left">
          <p className="text-sm font-medium text-primary">Rotation policy</p>
          <p className="text-xs text-muted">
            {rotation.intervalDays > 0
              ? `Auto-rotates every ${rotation.intervalDays} days`
              : 'Manual only'}
          </p>
        </div>
        <span className="text-muted">{open ? 'Close' : 'Edit'}</span>
      </button>
      {open && (
        <div className="mt-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs text-secondary">Every N days (0 = manual)</label>
              <input className="input" type="number" min="0" value={intervalDays} onChange={(e) => setIntervalDays(e.target.value)} />
            </div>
            <div>
              <label className="mb-1 block text-xs text-secondary">Length</label>
              <input className="input" type="number" min="8" value={length} onChange={(e) => setLength(e.target.value)} />
            </div>
          </div>
          <div>
            <label className="mb-1 block text-xs text-secondary">Charset</label>
            <select className="input" value={charset} onChange={(e) => setCharset(e.target.value)}>
              <option value="full">full (symbols)</option>
              <option value="alnum">alphanumeric</option>
              <option value="hex">hex</option>
              <option value="digits">digits</option>
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs text-secondary">Webhook URL (optional)</label>
            <input className="input mono" placeholder="https://automation.lan/hook" value={webhookUrl} onChange={(e) => setWebhookUrl(e.target.value)} />
          </div>
          <button className="btn-primary w-full" onClick={save}>
            Save policy
          </button>
        </div>
      )}
    </section>
  )
}
