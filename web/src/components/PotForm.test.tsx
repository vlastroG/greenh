import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { render } from '@testing-library/react'
import { Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { Node } from '../api/client'
import PotNew from '../pages/PotNew'
import { makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import { createQueryClient } from '../api/queryClient'
import { emptyPotValues, PotForm } from './PotForm'

afterEach(() => vi.restoreAllMocks())

const nodes: Node[] = [
  {
    id: 'esp-1',
    name: 'Shelf',
    last_seen: '2026-10-16T12:00:00Z',
    channels: [
      { ch: 0, last_raw: 2000, last_at: null, pot_id: 7 },
      { ch: 1, last_raw: 2100, last_at: null, pot_id: null },
    ],
  },
]

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

test('form create omits empty rules', async () => {
  const onSubmit = vi.fn()
  render(<PotForm initial={emptyPotValues} nodes={nodes} mode="create" onSubmit={onSubmit} busy={false} />)
  await userEvent.type(screen.getByLabelText('Name'), 'Fern')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ name: 'Fern', threshold_pct: null, amount_fraction: null }))
  expect(screen.getAllByPlaceholderText('Category default')).toHaveLength(2)
})

test('form converts fraction percent and hours', async () => {
  const onSubmit = vi.fn()
  render(<PotForm initial={emptyPotValues} nodes={nodes} mode="create" onSubmit={onSubmit} busy={false} />)
  await userEvent.type(screen.getByLabelText('Name'), 'Fern')
  await userEvent.type(screen.getByLabelText(/Water amount/), '25')
  await userEvent.clear(screen.getByLabelText('Check every (hours)'))
  await userEvent.type(screen.getByLabelText('Check every (hours)'), '12')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ amount_fraction: 0.25, check_interval_min: 720 }))
})

test('form disables channels linked to other pots', () => {
  render(<PotForm initial={emptyPotValues} nodes={nodes} mode="create" onSubmit={() => {}} busy={false} />)
  expect(screen.getByRole('option', { name: 'Shelf · ch 0' })).toBeDisabled()
  expect(screen.getByRole('option', { name: 'Shelf · ch 1' })).toBeEnabled()
  expect(screen.getByRole('option', { name: 'Not linked' })).toBeInTheDocument()
})

test('form shows the advisor hint for free-text fields', () => {
  render(<PotForm initial={emptyPotValues} nodes={nodes} mode="create" onSubmit={() => {}} busy={false} />)
  expect(screen.getAllByText('Sent to the advisor — avoid personal details.')).toHaveLength(3)
})

function renderNew(route = '/pots/new?node=esp-1&ch=1') {
  return renderWithProviders(
    <Routes>
      <Route path="/pots/new" element={<PotNew />} />
      <Route path="/pots/:id" element={<p>detail page</p>} />
    </Routes>,
    { route },
  )
}

function mockApi(postResponse: () => Response) {
  const bodies: unknown[] = []
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const req = input as Request
    if (req.method === 'POST') {
      bodies.push(await req.json())
      return postResponse()
    }
    return json(nodes)
  })
  return bodies
}

test('create page preselects link and omits empty rules from the request', async () => {
  const bodies = mockApi(() => json(makePot({ id: 9 }), 201))
  renderNew()
  expect(await screen.findByLabelText('Sensor')).toHaveValue('esp-1|1')
  await userEvent.type(screen.getByLabelText('Name'), 'Fern')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  expect(await screen.findByText('detail page')).toBeInTheDocument()
  const body = bodies[0] as Record<string, unknown>
  expect(body).toMatchObject({ name: 'Fern', node_id: 'esp-1', channel: 1, check_interval_min: 360 })
  expect(body).not.toHaveProperty('threshold_pct')
  expect(body).not.toHaveProperty('amount_fraction')
})

test('form shows field errors from 400 without a toast', async () => {
  mockApi(() =>
    json(
      {
        code: 'invalid_request',
        message: 'invalid request',
        details: [{ field: 'pot_ml', message: 'must be between 50 and 50000' }],
      },
      400,
    ),
  )
  renderNew('/pots/new')
  await userEvent.type(await screen.findByLabelText('Name'), 'Fern')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  expect(await screen.findByText('must be between 50 and 50000')).toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('409 shows its message in a single toast', async () => {
  mockApi(() => json({ code: 'conflict', message: 'channel already linked' }, 409))
  renderNew('/pots/new')
  await userEvent.type(await screen.findByLabelText('Name'), 'Fern')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(1))
  expect(screen.getByRole('alert')).toHaveTextContent('channel already linked')
})

async function submitWithGlobalSpy(res: () => Response) {
  mockApi(res)
  const globalToast = vi.fn()
  renderWithProviders(
    <Routes>
      <Route path="/pots/new" element={<PotNew />} />
    </Routes>,
    { route: '/pots/new', client: createQueryClient(globalToast, { queries: { retry: false } }) },
  )
  await userEvent.type(await screen.findByLabelText('Name'), 'Fern')
  await userEvent.click(screen.getByRole('button', { name: 'Create pot' }))
  return globalToast
}

test('500 is left to the global toast and not toasted again by the form', async () => {
  const globalToast = await submitWithGlobalSpy(() => json({ code: 'internal', message: 'boom' }, 500))
  await waitFor(() => expect(globalToast).toHaveBeenCalledTimes(1))
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('400 without field details toasts once from the form', async () => {
  const globalToast = await submitWithGlobalSpy(() =>
    json({ code: 'invalid_request', message: 'bad body', details: [{ field: 'unknown', message: 'x' }] }, 400),
  )
  expect(await screen.findByRole('alert')).toHaveTextContent('bad body')
  expect(globalToast).not.toHaveBeenCalled()
})
