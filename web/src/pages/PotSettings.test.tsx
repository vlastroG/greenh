import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { Node } from '../api/client'
import { makeAdvice, makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import PotSettings from './PotSettings'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let liveRaw = 540

const nodes = (): Node[] => [
  {
    id: 'esp-1',
    name: 'Shelf',
    last_seen: '2026-10-16T12:00:00Z',
    channels: [
      { ch: 0, last_raw: liveRaw, last_at: '2026-10-16T12:00:00Z', pot_id: 1 },
      { ch: 1, last_raw: 500, last_at: '2026-10-16T12:00:00Z', pot_id: null },
    ],
  },
]

function setup(pot = makePot()) {
  liveRaw = 540
  const writes: { method: string; body: unknown }[] = []
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const req = input as Request
    if (req.method === 'POST' && new URL(req.url).pathname.endsWith('/advice')) {
      writes.push({ method: 'POST /advice', body: null })
      return json(makeAdvice())
    }
    if (req.method === 'PATCH') {
      writes.push({ method: 'PATCH', body: await req.json() })
      return json(pot)
    }
    if (req.method === 'DELETE') {
      writes.push({ method: 'DELETE', body: null })
      return new Response(null, { status: 204 })
    }
    return new URL(req.url).pathname.endsWith('/nodes') ? json(nodes()) : json(pot)
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

test('saving calibration starts a check', async () => {
  const writes = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Calibrate sensor' }))
  await userEvent.click(await screen.findByRole('button', { name: 'Capture dry' }))
  liveRaw = 270
  // refocusing refetches the polled nodes query
  act(() => {
    window.dispatchEvent(new Event('visibilitychange'))
  })
  expect(await screen.findByText('270')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Capture wet' }))
  const wizard = screen.getByText('Dry 540 · Wet 270').parentElement as HTMLElement
  await userEvent.click(within(wizard).getByRole('button', { name: 'Save' }))
  expect(await screen.findByText('Calibration saved — checking…')).toBeInTheDocument()
  await waitFor(() => expect(writes.filter((w) => w.method === 'POST /advice')).toHaveLength(1))
  expect(writes[0]).toEqual({ method: 'PATCH', body: { dry_raw: 540, wet_raw: 270 } })
})

test('changing the sensor link starts a check', async () => {
  const writes = setup()
  await userEvent.selectOptions(await screen.findByLabelText('Sensor'), 'esp-1|1')
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(await screen.findByText('detail page')).toBeInTheDocument()
  await waitFor(() => expect(writes.filter((w) => w.method === 'POST /advice')).toHaveLength(1))
  expect(writes[0].body).toMatchObject({ node_id: 'esp-1', channel: 1 })
})

test('saving without a link change does not start a check', async () => {
  const writes = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Save' }))
  expect(await screen.findByText('detail page')).toBeInTheDocument()
  expect(writes.map((w) => w.method)).toEqual(['PATCH'])
})

test('rule fields are required when editing', async () => {
  const writes = setup()
  await userEvent.clear(await screen.findByLabelText('Dry threshold (%)'))
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(screen.getByLabelText('Dry threshold (%)')).toBeInvalid()
  await userEvent.type(screen.getByLabelText('Dry threshold (%)'), '40')
  await userEvent.clear(screen.getByLabelText('Water amount (% of pot volume)'))
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(screen.getByLabelText('Water amount (% of pot volume)')).toBeInvalid()
  expect(writes).toEqual([])
})
