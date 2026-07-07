import { createContext, useCallback, useContext, useEffect, useState } from 'react'

// --- Logo ---

export function Logo({ size = 28, withWord = false }) {
  return (
    <span className="inline-flex items-center gap-2.5">
      <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
        <defs>
          <linearGradient id="hushLogoGrad" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#A78BFF" />
            <stop offset="1" stopColor="#7C5CFF" />
          </linearGradient>
        </defs>
        <path
          fill="url(#hushLogoGrad)"
          d="M20 8 h24 a14 14 0 0 1 14 14 v12 a14 14 0 0 1 -14 14 h-14 l-10 10 v-10 a14 14 0 0 1 -14 -14 v-12 a14 14 0 0 1 14 -14 z"
        />
        <circle cx="22" cy="28" r="4.5" fill="#0D0B12" />
        <circle cx="32" cy="28" r="4.5" fill="#0D0B12" />
        <circle cx="42" cy="28" r="4.5" fill="#0D0B12" />
      </svg>
      {withWord && <span className="text-xl font-bold tracking-tight">hush</span>}
    </span>
  )
}

// --- Toasts ---

const ToastCtx = createContext(null)
export const useToast = () => useContext(ToastCtx)

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const push = useCallback((message, kind = 'info') => {
    const id = Math.random().toString(36).slice(2)
    setToasts((t) => [...t, { id, message, kind }])
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000)
  }, [])
  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`card px-4 py-2.5 text-sm shadow-lg ${
              t.kind === 'error'
                ? 'border-danger/50 text-danger'
                : t.kind === 'success'
                ? 'border-success/50 text-success'
                : 'text-primary'
            }`}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}

// --- Modal ---

export function Modal({ title, children, onClose, wide = false }) {
  useEffect(() => {
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-base/70 p-4 backdrop-blur-sm"
      onMouseDown={onClose}
    >
      <div
        className={`card w-full ${wide ? 'max-w-2xl' : 'max-w-md'} p-6`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-lg font-semibold">{title}</h2>
          <button onClick={onClose} className="text-muted hover:text-primary" aria-label="Close">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}

// --- Agent pill (the one teal thing) ---

export function AgentPill({ on }) {
  if (on) {
    return (
      <span className="pill border-agent/40 bg-agent/10 text-agent">
        <Dot className="bg-agent" /> agents
      </span>
    )
  }
  return (
    <span className="pill border-border text-muted">
      <Dot className="bg-muted" /> no agents
    </span>
  )
}

export function Dot({ className }) {
  return <span className={`inline-block h-1.5 w-1.5 rounded-full ${className}`} />
}

// --- Copyable value ---

export function CopyButton({ value, className = '', label = 'Copy' }) {
  const [copied, setCopied] = useState(false)
  const toast = useToast()
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      toast && toast('Clipboard blocked by the browser', 'error')
    }
  }
  return (
    <button onClick={copy} className={`btn-ghost ${className}`} type="button">
      {copied ? 'Copied' : label}
    </button>
  )
}

// --- Empty state ---

export function Empty({ children }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 rounded-card border border-dashed border-border py-16 text-center text-muted">
      {children}
    </div>
  )
}

// --- Role badge ---

export function RoleBadge({ role }) {
  const admin = role === 'admin'
  return (
    <span
      className={`pill ${
        admin ? 'border-accent/40 bg-accent/10 text-accent-hover' : 'border-border text-secondary'
      }`}
    >
      {role}
    </span>
  )
}
