import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { CalibrationWizard } from './CalibrationWizard'

test('wizard captures dry then wet and saves', async () => {
  const onSave = vi.fn()
  const ui = (raw: number | null) => <CalibrationWizard latestRaw={raw} onSave={onSave} onCancel={() => {}} />
  const { rerender } = render(ui(540))
  await userEvent.click(screen.getByRole('button', { name: 'Capture dry' }))
  rerender(ui(270))
  await userEvent.click(screen.getByRole('button', { name: 'Capture wet' }))
  expect(screen.getByText('Dry 540 · Wet 270')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(onSave).toHaveBeenCalledWith(540, 270)
})

test('wizard waits for a reading', () => {
  render(<CalibrationWizard latestRaw={null} onSave={() => {}} onCancel={() => {}} />)
  expect(screen.getByText('Waiting for a reading from the sensor')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Capture dry' })).toBeDisabled()
})

test('wizard blocks equal values and can start over', async () => {
  render(<CalibrationWizard latestRaw={500} onSave={() => {}} onCancel={() => {}} />)
  await userEvent.click(screen.getByRole('button', { name: 'Capture dry' }))
  await userEvent.click(screen.getByRole('button', { name: 'Capture wet' }))
  expect(screen.getByText('Dry and wet values must differ')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  await userEvent.click(screen.getByRole('button', { name: 'Start over' }))
  expect(screen.getByRole('button', { name: 'Capture dry' })).toBeInTheDocument()
})
