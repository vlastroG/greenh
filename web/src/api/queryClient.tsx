import { MutationCache, QueryCache, QueryClient, QueryClientProvider, type QueryClientConfig } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { errorMessage, useToast } from '../components/Toast'

type DefaultOptions = QueryClientConfig['defaultOptions']

export function createQueryClient(onError: (e: unknown) => void, defaultOptions?: DefaultOptions): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
    defaultOptions,
  })
}

// must render inside ToastProvider; failed queries and mutations show a toast
export function QueryProvider({ children, defaultOptions }: { children: ReactNode; defaultOptions?: DefaultOptions }) {
  const { show } = useToast()
  const [client] = useState(() => createQueryClient((e) => show(errorMessage(e)), defaultOptions))
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}
