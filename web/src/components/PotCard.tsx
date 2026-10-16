import { Link } from 'react-router'
import type { Pot } from '../api/client'
import { formatAgo } from '../lib/format'
import { staleHint } from '../lib/status'
import { MoistureGauge } from './MoistureGauge'
import { StatusBadge } from './StatusBadge'

export function PotCard({ pot, now }: { pot: Pot; now: Date }) {
  const advice = pot.latest_advice
  const stale = staleHint(pot, now)
  return (
    <Link
      to={`/pots/${pot.id}`}
      className="block rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition hover:shadow-md"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="truncate font-semibold text-gray-900">{pot.name}</h2>
          <p className="truncate text-sm text-gray-500">{pot.species}</p>
        </div>
        <StatusBadge pot={pot} />
      </div>
      <div className="mt-4">
        <MoistureGauge pct={pot.current.moisture_pct ?? null} />
      </div>
      <div className="mt-3 space-y-1 text-xs text-gray-500">
        {advice && <p>Checked {formatAgo(advice.created_at, now)}</p>}
        {advice?.disagree && <p className="font-medium text-amber-700">LLM and rules disagree</p>}
        {stale && <p className="text-amber-700">{stale}</p>}
      </div>
    </Link>
  )
}
