import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { makeAdvice } from '../test/fixtures'
import { renderWithProviders } from '../test/utils'
import { CheckNowButton } from './CheckNowButton'

afterEach(() => vi.restoreAllMocks())

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

test('check now disables while pending', async () => {
  let resolve!: (r: Response) => void
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockImplementation(() => new Promise<Response>((r) => (resolve = r)))
  renderWithProviders(<CheckNowButton potId={1} />)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Check now' }))
  const busy = await screen.findByRole('button', { name: 'Checking…' })
  expect(busy).toBeDisabled()
  await user.click(busy)
  expect(fetchMock).toHaveBeenCalledTimes(1)

  resolve(json(makeAdvice(), 201))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Check now' })).toBeEnabled())
})
