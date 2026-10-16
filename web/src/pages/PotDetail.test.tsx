import { screen } from '@testing-library/react'
import { Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { makeAdvice, makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import PotDetail from './PotDetail'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

function mockApi(pot = makePot()) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const path = new URL((input as Request).url).pathname
    if (path.endsWith('/readings') || path.endsWith('/waterings')) return json([])
    if (path.endsWith('/advice')) return json({ latest: makeAdvice(), history: [makeAdvice()] })
    return json(pot)
  })
}

const renderPage = () =>
  renderWithProviders(
    <Routes>
      <Route path="/pots/:id" element={<PotDetail />} />
    </Routes>,
    { route: '/pots/1' },
  )

test('detail page uncalibrated message', async () => {
  mockApi(makePot({ dry_raw: null, wet_raw: null }))
  renderPage()
  expect(await screen.findByText('Calibrate the sensor to see moisture %')).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: /settings|calibrate/i })[0]).toHaveAttribute('href', '/pots/1/settings')
})

test('detail page shows latest advice and range tabs', async () => {
  mockApi()
  renderPage()
  expect(await screen.findByText('Soil is moist enough.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '7 d' })).toBeInTheDocument()
  expect(screen.queryByText('Calibrate the sensor to see moisture %')).not.toBeInTheDocument()
})
