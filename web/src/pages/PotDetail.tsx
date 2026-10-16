import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useAdvice, usePot } from '../api/hooks'
import { AdviceHistory } from '../components/AdviceHistory'
import { CheckNowButton } from '../components/CheckNowButton'
import { LogWatering } from '../components/LogWatering'
import { MoistureGauge } from '../components/MoistureGauge'
import { PotCharts } from '../components/PotCharts'
import { StatusBadge } from '../components/StatusBadge'
import { formatAgo } from '../lib/format'
import type { Range } from '../lib/status'
import type { Advice } from '../api/client'

function LatestAdvice({ advice }: { advice: Advice | null }) {
  if (!advice) return <p className="text-sm text-gray-500">No check yet.</p>
  return (
    <div className="space-y-1 text-sm">
      <p className="text-gray-900">{advice.recommendation?.reason ?? `Status: ${advice.status}`}</p>
      <p className="text-gray-500">
        {advice.provider} · {advice.model || 'n/a'}
      </p>
      {advice.fallback && (
        <p className="text-amber-700">
          <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium">Fallback to rules</span>{' '}
          {advice.fallback_reason}
        </p>
      )}
      {advice.disagree && <p className="font-medium text-amber-700">LLM and rules disagree</p>}
    </div>
  )
}

export default function PotDetail() {
  const { id } = useParams()
  const potId = Number(id)
  const [range, setRange] = useState<Range>('24h')
  const pot = usePot(potId)
  const advice = useAdvice(potId)

  if (pot.isPending) return <p className="text-gray-500">Loading…</p>
  // on a failed first load the toast explains it
  if (!pot.data) return null
  const p = pot.data

  const history = advice.data?.history ?? []
  const suggested = history.find((a) => a.status === 'ok' && a.recommendation?.needs_water)
  const latest = advice.data?.latest ?? p.latest_advice ?? null
  const now = new Date()

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold text-gray-900">{p.name}</h1>
          <p className="truncate text-sm text-gray-500">{p.species}</p>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge pot={p} />
          <Link to={`/pots/${p.id}/settings`} className="text-sm text-green-700 underline">
            Settings
          </Link>
        </div>
      </header>

      <section className="space-y-4 rounded-lg border border-gray-200 bg-white p-4">
        <MoistureGauge pct={p.current.moisture_pct ?? null} />
        {p.current.last_reading_at && (
          <p className="text-xs text-gray-500">Last reading {formatAgo(p.current.last_reading_at, now)}</p>
        )}
        <LatestAdvice advice={latest} />
        <CheckNowButton potId={p.id} />
      </section>

      <PotCharts pot={p} range={range} onRangeChange={setRange} />

      <section className="space-y-2 rounded-lg border border-gray-200 bg-white p-4">
        <h2 className="text-sm font-medium text-gray-700">Log a watering</h2>
        <LogWatering potId={p.id} suggestedMl={suggested?.recommendation?.amount_ml ?? null} />
      </section>

      <section className="rounded-lg border border-gray-200 bg-white p-4">
        <h2 className="mb-2 text-sm font-medium text-gray-700">Advice history</h2>
        <AdviceHistory items={history} />
      </section>
    </div>
  )
}
