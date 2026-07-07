import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { useMe } from '../App'
import { PageHeader } from './Shell'
import { CopyButton, Empty, Modal, RoleBadge, useToast } from '../components/ui'

export default function Users() {
  const { me } = useMe()
  const toast = useToast()
  const [users, setUsers] = useState(null)
  const [creating, setCreating] = useState(false)
  const [granting, setGranting] = useState(null)
  const [fresh, setFresh] = useState(null)

  const load = () => api.users().then(setUsers).catch(() => setUsers([]))
  useEffect(load, [])

  const del = async (name) => {
    if (!confirm(`Delete user ${name}?`)) return
    await api.deleteUser(name).catch((e) => toast(e.message, 'error'))
    load()
  }

  return (
    <>
      <PageHeader title="Users" subtitle="Local accounts and folder access">
        <button className="btn-primary" onClick={() => setCreating(true)}>
          New user
        </button>
      </PageHeader>

      <div className="p-8">
        {users && users.length === 0 && (
          <Empty>
            <p className="mono text-sm">No users.</p>
          </Empty>
        )}
        {users && users.length > 0 && (
          <div className="card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted">
                  <th className="px-4 py-3 font-medium">Username</th>
                  <th className="px-4 py-3 font-medium">Role</th>
                  <th className="px-4 py-3 font-medium">Granted folders</th>
                  <th className="px-4 py-3 font-medium">Created</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {users.map((u) => (
                  <tr key={u.username}>
                    <td className="px-4 py-3 mono text-primary">{u.username}</td>
                    <td className="px-4 py-3">
                      <RoleBadge role={u.role} />
                    </td>
                    <td className="px-4 py-3">
                      {u.role === 'admin' ? (
                        <span className="text-xs text-muted">everything</span>
                      ) : u.grants && u.grants.length ? (
                        <div className="flex flex-wrap gap-1.5">
                          {u.grants.map((g) => (
                            <span key={g} className="pill border-border mono text-secondary">
                              {g}/
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-muted">none</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-muted">{fmtTime(u.createdAt)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-3">
                        {u.role === 'readonly' && (
                          <button className="text-accent-hover hover:underline" onClick={() => setGranting(u)}>
                            Grants
                          </button>
                        )}
                        {u.username !== me.username && (
                          <button className="text-danger hover:underline" onClick={() => del(u.username)}>
                            Delete
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {creating && (
        <CreateUser
          onClose={() => setCreating(false)}
          onCreated={(u) => {
            setCreating(false)
            load()
            if (u.password) setFresh(u)
          }}
        />
      )}
      {granting && (
        <GrantsModal
          user={granting}
          onClose={() => setGranting(null)}
          onChanged={() => {
            load()
          }}
        />
      )}
      {fresh && (
        <Modal title="User created" onClose={() => setFresh(null)}>
          <p className="mb-3 text-sm text-secondary">
            Share this password with <span className="mono text-primary">{fresh.username}</span> now.
            It is never shown again.
          </p>
          <div className="rounded-control border border-border bg-raised px-3 py-2.5 mono text-sm text-primary break-all">
            {fresh.password}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <CopyButton value={fresh.password} label="Copy password" />
            <button className="btn-primary" onClick={() => setFresh(null)}>
              Done
            </button>
          </div>
        </Modal>
      )}
    </>
  )
}

function CreateUser({ onClose, onCreated }) {
  const [username, setUsername] = useState('')
  const [role, setRole] = useState('readonly')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')

  const submit = async (e) => {
    e.preventDefault()
    try {
      onCreated(await api.createUser({ username, role, password }))
    } catch (e) {
      setErr(e.message)
    }
  }
  return (
    <Modal title="New user" onClose={onClose}>
      <form onSubmit={submit}>
        <label className="mb-1.5 block text-xs font-medium text-secondary">Username</label>
        <input className="input mono mb-4" autoFocus value={username} onChange={(e) => setUsername(e.target.value)} />

        <label className="mb-1.5 block text-xs font-medium text-secondary">Role</label>
        <div className="mb-4 grid grid-cols-2 gap-2">
          <RoleCard active={role === 'readonly'} onClick={() => setRole('readonly')} title="Read only" desc="Sees granted folders. GET only." />
          <RoleCard active={role === 'admin'} onClick={() => setRole('admin')} title="Admin" desc="Full control of the vault." />
        </div>

        <label className="mb-1.5 block text-xs font-medium text-secondary">
          Password <span className="text-muted">(leave blank to generate)</span>
        </label>
        <input className="input mono mb-4" type="text" value={password} onChange={(e) => setPassword(e.target.value)} />

        {err && <p className="mb-3 text-sm text-danger">{err}</p>}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary" disabled={!username}>
            Create
          </button>
        </div>
      </form>
    </Modal>
  )
}

function RoleCard({ active, onClick, title, desc }) {
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

function GrantsModal({ user, onClose, onChanged }) {
  const toast = useToast()
  const [grants, setGrants] = useState(user.grants || [])
  const [path, setPath] = useState('')

  const add = async (e) => {
    e.preventDefault()
    if (!path) return
    try {
      await api.grant(user.username, path)
      setGrants((g) => [...new Set([...g, path.replace(/\/$/, '')])])
      setPath('')
      onChanged()
    } catch (e) {
      toast(e.message, 'error')
    }
  }
  const remove = async (p) => {
    try {
      await api.revoke(user.username, p)
      setGrants((g) => g.filter((x) => x !== p))
      onChanged()
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  return (
    <Modal title={`Grants for ${user.username}`} onClose={onClose}>
      <p className="mb-4 text-sm text-secondary">
        A grant on a folder cascades to everything beneath it.
      </p>
      <div className="mb-4 space-y-2">
        {grants.length === 0 && <p className="text-sm text-muted">No folders granted yet.</p>}
        {grants.map((g) => (
          <div key={g} className="flex items-center justify-between rounded-control border border-border bg-raised px-3 py-2">
            <span className="mono text-sm text-primary">{g}/</span>
            <button className="text-danger hover:underline" onClick={() => remove(g)}>
              Revoke
            </button>
          </div>
        ))}
      </div>
      <form onSubmit={add} className="flex gap-2">
        <input className="input mono" placeholder="infra/dns" value={path} onChange={(e) => setPath(e.target.value)} />
        <button className="btn-primary shrink-0" disabled={!path}>
          Grant
        </button>
      </form>
    </Modal>
  )
}
