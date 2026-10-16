import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { Node } from '../api/client'
import { makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import PotSettings from './PotSettings'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const nodes: Node[] = [
  {
    id: 'esp-1',
    name: 'Shelf',
    last_seen: '2026-10-16T12:00:00Z',
    channels: [{ ch: 0, last_raw: 540, last_at: '2026-10-16T12:00:00Z', pot_id: 1 }],
  },
]

function setup(pot = makePot()) {
  const writes: { method: string; body: unknown }[] = []
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const req = input as Request
    if (req.method === 'PATCH') {
      writes.push({ method: 'PATCH', body: await req.json() })
      return json(pot)
    }
    if (req.method === 'DELETE') {
      writes.push({ method: 'DELETE', body: null })
      return new Response(null, { status: 204 })
    }
    return new URL(req.url).pathname.endsWith('/nodes') ? json(nodes) : json(pot)
  })
  renderWithProviders(
    <Routes>
      <Route path="/" element={<p>overview</p>} />
      <Route path="/pots/:id" element={<p>detail page</p>} />
      <Route path="/pots/:id/settings" element={<PotSettings />} />
    </Routes>,
    { route: '/pots/1/settings' },
  )
  return writes
}

test('calibration wizard shows the live raw value of the linked channel', async () => {
  setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Calibrate sensor' }))
  expect(await screen.findByText('540')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Capture dry' })).toBeEnabled()
})

test('unlinked pot asks to link a sensor first', async () => {
  setup(makePot({ node_id: null, channel: null }))
  expect(await screen.findByText('Link a sensor first to calibrate it.')).toBeInTheDocument()
})

test('delete confirms then navigates home', async () => {
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const writes = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Delete pot' }))
  expect(await screen.findByText('overview')).toBeInTheDocument()
  expect(writes).toContainEqual({ method: 'DELETE', body: null })
})

test('delete does nothing when not confirmed', async () => {
  vi.spyOn(window, 'confirm').mockReturnValue(false)
  const writes = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Delete pot' }))
  expect(writes).toEqual([])
})

test('saving sends the edited values and returns to the pot', async () => {
  const writes = setup()
  const name = await screen.findByLabelText('Name')
  await userEvent.clear(name)
  await userEvent.type(name, 'Big Monstera')
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(await screen.findByText('detail page')).toBeInTheDocument()
  expect(writes[0].body).toMatchObject({ name: 'Big Monstera', node_id: 'esp-1', channel: 0, amount_fraction: 0.15 })
})
