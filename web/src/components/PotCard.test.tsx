import { screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import type { Pot } from '../api/client'
import { makeAdvice, makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import { PotCard } from './PotCard'

const now = new Date('2026-10-16T14:00:00Z')

function card(overrides: Partial<Pot> = {}) {
  renderWithProviders(<PotCard pot={makePot(overrides)} now={now} />)
}

describe('PotCard', () => {
  test('card shows OK pot', () => {
    card({ current: { moisture_raw: 2000, moisture_pct: 42, last_reading_at: '2026-10-16T13:58:00Z' } })
    expect(screen.getByText('Monstera')).toBeInTheDocument()
    expect(screen.getByText('42%')).toBeInTheDocument()
    expect(screen.getByText('OK')).toBeInTheDocument()
    expect(screen.getByText('Checked 2 h ago')).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/pots/1')
  })

  test('card never checked', () => {
    card({ latest_advice: null })
    expect(screen.getByText('No check yet')).toBeInTheDocument()
    expect(screen.queryByText(/Checked/)).not.toBeInTheDocument()
  })

  test('card uncalibrated shows dash', () => {
    card({ current: { moisture_raw: 2000, moisture_pct: null, last_reading_at: '2026-10-16T13:58:00Z' } })
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  test('card shows disagreement marker', () => {
    card({ latest_advice: makeAdvice({ disagree: true }) })
    expect(screen.getByText('LLM and rules disagree')).toBeInTheDocument()
  })

  test('card shows stale hint', () => {
    card({ current: { moisture_raw: 2000, moisture_pct: 55, last_reading_at: '2026-10-16T12:00:00Z' } })
    expect(screen.getByText('Last reading 2 h ago')).toBeInTheDocument()
  })
})
