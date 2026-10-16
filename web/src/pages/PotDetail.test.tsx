import { screen } from '@testing-library/react'
import { Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { Advice } from '../api/client'
import { makeAdvice, makePot } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import PotDetail from './PotDetail'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

function mockApi(pot = makePot(), history: Advice[] = [makeAdvice()]) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const path = new URL((input as Request).url).pathname
    if (path.endsWith('/readings') || path.endsWith('/waterings')) return json([])
    if (path.endsWith('/advice')) return json({ latest: history[0] ?? null, history })
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

const water = (ml: number, id: number) =>
  makeAdvice({ id, recommendation: { needs_water: true, amount_ml: ml, confidence: 'medium', reason: `Water ${ml}` } })

test('watering amount is prefilled from the latest advice', async () => {
  const pot = makePot()
  mockApi({ ...pot, latest_advice: water(300, 2) }, [water(300, 2), water(200, 1)])
  renderPage()
  expect(await screen.findByText('Water 300')).toBeInTheDocument()
  expect(screen.getByLabelText('Amount (ml)')).toHaveValue(300)
})

test('watering amount stays empty when the latest advice says no water', async () => {
  mockApi(makePot(), [makeAdvice({ id: 2 }), water(300, 1)])
  renderPage()
  // the older watering advice is loaded and listed in the history
  expect(await screen.findByText('300 ml')).toBeInTheDocument()
  expect(screen.getByLabelText('Amount (ml)')).toHaveValue(null)
})
