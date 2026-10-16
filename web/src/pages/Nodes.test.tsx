import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes, useLocation } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { Node } from '../api/client'
import { makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import Nodes from './Nodes'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const threeMinAgo = new Date(Date.now() - 3 * 60_000).toISOString()

const nodes: Node[] = [
  {
    id: 'windowsill-1',
    name: 'Windowsill',
    last_seen: threeMinAgo,
    channels: [
      { ch: 0, last_raw: 2100, last_at: threeMinAgo, pot_id: 1 },
      { ch: 1, last_raw: null, last_at: null, pot_id: null },
    ],
  },
]

function LocationProbe() {
  const loc = useLocation()
  return <p>{`at ${loc.pathname}${loc.search}`}</p>
}

function setup(list: Node[] = nodes) {
  const patches: { url: string; body: unknown }[] = []
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const req = input as Request
    const path = new URL(req.url).pathname
    if (req.method === 'PATCH') {
      patches.push({ url: path, body: await req.json() })
      return json({ ...list[0], name: 'Kitchen' })
    }
    return path.endsWith('/nodes') ? json(list) : json([makePot({ id: 1, name: 'Monstera' })])
  })
  renderWithProviders(
    <Routes>
      <Route path="/nodes" element={<Nodes />} />
      <Route path="/pots/new" element={<LocationProbe />} />
    </Routes>,
    { route: '/nodes' },
  )
  return patches
}

test('lists nodes and channels', async () => {
  setup()
  expect(await screen.findByText('Windowsill')).toBeInTheDocument()
  expect(screen.getByText('Last seen 3 min ago')).toBeInTheDocument()
  expect(await screen.findByRole('link', { name: 'Monstera' })).toHaveAttribute('href', '/pots/1')
  expect(screen.getByText(/2100/)).toBeInTheDocument()
  expect(screen.getAllByRole('button', { name: 'Create pot from this channel' })).toHaveLength(1)
})

test('empty state', async () => {
  setup([])
  expect(
    await screen.findByText('No nodes have reported yet. Start the simulator or a sensor node.'),
  ).toBeInTheDocument()
})

test('create pot button navigates with query', async () => {
  setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Create pot from this channel' }))
  expect(await screen.findByText('at /pots/new?node=windowsill-1&ch=1')).toBeInTheDocument()
})

test('rename node', async () => {
  const patches = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Rename node' }))
  const input = screen.getByRole('textbox', { name: 'Node name' })
  await userEvent.clear(input)
  await userEvent.type(input, 'Kitchen{Enter}')
  await vi.waitFor(() =>
    expect(patches).toEqual([{ url: '/api/v1/nodes/windowsill-1', body: { name: 'Kitchen' } }]),
  )
})

test('escape cancels and empty name is not submitted', async () => {
  const patches = setup()
  await userEvent.click(await screen.findByRole('button', { name: 'Rename node' }))
  await userEvent.type(screen.getByRole('textbox', { name: 'Node name' }), 'x{Escape}')
  expect(screen.getByText('Windowsill')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Rename node' }))
  await userEvent.clear(screen.getByRole('textbox', { name: 'Node name' }))
  await userEvent.keyboard('{Enter}')
  expect(patches).toEqual([])
})
