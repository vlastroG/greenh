import type { QueryClient } from '@tanstack/react-query'
import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router'
import { QueryProvider } from '../api/queryClient'
import { ToastProvider } from '../components/Toast'

// without `client`, uses the app's toast-wired query client with retries off
export function renderWithProviders(ui: ReactElement, opts: { client?: QueryClient; route?: string } = {}) {
  const router = <MemoryRouter initialEntries={[opts.route ?? '/']}>{ui}</MemoryRouter>
  return render(
    <ToastProvider>
      {opts.client ? (
        <QueryClientProvider client={opts.client}>{router}</QueryClientProvider>
      ) : (
        <QueryProvider defaultOptions={{ queries: { retry: false } }}>{router}</QueryProvider>
      )}
    </ToastProvider>,
  )
}
