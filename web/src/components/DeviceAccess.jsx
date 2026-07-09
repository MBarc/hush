import { useEffect, useState } from 'react'
import { api } from '../api'
import { useToast } from './ui'

// DeviceAccess lists the devices allowed to read a folder or secret and lets
// an admin grant or revoke them. Grants made higher up (on an ancestor
// folder) show as inherited and read-only here.
export function DeviceAccess({ path }) {
  const toast = useToast()
  const [grants, setGrants] = useState(null)
  const [devices, setDevices] = useState([])
  const [adding, setAdding] = useState(false)
  const [pick, setPick] = useState('')

  const load = () => api.pathGrants(path).then(setGrants).catch(() => setGrants([]))
  useEffect(() => {
    load()
    api.devices().then(setDevices).catch(() => {})
  }, [path])

  const name = (d) => d.label || (d.hostname !== d.ip ? d.hostname : d.ip)

  const grant = async () => {
    if (!pick) return
    try {
      await api.grantDevice(pick, path)
      setPick('')
      setAdding(false)
      load()
      toast('Device granted', 'success')
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  const revoke = async (hostname) => {
    try {
      await api.revokeDeviceGrant(hostname, path)
      load()
    } catch (e) {
      toast(e.message, 'error')
    }
  }

  const grantedHosts = new Set((grants || []).map((g) => g.hostname))
  const available = devices.filter((d) => d.status !== 'blocked' && !grantedHosts.has(d.hostname))

  return (
    <section className="card p-4">
      <div className="mb-2 flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-primary">Device access</p>
          <p className="text-xs text-muted">Devices allowed to read this. A grant here cascades to everything beneath.</p>
        </div>
        <button className="btn-ghost !px-2 !py-1" onClick={() => setAdding((a) => !a)}>
          Add device
        </button>
      </div>

      {adding && (
        <div className="mb-3 flex gap-2">
          <select className="input" value={pick} onChange={(e) => setPick(e.target.value)} autoFocus>
            <option value="">Select a device...</option>
            {available.map((d) => (
              <option key={d.hostname} value={d.hostname}>
                {name(d)} ({d.ip})
              </option>
            ))}
          </select>
          <button className="btn-primary shrink-0" onClick={grant} disabled={!pick}>
            Grant
          </button>
        </div>
      )}

      {grants && grants.length === 0 && <p className="text-xs text-muted">No devices have access yet.</p>}

      <div className="space-y-1.5">
        {(grants || []).map((g) => (
          <div
            key={g.hostname}
            className="flex items-center justify-between rounded-control border border-border bg-raised px-3 py-2"
          >
            <div className="min-w-0">
              <span className="mono text-sm text-primary">{g.label || g.hostname}</span>
              {g.via && <span className="ml-2 text-xs text-muted">inherited from {g.via}/</span>}
            </div>
            {g.via ? (
              <span className="shrink-0 text-xs text-muted">inherited</span>
            ) : (
              <button className="shrink-0 text-sm text-danger hover:underline" onClick={() => revoke(g.hostname)}>
                Remove
              </button>
            )}
          </div>
        ))}
      </div>
    </section>
  )
}
