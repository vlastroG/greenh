import { act, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import Overview from './Overview'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

test('overview lists pots from API', async () => {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async () =>
    json([makePot(), makePot({ id: 2, name: 'Ficus' })]),
  )
  renderWithProviders(<Overview />)
  expect(await screen.findByText('Monstera')).toBeInTheDocument()
  expect(screen.getByText('Ficus')).toBeInTheDocument()
  expect(screen.getAllByRole('link')).toHaveLength(2)
})

test('overview shows empty state', async () => {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async () => json([]))
  renderWithProviders(<Overview />)
  expect(await screen.findByText(/No pots yet/)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Nodes' })).toHaveAttribute('href', '/nodes')
})

test('overview keeps cards and toasts on failure', async () => {
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockImplementationOnce(async () => json([makePot(), makePot({ id: 2, name: 'Ficus' })]))
    .mockImplementation(async () => {
      throw new TypeError('Failed to fetch')
    })
  renderWithProviders(<Overview />)
  expect(await screen.findByText('Ficus')).toBeInTheDocument()

  // refocusing the window refetches the stale query
  act(() => {
    window.dispatchEvent(new Event('visibilitychange'))
  })

  expect(await screen.findByText("Can't reach the server")).toBeInTheDocument()
  expect(fetchMock).toHaveBeenCalledTimes(2)
  expect(screen.getByText('Monstera')).toBeInTheDocument()
  expect(screen.getByText('Ficus')).toBeInTheDocument()
})
