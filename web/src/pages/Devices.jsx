import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { PageHeader } from './Shell'
import { Empty, Modal, useToast } from '../components/ui'

const statusTone = {
  trusted: 'border-agent/40 bg-agent/10 text-agent',
  discovered: 'border-border text-secondary',
  blocked: 'border-danger/40 bg-danger/10 text-danger',
}

export default function Devices() {
  const toast = useToast()
  const [devices, setDevices] = useState(null)
  const [trusting, setTrusting] = useState(null)

  const load = () => api.devices().then(setDevices).catch(() => setDevices([]))
  useEffect(load, [])

  const block = async (h) => {
    await api.blockDevice(h).catch((e) => toast(e.message, 'error'))
    load()
  }
  const forget = async (h) => {
    if (!confirm(`Forget ${h}? It reappears on the next sweep if still online.`)) return
    await api.deleteDevice(h).catch((e) => toast(e.message, 'error'))
    load()
  }

  return (
    <>
      <PageHeader title="Devices" subtitle="Machines on your network, identified by hostname" />
      <div className="p-8">
        <div className="mb-5 rounded-card border border-border bg-surface px-4 py-3 text-sm text-secondary">
          Hush verifies a claimed hostname against the source IP it was last seen at. Trust a device
          to let it fetch scoped secrets with no token, just an{' '}
          <span className="mono text-primary">X-Hush-Device</span> header.
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
                  <th className="px-4 py-3 font-medium">Hostname</th>
                  <th className="px-4 py-3 font-medium">IP</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Scopes</th>
                  <th className="px-4 py-3 font-medium">Last seen</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {devices.map((d) => (
                  <tr key={d.hostname}>
                    <td className="px-4 py-3 mono text-primary">{d.hostname}</td>
                    <td className="px-4 py-3 mono text-secondary">{d.ip}</td>
                    <td className="px-4 py-3">
                      <span className={`pill ${statusTone[d.status]}`}>
                        {d.status}
                        {d.allowWrite ? ' +write' : ''}
                      </span>
                    </td>
                    <td className="px-4 py-3 mono text-xs text-secondary">
                      {d.scopes && d.scopes.length ? d.scopes.join(', ') : '-'}
                    </td>
                    <td className="px-4 py-3 text-muted">{fmtTime(d.lastSeen)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-3">
                        <button className="text-accent-hover hover:underline" onClick={() => setTrusting(d)}>
                          {d.status === 'trusted' ? 'Edit' : 'Trust'}
                        </button>
                        {d.status !== 'blocked' && (
                          <button className="text-danger hover:underline" onClick={() => block(d.hostname)}>
                            Block
                          </button>
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

      {trusting && (
        <TrustModal
          device={trusting}
          onClose={() => setTrusting(null)}
          onSaved={() => {
            setTrusting(null)
            load()
            toast('Device trusted', 'success')
          }}
        />
      )}
    </>
  )
}

function TrustModal({ device, onClose, onSaved }) {
  const [scopes, setScopes] = useState((device.scopes || []).join(', '))
  const [allowWrite, setAllowWrite] = useState(device.allowWrite || false)
  const [ttlDays, setTtlDays] = useState(0)
  const [err, setErr] = useState('')

  const submit = async (e) => {
    e.preventDefault()
    const list = scopes.split(',').map((s) => s.trim()).filter(Boolean)
    if (!list.length) {
      setErr('At least one scope is required.')
      return
    }
    try {
      await api.trustDevice(device.hostname, { scopes: list, allowWrite, ttlDays: Number(ttlDays) })
      onSaved()
    } catch (e) {
      setErr(e.message)
    }
  }

  return (
    <Modal title={`Trust ${device.hostname}`} onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mb-4 text-sm text-secondary">
          Last seen at <span className="mono text-primary">{device.ip}</span>. Requests are only honored
          from this address.
        </p>
        <label className="mb-1.5 block text-xs font-medium text-secondary">Scopes (comma-separated globs)</label>
        <input className="input mono mb-4" autoFocus placeholder="infra/nas/*" value={scopes} onChange={(e) => setScopes(e.target.value)} />

        <label className="mb-4 flex items-center gap-2.5 text-sm">
          <input type="checkbox" checked={allowWrite} onChange={(e) => setAllowWrite(e.target.checked)} className="accent-agent" />
          <span className="text-secondary">Allow writes within scope (not just reads)</span>
        </label>

        <label className="mb-1.5 block text-xs font-medium text-secondary">Trust expires after (days, 0 = never)</label>
        <input className="input mb-4" type="number" min="0" value={ttlDays} onChange={(e) => setTtlDays(e.target.value)} />

        {err && <p className="mb-3 text-sm text-danger">{err}</p>}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn-primary">Trust device</button>
        </div>
      </form>
    </Modal>
  )
}
