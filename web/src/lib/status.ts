import type { Pot } from '../api/client'
import { formatAgo } from './format'

export type Tone = 'ok' | 'water' | 'warn' | 'neutral'
export type Range = '24h' | '7d' | '30d'

const STALE_AFTER_MS = 15 * 60_000

export function badgeFor(pot: Pot): { label: string; tone: Tone } {
  const advice = pot.latest_advice
  if (!advice) return { label: 'No check yet', tone: 'neutral' }
  if (advice.status === 'sensor_offline') return { label: 'Sensor offline', tone: 'warn' }
  if (advice.status === 'needs_calibration') return { label: 'Needs calibration', tone: 'warn' }
  if (advice.recommendation?.needs_water) {
    return { label: `Water ~${advice.recommendation.amount_ml} ml`, tone: 'water' }
  }
  return { label: 'OK', tone: 'ok' }
}

export function staleHint(pot: Pot, now: Date): string | null {
  if (pot.node_id == null) return null
  const last = pot.current.last_reading_at
  if (last == null) return 'No readings yet'
  if (now.getTime() - new Date(last).getTime() > STALE_AFTER_MS) {
    return `Last reading ${formatAgo(last, now)}`
  }
  return null
}

const RANGES: Record<Range, { ms: number; step: number }> = {
  '24h': { ms: 24 * 3_600_000, step: 300 },
  '7d': { ms: 7 * 24 * 3_600_000, step: 3600 },
  '30d': { ms: 30 * 24 * 3_600_000, step: 10800 },
}

export function rangeQuery(range: Range, now: Date): { from: string; to: string; step: number } {
  const { ms, step } = RANGES[range]
  return { from: new Date(now.getTime() - ms).toISOString(), to: now.toISOString(), step }
}
