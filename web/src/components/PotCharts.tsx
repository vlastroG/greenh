import { useMemo, type ReactNode } from 'react'
import { Link } from 'react-router'
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { Pot } from '../api/client'
import { useAdvice, useHistory, useWaterings } from '../api/hooks'
import { markers, toRows } from '../lib/chart'
import { rangeQuery, type Range } from '../lib/status'

const RANGE_LABELS: [Range, string][] = [
  ['24h', '24 h'],
  ['7d', '7 d'],
  ['30d', '30 d'],
]

const timeTick = (range: Range) => (t: number) =>
  range === '24h'
    ? new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : new Date(t).toLocaleDateString([], { month: 'short', day: 'numeric' })

function ChartCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-gray-200 bg-white p-3">
      <h3 className="mb-2 text-sm font-medium text-gray-700">{title}</h3>
      <div className="h-48 w-full">{children}</div>
    </section>
  )
}

export function PotCharts({ pot, range, onRangeChange }: { pot: Pot; range: Range; onRangeChange(r: Range): void }) {
  const history = useHistory(pot.id, range)
  const waterings = useWaterings(pot.id)
  const advice = useAdvice(pot.id)

  const rows = useMemo(() => toRows(history.data ?? []), [history.data])
  const marks = useMemo(() => {
    const q = rangeQuery(range, new Date())
    return markers(waterings.data ?? [], advice.data?.history ?? [], Date.parse(q.from), Date.parse(q.to))
    // recompute window when fresh history lands
  }, [range, waterings.data, advice.data, history.dataUpdatedAt])

  const calibrated = pot.dry_raw != null && pot.wet_raw != null
  const xAxis = (
    <XAxis
      dataKey="t"
      type="number"
      scale="time"
      domain={['dataMin', 'dataMax']}
      tickFormatter={timeTick(range)}
      fontSize={11}
    />
  )
  const tooltipLabel = (t: unknown) => new Date(Number(t)).toLocaleString()

  return (
    <div className="space-y-3">
      <div className="flex gap-1" role="group" aria-label="Time range">
        {RANGE_LABELS.map(([r, label]) => (
          <button
            key={r}
            type="button"
            aria-pressed={r === range}
            onClick={() => onRangeChange(r)}
            className={`rounded-md px-3 py-1 text-sm ${
              r === range ? 'bg-green-700 text-white' : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {calibrated ? (
        <ChartCard title="Moisture %">
          <ResponsiveContainer>
            <LineChart data={rows}>
              <CartesianGrid strokeDasharray="3 3" />
              {xAxis}
              <YAxis domain={[0, 100]} unit="%" fontSize={11} width={40} />
              <Tooltip labelFormatter={tooltipLabel} />
              <Line type="monotone" dataKey="moisture" name="Moisture %" stroke="#15803d" dot={false} connectNulls />
              {marks.waterings.map((w) => (
                <ReferenceLine key={`w${w.t}`} x={w.t} stroke="#2563eb" strokeDasharray="4 2" label={{ value: `${w.ml} ml`, fontSize: 10, fill: '#2563eb' }} />
              ))}
              {marks.advice.map((a) => (
                <ReferenceDot
                  key={`a${a.t}`}
                  x={a.t}
                  y={4}
                  r={4}
                  stroke="#b45309"
                  fill={a.needsWater ? '#b45309' : '#ffffff'}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </ChartCard>
      ) : (
        <p className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          <span>Calibrate the sensor to see moisture %</span>{' '}
          <Link to={`/pots/${pot.id}/settings`} className="underline">
            Open settings
          </Link>
        </p>
      )}

      <ChartCard title="Temperature and humidity">
        <ResponsiveContainer>
          <LineChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" />
            {xAxis}
            <YAxis yAxisId="temp" unit="°" fontSize={11} width={40} />
            <YAxis yAxisId="hum" orientation="right" unit="%" fontSize={11} width={40} />
            <Tooltip labelFormatter={tooltipLabel} />
            <Line yAxisId="temp" type="monotone" dataKey="temp" name="Temp °C" stroke="#dc2626" dot={false} connectNulls />
            <Line yAxisId="hum" type="monotone" dataKey="humidity" name="Humidity %" stroke="#0891b2" dot={false} connectNulls />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="Light (lux)">
        <ResponsiveContainer>
          <LineChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" />
            {xAxis}
            <YAxis fontSize={11} width={48} />
            <Tooltip labelFormatter={tooltipLabel} />
            <Line type="monotone" dataKey="light" name="Light lux" stroke="#ca8a04" dot={false} connectNulls />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>
    </div>
  )
}
