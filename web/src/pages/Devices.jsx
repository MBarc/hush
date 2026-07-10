import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { PageHeader } from './Shell'
import { Empty, useToast } from '../components/ui'

const statusTone = {
  trusted: 'border-agent/40 bg-agent/10 text-agent',
  discovered: 'border-border text-secondary',
  blocked: 'border-danger/40 bg-danger/10 text-danger',
}

export default function Devices() {
  const toast = useToast()
  const [devices, setDevices] = useState(null)

  const load = () => api.devices().then(setDevices).catch(() => setDevices([]))
  useEffect(() => {
    load()
  }, [])

  const block = async (h) => {
    await api.blockDevice(h).catch((e) => toast(e.message, 'error'))
    load()
  }
  const unblock = async (h) => {
    await api.unblockDevice(h).catch((e) => toast(e.message, 'error'))
    load()
  }
  const toggleWrite = async (d) => {
    try {
      await api.setDeviceWrite(d.hostname, !d.allowWrite)
      load()
      toast(d.allowWrite ? 'Writes disabled' : 'Writes allowed within its grants', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }
  const forget = async (h) => {
    if (!confirm(`Forget ${h}? It reappears on the next sweep if still online.`)) return
    await api.deleteDevice(h).catch((e) => toast(e.message, 'error'))
    load()
  }

  return (
    <>
      <PageHeader title="Devices" subtitle="Machines on your network. Click a name to label one." />
      <div className="p-8">
        <div className="mb-5 rounded-card border border-border bg-surface px-4 py-3 text-sm text-secondary">
          A device gains access when you grant it a folder or secret from that item's{' '}
          <span className="text-primary">Device access</span> panel. It then fetches those secrets with
          no token, just an <span className="mono text-primary">X-Hush-Device</span> header, honored only
          from the source IP the device was last seen at.
        </div>

        {devices && devices.length === 0 && (
          <Empty>
            <p className="mono text-sm">No devices seen yet.</p>
            <p className="text-xs">
              Set <span className="mono">HUSH_NETWORK_CIDR</span> to your LAN subnet to start the poller.
            </p>
          </Empty>
        )}

        {devices && devices.length > 0 && (
          <div className="card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted">
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">IP</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Granted paths</th>
                  <th className="px-4 py-3 font-medium">Last seen</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {devices.map((d) => (
                  <tr key={d.hostname}>
                    <td className="px-4 py-3 mono">
                      <NameCell d={d} onSaved={load} />
                    </td>
                    <td className="px-4 py-3 mono text-secondary">{d.ip}</td>
                    <td className="px-4 py-3">
                      <span className={`pill ${statusTone[d.status]}`}>
                        {d.status}
                        {d.allowWrite ? ' +write' : ''}
                      </span>
                    </td>
                    <td className="px-4 py-3 mono text-xs text-secondary">
                      {d.grants && d.grants.length ? d.grants.join(', ') : '-'}
                    </td>
                    <td className="px-4 py-3 text-muted">{fmtTime(d.lastSeen)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-3">
                        {d.status === 'blocked' ? (
                          <button className="text-accent-hover hover:underline" onClick={() => unblock(d.hostname)}>
                            Unblock
                          </button>
                        ) : (
                          <>
                            {d.grants && d.grants.length > 0 && (
                              <button
                                className={`hover:underline ${d.allowWrite ? 'text-agent' : 'text-muted hover:text-primary'}`}
                                onClick={() => toggleWrite(d)}
                                title="Toggle write access within its granted paths"
                              >
                                {d.allowWrite ? 'Writes on' : 'Read-only'}
                              </button>
                            )}
                            <button className="text-danger hover:underline" onClick={() => block(d.hostname)}>
                              Block
                            </button>
                          </>
                        )}
                        <button className="text-muted hover:text-primary hover:underline" onClick={() => forget(d.hostname)}>
                          Forget
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

    </>
  )
}

// NameCell shows a device's friendly label (or its real hostname, or
// "unnamed") and turns into an inline editor on click.
function NameCell({ d, onSaved }) {
  const toast = useToast()
  const [editing, setEditing] = useState(false)
  const [val, setVal] = useState(d.label || '')
  const display = d.label || (d.hostname !== d.ip ? d.hostname : '')

  const save = async () => {
    const next = val.trim()
    if (next === (d.label || '')) {
      setEditing(false)
      return
    }
    try {
      await api.nameDevice(d.hostname, next)
      setEditing(false)
      onSaved()
      toast(next ? `Named ${next}` : 'Name cleared', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  if (editing) {
    return (
      <input
        autoFocus
        className="input !w-40 !px-2 !py-1 mono"
        value={val}
        placeholder="name this device"
        onChange={(e) => setVal(e.target.value)}
        onBlur={save}
        onKeyDown={(e) => {
          if (e.key === 'Enter') save()
          if (e.key === 'Escape') {
            setVal(d.label || '')
            setEditing(false)
          }
        }}
      />
    )
  }

  return (
    <button
      className="group inline-flex items-center gap-1.5 text-left"
      title="Click to rename"
      onClick={() => {
        setVal(d.label || '')
        setEditing(true)
      }}
    >
      {display ? (
        <span className="text-primary">{display}</span>
      ) : (
        <span className="text-muted">unnamed</span>
      )}
      <svg
        className="text-muted opacity-0 transition-opacity group-hover:opacity-100"
        width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      >
        <path d="M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z" />
      </svg>
    </button>
  )
}

