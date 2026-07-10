import { useEffect, useState } from 'react'
import { api, fmtTime } from '../api'
import { PageHeader } from './Shell'
import { Empty } from '../components/ui'

// Actor-type coloring: devices and agent tokens (machines) get the teal
// treatment so a glance at the log answers "what did a machine do?".
const actorTone = {
  device: 'text-agent',
  token: 'text-agent',
  user: 'text-primary',
  system: 'text-muted',
}

const actionTone = (a) => {
  if (a.includes('denied') || a.includes('failed')) return 'text-danger'
  if (a.includes('rotate') || a.includes('write') || a.includes('create')) return 'text-warning'
  if (a.includes('delete') || a.includes('block')) return 'text-danger'
  return 'text-secondary'
}

export default function Audit() {
  const [entries, setEntries] = useState(null)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    api.audit(200).then(setEntries).catch(() => setEntries([]))
  }, [])

  const shown = (entries || []).filter((e) => {
    if (!filter) return true
    const hay = `${e.actor} ${e.action} ${e.path} ${e.ip}`.toLowerCase()
    return hay.includes(filter.toLowerCase())
  })

  return (
    <>
      <PageHeader title="Audit" subtitle="Every read, write, rotation, login, and device access">
        <input
          className="input w-64"
          placeholder="Filter by actor, path, action..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        {/* Real anchors: the server sends the whole log as a file download. */}
        <a className="btn-ghost" href="/api/v1/audit/export?format=csv" download>
          Export CSV
        </a>
        <a className="btn-ghost" href="/api/v1/audit/export?format=json" download>
          Export JSON
        </a>
      </PageHeader>

      <div className="p-8">
        {entries && entries.length === 0 && (
          <Empty>
            <p className="mono text-sm">Nothing logged yet.</p>
          </Empty>
        )}
        {entries && entries.length > 0 && (
          <div className="card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted">
                  <th className="px-4 py-3 font-medium">Time</th>
                  <th className="px-4 py-3 font-medium">Actor</th>
                  <th className="px-4 py-3 font-medium">Action</th>
                  <th className="px-4 py-3 font-medium">Path</th>
                  <th className="px-4 py-3 font-medium">Source</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {shown.map((e) => (
                  <tr key={e.id} className="hover:bg-raised/40">
                    <td className="whitespace-nowrap px-4 py-2.5 text-muted">{fmtTime(e.ts)}</td>
                    <td className={`px-4 py-2.5 mono ${actorTone[e.actorType] || 'text-primary'}`}>
                      {e.actor}
                    </td>
                    <td className={`px-4 py-2.5 mono text-xs ${actionTone(e.action)}`}>{e.action}</td>
                    <td className="px-4 py-2.5 mono text-xs text-secondary">{e.path || '-'}</td>
                    <td className="px-4 py-2.5 mono text-xs text-muted">{e.ip || '-'}</td>
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
