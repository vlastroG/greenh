import { formatPct } from '../lib/format'

export function MoistureGauge({ pct }: { pct: number | null }) {
  const width = pct == null ? 0 : Math.max(0, Math.min(100, pct))
  return (
    <div>
      <div className="text-3xl font-semibold text-gray-900">{formatPct(pct)}</div>
      <div className="mt-1 h-2 overflow-hidden rounded-full bg-gray-200">
        <div className="h-full rounded-full bg-sky-500" style={{ width: `${width}%` }} />
      </div>
    </div>
  )
}
