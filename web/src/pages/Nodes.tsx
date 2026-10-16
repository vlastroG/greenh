import { useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import type { Node } from '../api/client'
import { useNodes, usePots, useRenameNode } from '../api/hooks'
import { formatAgo } from '../lib/format'

function NodeName({ node }: { node: Node }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const done = useRef(false)
  const rename = useRenameNode()

  const start = () => {
    done.current = false
    setDraft(node.name)
    setEditing(true)
  }
  // enter and the blur it triggers must not submit twice
  const finish = (save: boolean) => {
    if (done.current) return
    done.current = true
    setEditing(false)
    const name = draft.trim()
    if (save && name && name !== node.name) rename.mutate({ id: node.id, name })
  }

  if (!editing) {
    return (
      <span className="flex items-center gap-2">
        <h2 className="text-lg font-semibold">{node.name || node.id}</h2>
        <button type="button" aria-label="Rename node" onClick={start} className="text-sm text-gray-500 underline">
          Rename
        </button>
      </span>
    )
  }
  return (
    <input
      autoFocus
      aria-label="Node name"
      maxLength={64}
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => finish(true)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') finish(true)
        if (e.key === 'Escape') finish(false)
      }}
      className="rounded border border-gray-300 px-2 py-1"
    />
  )
}

export default function Nodes() {
  const { data: nodes, isPending } = useNodes()
  const { data: pots } = usePots()
  const navigate = useNavigate()

  if (isPending) return <p className="text-gray-500">Loading…</p>
  // on a failed first load the toast explains it
  if (!nodes) return null
  if (nodes.length === 0) {
    return <p className="text-gray-600">No nodes have reported yet. Start the simulator or a sensor node.</p>
  }
  const now = new Date()
  const potName = (id: number) => pots?.find((p) => p.id === id)?.name ?? `Pot #${id}`

  return (
    <div className="space-y-6">
      {nodes.map((node) => (
        <section key={node.id} className="rounded-lg border border-gray-200 bg-white p-4">
          <NodeName node={node} />
          <p className="text-sm text-gray-500">Last seen {formatAgo(node.last_seen, now)}</p>
          <table className="mt-3 w-full text-left text-sm">
            <thead>
              <tr className="text-gray-500">
                <th className="py-1">Channel</th>
                <th>Latest raw</th>
                <th>Pot</th>
              </tr>
            </thead>
            <tbody>
              {node.channels.map((c) => (
                <tr key={c.ch} className="border-t border-gray-100">
                  <td className="py-2">{c.ch}</td>
                  <td>
                    {c.last_raw === null || c.last_at === null
                      ? '—'
                      : `${c.last_raw} (${formatAgo(c.last_at, now)})`}
                  </td>
                  <td>
                    {c.pot_id !== null ? (
                      <Link to={`/pots/${c.pot_id}`} className="text-green-700 underline">
                        {potName(c.pot_id)}
                      </Link>
                    ) : (
                      <button
                        type="button"
                        onClick={() => navigate(`/pots/new?node=${encodeURIComponent(node.id)}&ch=${c.ch}`)}
                        className="rounded bg-green-700 px-2 py-1 text-white"
                      >
                        Create pot from this channel
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}
    </div>
  )
}
