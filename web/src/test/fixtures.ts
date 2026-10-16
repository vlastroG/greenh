import type { Advice, Pot } from '../api/client'

export function makeAdvice(overrides: Partial<Advice> = {}): Advice {
  return {
    id: 1,
    pot_id: 1,
    created_at: '2026-10-16T12:00:00Z',
    trigger: 'scheduled',
    status: 'ok',
    recommendation: { needs_water: false, amount_ml: 0, confidence: 'high', reason: 'Soil is moist enough.' },
    rules: { needs_water: false, amount_ml: 0, confidence: 'high', reason: 'Above threshold.' },
    provider: 'rules',
    model: '',
    prompt_version: 'v1',
    fallback: false,
    fallback_reason: '',
    disagree: false,
    summary: null,
    ...overrides,
  }
}

export function makePot(overrides: Partial<Pot> = {}): Pot {
  return {
    id: 1,
    name: 'Monstera',
    species: 'Monstera deliciosa',
    category: 'foliage',
    pot_ml: 2000,
    soil_type: 'potting mix',
    placement: 'living room',
    node_id: 'esp-1',
    channel: 0,
    dry_raw: 3000,
    wet_raw: 1200,
    threshold_pct: 35,
    amount_fraction: 0.15,
    check_interval_min: 360,
    created_at: '2026-10-01T08:00:00Z',
    current: { status: 'ok', moisture_raw: 2000, moisture_pct: 55, last_reading_at: '2026-10-16T11:58:00Z' },
    latest_advice: makeAdvice(),
    ...overrides,
  }
}
