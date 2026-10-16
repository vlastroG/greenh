import { useState } from 'react'
import type { Advice } from '../api/client'

const STATUS_LABEL: Record<Advice['status'], string> = {
  ok: 'OK',
  sensor_offline: 'Sensor offline',
  needs_calibration: 'Needs calibration',
}

function Row({ item }: { item: Advice }) {
  const [open, setOpen] = useState(false)
  const rec = item.recommendation
  const label = item.status === 'ok' && rec?.needs_water ? 'Water' : STATUS_LABEL[item.status]
  return (
    <li className="border-b border-gray-100 last:border-0">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full flex-wrap items-center gap-x-3 gap-y-1 px-1 py-2 text-left text-sm hover:bg-gray-50"
      >
        <span className="text-gray-600">{new Date(item.created_at).toLocaleString()}</span>
        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">{label}</span>
        <span className="text-gray-900">{rec ? `${rec.amount_ml} ml` : '—'}</span>
        <span className="ml-auto text-gray-500">{item.provider}</span>
      </button>
      {open && (
        <pre className="mb-2 overflow-x-auto rounded bg-gray-50 p-2 text-xs">
          {item.summary ? JSON.stringify(item.summary, null, 2) : `No summary (status: ${item.status})`}
        </pre>
      )}
    </li>
  )
}

export function AdviceHistory({ items }: { items: Advice[] }) {
  if (items.length === 0) return <p className="text-sm text-gray-500">No advice yet.</p>
  return (
    <ul>
      {items.map((a) => (
        <Row key={a.id} item={a} />
      ))}
    </ul>
  )
}
