import type { Advice, HistoryPoint, Watering } from '../api/client'

export type ChartRow = {
  t: number
  moisture: number | null
  temp: number | null
  humidity: number | null
  light: number | null
}

export function toRows(points: HistoryPoint[]): ChartRow[] {
  return points.map((p) => ({
    t: Date.parse(p.ts),
    moisture: p.moisture_pct,
    temp: p.temp_c,
    humidity: p.humidity_pct,
    light: p.light_lux,
  }))
}

export function markers(waterings: Watering[], advice: Advice[], from: number, to: number) {
  const inRange = (t: number) => t >= from && t <= to
  return {
    waterings: waterings.map((w) => ({ t: Date.parse(w.ts), ml: w.amount_ml })).filter((w) => inRange(w.t)),
    advice: advice
      .filter((a) => a.status === 'ok')
      .map((a) => ({ t: Date.parse(a.created_at), needsWater: a.recommendation?.needs_water ?? false }))
      .filter((a) => inRange(a.t))
      .sort((a, b) => a.t - b.t),
  }
}
