import { useState } from 'react'
import { api } from '../api'
import { Logo } from '../components/ui'

export default function Login({ onLogin }) {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.login(username, password)
      onLogin(await api.me())
    } catch (err) {
      setError(err.message)
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      {/* Ambient violet glow, kept faint so the vault stays hushed */}
      <div
        className="pointer-events-none fixed inset-0 opacity-60"
        style={{
          background:
            'radial-gradient(600px circle at 50% 30%, rgba(95,61,214,0.18), transparent 70%)',
        }}
      />
      <form onSubmit={submit} className="card relative w-full max-w-sm p-8">
        <div className="mb-7 flex flex-col items-center gap-3 text-center">
          <Logo size={44} />
          <div>
            <h1 className="text-2xl font-bold tracking-tight">hush</h1>
            <p className="mt-1 text-sm text-secondary">A quiet little vault for your homelab.</p>
          </div>
        </div>

        <label className="mb-1.5 block text-xs font-medium text-secondary">Username</label>
        <input
          className="input mb-4"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          autoFocus
        />

        <label className="mb-1.5 block text-xs font-medium text-secondary">Password</label>
        <input
          className="input mb-5"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />

        {error && (
          <p className="mb-4 rounded-control border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
            {error}
          </p>
        )}

        <button className="btn-primary w-full" disabled={busy}>
          {busy ? 'Unlocking...' : 'Unlock'}
        </button>

        <p className="mt-5 text-center text-xs text-muted">
          Local accounts only. First password is printed to the container logs.
        </p>
      </form>
    </div>
  )
}
