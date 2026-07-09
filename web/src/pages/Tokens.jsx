import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { useMe } from '../App'
import { VaultTabs } from './Shell'
import { CopyButton, Empty, Modal, useToast } from '../components/ui'

export default function Tokens() {
  const { me } = useMe()
  const toast = useToast()
  const [tokens, setTokens] = useState(null)
  const [creating, setCreating] = useState(false)
  const [fresh, setFresh] = useState(null)

  const load = () => api.tokens().then(setTokens).catch(() => setTokens([]))
  useEffect(() => {
    load()
  }, [])

  const del = async (name) => {
    if (!confirm(`Revoke token ${name}?`)) return
    await api.deleteToken(name).catch((e) => toast(e.message, 'error'))
    load()
  }

  return (
    <>
      <div className="flex h-16 items-center justify-between border-b border-border px-8">
        <VaultTabs active="tokens" />
        <button className="btn-primary" onClick={() => setCreating(true)}>
          New token
        </button>
      </div>

      <div className="p-8">
        {tokens && tokens.length === 0 && (
          <Empty>
            <p className="mono text-sm">No tokens yet.</p>
            <p className="text-xs">Create a user token for the CLI, or an agent token for automation.</p>
          </Empty>
        )}
        {tokens && tokens.length > 0 && (
          <div className="card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted">
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 font-medium">Scopes</th>
                  <th className="px-4 py-3 font-medium">Expires</th>
                  <th className="px-4 py-3 font-medium">Last used</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {tokens.map((t) => (
                  <tr key={t.name}>
                    <td className="px-4 py-3 mono text-primary">{t.name}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`pill ${
                          t.type === 'agent'
                            ? 'border-agent/40 bg-agent/10 text-agent'
                            : 'border-border text-secondary'
                        }`}
                      >
                        {t.type}
                      </span>
                    </td>
                    <td className="px-4 py-3 mono text-xs text-secondary">
                      {t.scopes && t.scopes.length ? t.scopes.join(', ') : '-'}
                    </td>
                    <td className="px-4 py-3 text-muted">{fmtTime(t.expiresAt)}</td>
                    <td className="px-4 py-3 text-muted">{fmtTime(t.lastUsedAt)}</td>
                    <td className="px-4 py-3 text-right">
                      <button className="text-danger hover:underline" onClick={() => del(t.name)}>
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {creating && (
        <CreateToken
          isAdmin={me.admin}
          onClose={() => setCreating(false)}
          onCreated={(t) => {
            setCreating(false)
            setFresh(t)
            load()
          }}
        />
      )}
      {fresh && <RevealToken token={fresh} onClose={() => setFresh(null)} />}
    </>
  )
}

function CreateToken({ isAdmin, onClose, onCreated }) {
  const [name, setName] = useState('')
  const [type, setType] = useState('user')
  const [scopes, setScopes] = useState('')
  const [ttlDays, setTtlDays] = useState(0)
  const [err, setErr] = useState('')

  const submit = async (e) => {
    e.preventDefault()
    const body = { name, type, ttlDays: Number(ttlDays) }
    if (type === 'agent') body.scopes = scopes.split(',').map((s) => s.trim()).filter(Boolean)
    try {
      onCreated(await api.createToken(body))
    } catch (e) {
      setErr(e.message)
    }
  }

  return (
    <Modal title="New token" onClose={onClose}>
      <form onSubmit={submit}>
        <label className="mb-1.5 block text-xs font-medium text-secondary">Name</label>
        <input className="input mono mb-4" autoFocus placeholder="ci-deploy" value={name} onChange={(e) => setName(e.target.value)} />

        <label className="mb-1.5 block text-xs font-medium text-secondary">Type</label>
        <div className="mb-4 grid grid-cols-2 gap-2">
          <TypeCard active={type === 'user'} onClick={() => setType('user')} title="User" desc="Acts as you. For the CLI." tone="violet" />
          <TypeCard
            active={type === 'agent'}
            onClick={() => isAdmin && setType('agent')}
            title="Agent"
            desc={isAdmin ? 'GET-only, path-scoped. For AI.' : 'Admin only'}
            tone="agent"
            disabled={!isAdmin}
          />
        </div>

        {type === 'agent' && (
          <>
            <label className="mb-1.5 block text-xs font-medium text-secondary">
              Scopes (comma-separated globs)
            </label>
            <input className="input mono mb-4" placeholder="infra/dns/*, media/*" value={scopes} onChange={(e) => setScopes(e.target.value)} />
          </>
        )}

        <label className="mb-1.5 block text-xs font-medium text-secondary">Expire after (days, 0 = never)</label>
        <input className="input mb-4" type="number" min="0" value={ttlDays} onChange={(e) => setTtlDays(e.target.value)} />

        {err && <p className="mb-3 text-sm text-danger">{err}</p>}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary" disabled={!name || (type === 'agent' && !scopes.trim())}>
            Create
          </button>
        </div>
      </form>
    </Modal>
  )
}

function TypeCard({ active, onClick, title, desc, tone, disabled }) {
  const activeCls =
    tone === 'agent' ? 'border-agent bg-agent/10' : 'border-accent bg-accent/10'
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`rounded-control border p-3 text-left transition-colors disabled:opacity-40 ${
        active ? activeCls : 'border-border hover:border-border-strong'
      }`}
    >
      <p className="text-sm font-medium text-primary">{title}</p>
      <p className="mt-0.5 text-xs text-muted">{desc}</p>
    </button>
  )
}

function RevealToken({ token, onClose }) {
  return (
    <Modal title="Token created" onClose={onClose}>
      <p className="mb-3 text-sm text-secondary">
        Copy this now. For your security, it is never shown again.
      </p>
      <div className="rounded-control border border-border bg-raised px-3 py-2.5 mono text-sm text-agent break-all">
        {token.token}
      </div>
      <div className="mt-4 flex justify-end gap-2">
        <CopyButton value={token.token} label="Copy token" />
        <button className="btn-primary" onClick={onClose}>
          Done
        </button>
      </div>
    </Modal>
  )
}
