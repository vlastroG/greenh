import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { makeAdvice } from '../test/fixtures'
import { AdviceHistory } from './AdviceHistory'

test('advice history expands summary', async () => {
  render(
    <AdviceHistory
      items={[
        makeAdvice({ id: 1, summary: { moisture_pct: 18 } }),
        makeAdvice({ id: 2, status: 'sensor_offline', recommendation: null }),
      ]}
    />,
  )
  const user = userEvent.setup()
  expect(screen.queryByText(/"moisture_pct": 18/)).not.toBeInTheDocument()
  const rows = screen.getAllByRole('button')
  await user.click(rows[0])
  expect(screen.getByText(/"moisture_pct": 18/)).toBeInTheDocument()
  await user.click(rows[1])
  expect(screen.getByText('No summary (status: sensor_offline)')).toBeInTheDocument()
})
