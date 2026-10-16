import { expect, test } from 'vitest'
import { makeAdvice, makePot } from '../test/fixtures'
import { badgeFor, rangeQuery, staleHint } from './status'

test('badgeFor falls back to latest advice when the live status is ok', () => {
  expect(badgeFor(makePot({ latest_advice: null }))).toEqual({ label: 'No check yet', tone: 'neutral' })
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ recommendation: { needs_water: true, amount_ml: 250, confidence: 'medium', reason: 'x' } }) })))
    .toEqual({ label: 'Water ~250 ml', tone: 'water' })
  expect(badgeFor(makePot())).toEqual({ label: 'OK', tone: 'ok' })
})

test('badgeFor prefers the live status over stale advice', () => {
  const current = (status: 'ok' | 'sensor_offline' | 'needs_calibration') => ({ ...makePot().current, status })
  // dropout while the last advice still says ok
  expect(badgeFor(makePot({ current: current('sensor_offline') }))).toEqual({ label: 'Sensor offline', tone: 'warn' })
  expect(badgeFor(makePot({ current: current('needs_calibration'), latest_advice: null })))
    .toEqual({ label: 'Needs calibration', tone: 'warn' })
  // calibrated or recovered, but the last advice was a precheck failure
  for (const status of ['sensor_offline', 'needs_calibration'] as const) {
    expect(badgeFor(makePot({ latest_advice: makeAdvice({ status, recommendation: null, rules: null }) })))
      .toEqual({ label: 'Checking soon', tone: 'neutral' })
  }
})

test('badgeFor shows unlinked pots as not linked', () => {
  const pot = makePot({ node_id: null, channel: null, current: { ...makePot().current, status: 'sensor_offline' } })
  expect(badgeFor(pot)).toEqual({ label: 'Not linked', tone: 'warn' })
})

test('staleHint', () => {
  const now = new Date('2026-10-16T12:00:00Z')
  const reading = (iso: string | null) => ({ status: 'ok' as const, moisture_raw: 1, moisture_pct: 1, last_reading_at: iso })
  expect(staleHint(makePot({ current: reading('2026-10-16T11:55:00Z') }), now)).toBeNull()
  expect(staleHint(makePot({ current: reading('2026-10-16T10:00:00Z') }), now)).toBe('Last reading 2 h ago')
  expect(staleHint(makePot({ current: reading(null) }), now)).toBe('No readings yet')
  expect(staleHint(makePot({ node_id: null, channel: null, current: reading(null) }), now)).toBeNull()
})

test('rangeQuery', () => {
  const now = new Date('2026-10-16T12:00:00Z')
  expect(rangeQuery('7d', now)).toEqual({ from: '2026-10-09T12:00:00.000Z', to: '2026-10-16T12:00:00.000Z', step: 3600 })
  expect(rangeQuery('24h', now)).toMatchObject({ from: '2026-10-15T12:00:00.000Z', step: 300 })
  expect(rangeQuery('30d', now)).toMatchObject({ from: '2026-09-16T12:00:00.000Z', step: 10800 })
})
