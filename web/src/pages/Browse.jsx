import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useMe } from '../App'
import { VaultTabs } from './Shell'
import { AgentPill, Empty, Modal, useToast } from '../components/ui'
import { DeviceAccess } from '../components/DeviceAccess'
import SecretDrawer from './SecretDrawer'

export default function Browse() {
  const { me } = useMe()
  const location = useLocation()
  const navigate = useNavigate()
  const toast = useToast()

  const path = decodeURIComponent(location.pathname.replace(/^\/(browse\/?)?/, '')).replace(/\/$/, '')
  const [tree, setTree] = useState(null)
  const [error, setError] = useState('')
  const [openSecret, setOpenSecret] = useState(null)
  const [newSecret, setNewSecret] = useState(false)
  const [newFolder, setNewFolder] = useState(false)
  const [folderDrawer, setFolderDrawer] = useState(false)

  const load = () => {
    setError('')
    api
      .tree(path)
      .then(setTree)
      .catch((e) => setError(e.message))
  }
  useEffect(() => {
    load()
  }, [path])

  const go = (p) => navigate('/browse/' + p.split('/').map(encodeURIComponent).join('/'))
  const crumbs = path ? path.split('/') : []

  return (
    <>
      <div className="flex h-16 items-center justify-between border-b border-border px-8">
        <VaultTabs active="secrets" />
        <div className="flex items-center gap-2">
          {me.admin && (
            <button className="btn-ghost" onClick={() => setFolderDrawer(true)}>
              Folder access
            </button>
          )}
          {me.admin && (
            <>
              <button className="btn-ghost" onClick={() => setNewFolder(true)}>
                New folder
              </button>
              <button className="btn-primary" onClick={() => setNewSecret(true)}>
                New secret
              </button>
            </>
          )}
        </div>
      </div>

      <div className="p-8">
        {/* Breadcrumbs as a filesystem path */}
        <div className="mb-5 flex items-center gap-1.5 mono text-sm">
          <button onClick={() => go('')} className={path ? 'text-accent-hover hover:underline' : 'text-primary'}>
            vault
          </button>
          {crumbs.map((c, i) => {
            const sub = crumbs.slice(0, i + 1).join('/')
            const last = i === crumbs.length - 1
            return (
              <span key={sub} className="flex items-center gap-1.5">
                <span className="text-muted">/</span>
                <button
                  onClick={() => go(sub)}
                  className={last ? 'text-primary' : 'text-accent-hover hover:underline'}
                >
                  {c}
                </button>
              </span>
            )
          })}
        </div>

        {error && (
          <div className="rounded-card border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
            {error}
          </div>
        )}

        {tree && (
          <div className="card divide-y divide-border overflow-hidden">
            {tree.folders.length === 0 && tree.secrets.length === 0 && (
              <Empty>
                <p className="mono text-sm">This folder is empty.</p>
                {me.admin && <p className="text-xs">Create a secret or folder to fill it.</p>}
              </Empty>
            )}

            {tree.folders.map((f) => (
              <button
                key={f.path}
                onClick={() => go(f.path)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-raised/50"
              >
                <FolderGlyph />
                <span className="mono text-sm text-primary">{f.name}</span>
                <span className="text-muted">/</span>
                <span className="ml-auto text-xs text-muted">open</span>
              </button>
            ))}

            {tree.secrets.map((s) => (
              <button
                key={s.path}
                onClick={() => setOpenSecret(s.path)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-raised/50"
              >
                <SecretGlyph credential={s.type === 'credential'} />
                <span className="mono text-sm text-primary">{s.name}</span>
                <span className="mono text-xs text-muted">v{s.currentVersion}</span>
                <span className="ml-auto flex items-center gap-3">
                  {s.type === 'credential' && (
                    <span className="pill border-border text-secondary">login</span>
                  )}
                  {JSON.parse(s.rotation || '{}').intervalDays > 0 && (
                    <span className="pill border-border text-secondary">
                      rotates {JSON.parse(s.rotation).intervalDays}d
                    </span>
                  )}
                  <AgentPill on={s.agentAccess} />
                </span>
              </button>
            ))}
          </div>
        )}
      </div>

      {openSecret && (
        <SecretDrawer
          path={openSecret}
          canEdit={me.admin}
          onClose={() => setOpenSecret(null)}
          onChanged={load}
        />
      )}
      {folderDrawer && <FolderDrawer path={path} onClose={() => setFolderDrawer(false)} />}
      {newFolder && (
        <NewFolderModal
          base={path}
          onClose={() => setNewFolder(false)}
          onCreated={() => {
            setNewFolder(false)
            load()
            toast('Folder created', 'success')
          }}
        />
      )}
      {newSecret && (
        <NewSecretModal
          base={path}
          onClose={() => setNewSecret(false)}
          onCreated={(p) => {
            setNewSecret(false)
            load()
            toast('Secret saved', 'success')
            setOpenSecret(p)
          }}
        />
      )}
    </>
  )
}

function NewFolderModal({ base, onClose, onCreated }) {
  const [name, setName] = useState('')
  const [err, setErr] = useState('')
  const submit = async (e) => {
    e.preventDefault()
    try {
      await api.createFolder(base ? `${base}/${name}` : name)
      onCreated()
    } catch (e) {
      setErr(e.message)
    }
  }
  return (
    <Modal title="New folder" onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mb-3 text-sm text-secondary">
          Inside <span className="mono text-primary">{base || 'vault'}/</span>
        </p>
        <input className="input mono" autoFocus placeholder="proxmox" value={name} onChange={(e) => setName(e.target.value)} />
        {err && <p className="mt-3 text-sm text-danger">{err}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary" disabled={!name}>
            Create
          </button>
        </div>
      </form>
    </Modal>
  )
}

function genPassword() {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%^*'
  const a = new Uint32Array(24)
  crypto.getRandomValues(a)
  return Array.from(a, (n) => chars[n % chars.length]).join('')
}

function NewSecretModal({ base, onClose, onCreated }) {
  const [name, setName] = useState('')
  const [type, setType] = useState('value')
  const [value, setValue] = useState('')
  const [cred, setCred] = useState({ username: '', password: '', url: '', notes: '' })
  const [agentAccess, setAgentAccess] = useState(false)
  const [err, setErr] = useState('')

  const setField = (k) => (e) => setCred({ ...cred, [k]: e.target.value })

  const submit = async (e) => {
    e.preventDefault()
    if (!base) {
      setErr('Secrets live inside a folder. Open or create one first.')
      return
    }
    try {
      const full = `${base}/${name}`
      if (type === 'credential') {
        await api.setCredential(full, cred, agentAccess)
      } else {
        await api.setSecret(full, value, agentAccess)
      }
      onCreated(full)
    } catch (e) {
      setErr(e.message)
    }
  }

  const canSave = name && base && (type === 'value' ? !!value : !!cred.password)

  return (
    <Modal title="New secret" onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mb-3 text-sm text-secondary">
          Inside <span className="mono text-primary">{base || 'vault'}/</span>
          {!base && <span className="text-warning"> pick a folder first</span>}
        </p>

        <div className="mb-4 grid grid-cols-2 gap-2">
          <TypeCard active={type === 'value'} onClick={() => setType('value')} title="Value" desc="A single secret string." />
          <TypeCard active={type === 'credential'} onClick={() => setType('credential')} title="Credential" desc="Username, password, url, notes." />
        </div>

        <label className="mb-1.5 block text-xs font-medium text-secondary">Name</label>
        <input
          className="input mono mb-4"
          autoFocus
          placeholder={type === 'credential' ? 'Hush Server' : 'root'}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />

        {type === 'value' ? (
          <>
            <label className="mb-1.5 block text-xs font-medium text-secondary">Value</label>
            <div className="flex gap-2">
              <input className="input mono" value={value} onChange={(e) => setValue(e.target.value)} />
              <button type="button" className="btn-ghost shrink-0" onClick={() => setValue(genPassword())}>
                Generate
              </button>
            </div>
          </>
        ) : (
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-xs text-secondary">Username</label>
              <input className="input mono" value={cred.username} onChange={setField('username')} />
            </div>
            <div>
              <label className="mb-1 block text-xs text-secondary">Password</label>
              <div className="flex gap-2">
                <input className="input mono" value={cred.password} onChange={setField('password')} />
                <button type="button" className="btn-ghost shrink-0" onClick={() => setCred({ ...cred, password: genPassword() })}>
                  Generate
                </button>
              </div>
            </div>
            <div>
              <label className="mb-1 block text-xs text-secondary">URL</label>
              <input className="input mono" placeholder="http://hush.local:4874" value={cred.url} onChange={setField('url')} />
            </div>
            <div>
              <label className="mb-1 block text-xs text-secondary">Notes</label>
              <textarea className="input mono min-h-[50px] resize-y" value={cred.notes} onChange={setField('notes')} />
            </div>
          </div>
        )}

        <label className="mt-4 flex items-center gap-2.5 text-sm">
          <input type="checkbox" checked={agentAccess} onChange={(e) => setAgentAccess(e.target.checked)} className="accent-agent" />
          <span className="text-secondary">Allow AI agents and devices to read this</span>
        </label>
        {err && <p className="mt-3 text-sm text-danger">{err}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary" disabled={!canSave}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  )
}

function TypeCard({ active, onClick, title, desc }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-control border p-3 text-left transition-colors ${
        active ? 'border-accent bg-accent/10' : 'border-border hover:border-border-strong'
      }`}
    >
      <p className="text-sm font-medium text-primary">{title}</p>
      <p className="mt-0.5 text-xs text-muted">{desc}</p>
    </button>
  )
}

function FolderDrawer({ path, onClose }) {
  useEffect(() => {
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  const isRoot = !path
  const name = isRoot ? 'vault' : path.split('/').pop()
  const parent = isRoot ? '' : path.split('/').slice(0, -1).join('/')
  return (
    <div className="fixed inset-0 z-30 flex justify-end bg-base/60 backdrop-blur-sm" onMouseDown={onClose}>
      <div
        className="flex h-full w-full max-w-lg flex-col border-l border-border bg-surface"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between border-b border-border p-6">
          <div>
            {parent && <p className="mono text-xs text-muted">{parent}/</p>}
            <h2 className="mono text-xl font-semibold text-primary">{name}/</h2>
          </div>
          <button onClick={onClose} className="text-muted hover:text-primary" aria-label="Close">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>
        <div className="flex-1 space-y-4 overflow-y-auto p-6">
          <p className="text-sm text-secondary">
            {isRoot
              ? 'Devices granted at the vault root can read every secret in the entire vault.'
              : 'Devices granted on this folder can read every secret inside it and its subfolders.'}
          </p>
          <DeviceAccess path={path} />
        </div>
      </div>
    </div>
  )
}

function FolderGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#8F6FFF" strokeWidth="1.8">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
    </svg>
  )
}
function SecretGlyph({ credential }) {
  if (credential) {
    // person-in-a-card, to distinguish a login from a plain value
    return (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#35D0BA" strokeWidth="1.8">
        <circle cx="9" cy="10" r="2.5" />
        <path d="M5 18a4 4 0 0 1 8 0M15 9h4M15 13h3" />
        <rect x="2" y="4" width="20" height="16" rx="2" />
      </svg>
    )
  }
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6E6389" strokeWidth="1.8">
      <rect x="5" y="11" width="14" height="9" rx="2" />
      <path d="M8 11V8a4 4 0 0 1 8 0v3" />
    </svg>
  )
}
