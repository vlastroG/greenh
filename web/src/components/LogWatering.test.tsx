import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { renderWithProviders } from '../test/utils'
import { LogWatering } from './LogWatering'

afterEach(() => vi.restoreAllMocks())

test('log watering prefilled and posts amount', async () => {
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(
    async () =>
      new Response(
        JSON.stringify({
          id: 1,
          pot_id: 1,
          ts: '2026-10-16T12:00:00Z',
          created_at: '2026-10-16T12:00:00Z',
          amount_ml: 250,
          source: 'manual',
        }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
  )
  renderWithProviders(<LogWatering potId={1} suggestedMl={250} />)
  expect(screen.getByLabelText('Amount (ml)')).toHaveValue(250)
  await userEvent.setup().click(screen.getByRole('button', { name: 'Log watering' }))
  expect(await screen.findByText('Logged 250 ml')).toBeInTheDocument()
  const req = fetchMock.mock.calls[0][0] as Request
  expect(req.method).toBe('POST')
  expect(new URL(req.url).pathname).toBe('/api/v1/pots/1/waterings')
  expect(await req.json()).toMatchObject({ amount_ml: 250 })
})

test('log watering rejects 0', async () => {
  renderWithProviders(<LogWatering potId={1} suggestedMl={null} />)
  const input = screen.getByLabelText('Amount (ml)')
  expect(input).toHaveValue(null)
  await userEvent.setup().type(input, '0')
  expect(screen.getByRole('button', { name: 'Log watering' })).toBeDisabled()
})
