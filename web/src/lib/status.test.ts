import { expect, test } from 'vitest'
import { makeAdvice, makePot } from '../test/fixtures'
import { badgeFor, rangeQuery, staleHint } from './status'

test('badgeFor', () => {
  expect(badgeFor(makePot({ latest_advice: null })).label).toBe('No check yet')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ status: 'sensor_offline', recommendation: null }) })).label).toBe('Sensor offline')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ status: 'needs_calibration', recommendation: null }) })).label).toBe('Needs calibration')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ recommendation: { needs_water: true, amount_ml: 250, confidence: 'medium', reason: 'x' } }) })))
    .toEqual({ label: 'Water ~250 ml', tone: 'water' })
  expect(badgeFor(makePot())).toEqual({ label: 'OK', tone: 'ok' })
})

test('staleHint', () => {
  const now = new Date('2026-10-16T12:00:00Z')
  const reading = (iso: string | null) => ({ moisture_raw: 1, moisture_pct: 1, last_reading_at: iso })
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
