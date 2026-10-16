import { expect, test } from 'vitest'
import { markers, toRows } from './chart'
import { makeAdvice } from '../test/fixtures'

test('toRows maps nulls through', () => {
  const rows = toRows([
    { ts: '2026-10-16T12:00:00Z', moisture_raw: 2000, moisture_pct: 55, temp_c: null, humidity_pct: 40, light_lux: null },
  ])
  expect(rows).toEqual([
    { t: Date.parse('2026-10-16T12:00:00Z'), moisture: 55, temp: null, humidity: 40, light: null },
  ])
})

test('markers filters to range and ok advice', () => {
  const from = Date.parse('2026-10-16T00:00:00Z')
  const to = Date.parse('2026-10-17T00:00:00Z')
  const w = (ts: string, amount_ml: number) => ({ id: 1, pot_id: 1, ts, created_at: ts, amount_ml, source: 'manual' })
  const m = markers(
    [w('2026-10-15T23:00:00Z', 100), w('2026-10-16T10:00:00Z', 200)],
    [
      makeAdvice({ created_at: '2026-10-16T09:00:00Z' }),
      makeAdvice({
        created_at: '2026-10-16T11:00:00Z',
        recommendation: { needs_water: true, amount_ml: 250, confidence: 'high', reason: 'dry' },
      }),
      makeAdvice({ created_at: '2026-10-16T12:00:00Z', status: 'sensor_offline', recommendation: null }),
      makeAdvice({ created_at: '2026-10-18T12:00:00Z' }),
    ],
    from,
    to,
  )
  expect(m.waterings).toEqual([{ t: Date.parse('2026-10-16T10:00:00Z'), ml: 200 }])
  expect(m.advice).toEqual([
    { t: Date.parse('2026-10-16T09:00:00Z'), needsWater: false },
    { t: Date.parse('2026-10-16T11:00:00Z'), needsWater: true },
  ])
})
