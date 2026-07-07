import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useMe } from '../App'
import { PageHeader } from './Shell'
import { AgentPill, Empty, Modal, useToast } from '../components/ui'
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

  const load = () => {
    setError('')
    api
      .tree(path)
      .then(setTree)
      .catch((e) => setError(e.message))
  }
  useEffect(load, [path])

  const go = (p) => navigate('/browse/' + p.split('/').map(encodeURIComponent).join('/'))
  const crumbs = path ? path.split('/') : []

  return (
    <>
      <PageHeader title="Secrets" subtitle={me.admin ? 'Full vault access' : 'Your granted folders'}>
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
      </PageHeader>

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
                <SecretGlyph />
                <span className="mono text-sm text-primary">{s.name}</span>
                <span className="mono text-xs text-muted">v{s.currentVersion}</span>
                <span className="ml-auto flex items-center gap-3">
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

function NewSecretModal({ base, onClose, onCreated }) {
  const [name, setName] = useState('')
  const [value, setValue] = useState('')
  const [agentAccess, setAgentAccess] = useState(false)
  const [err, setErr] = useState('')
  const gen = () => {
    const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%^*'
    const a = new Uint32Array(24)
    crypto.getRandomValues(a)
    setValue(Array.from(a, (n) => chars[n % chars.length]).join(''))
  }
  const submit = async (e) => {
    e.preventDefault()
    const folder = base
    if (!folder) {
      setErr('Secrets live inside a folder. Open or create one first.')
      return
    }
    try {
      const full = `${folder}/${name}`
      await api.setSecret(full, value, agentAccess)
      onCreated(full)
    } catch (e) {
      setErr(e.message)
    }
  }
  return (
    <Modal title="New secret" onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mb-3 text-sm text-secondary">
          Inside <span className="mono text-primary">{base || 'vault'}/</span>
          {!base && <span className="text-warning"> pick a folder first</span>}
        </p>
        <label className="mb-1.5 block text-xs font-medium text-secondary">Name</label>
        <input className="input mono mb-4" autoFocus placeholder="root" value={name} onChange={(e) => setName(e.target.value)} />
        <label className="mb-1.5 block text-xs font-medium text-secondary">Value</label>
        <div className="flex gap-2">
          <input className="input mono" value={value} onChange={(e) => setValue(e.target.value)} />
          <button type="button" className="btn-ghost shrink-0" onClick={gen}>
            Generate
          </button>
        </div>
        <label className="mt-4 flex items-center gap-2.5 text-sm">
          <input type="checkbox" checked={agentAccess} onChange={(e) => setAgentAccess(e.target.checked)} className="accent-agent" />
          <span className="text-secondary">Allow AI agents and devices to read this</span>
        </label>
        {err && <p className="mt-3 text-sm text-danger">{err}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary" disabled={!name || !value || !base}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  )
}

function FolderGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#8F6FFF" strokeWidth="1.8">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
    </svg>
  )
}
function SecretGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6E6389" strokeWidth="1.8">
      <rect x="5" y="11" width="14" height="9" rx="2" />
      <path d="M8 11V8a4 4 0 0 1 8 0v3" />
    </svg>
  )
}
