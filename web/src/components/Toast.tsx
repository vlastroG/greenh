import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { ApiRequestError } from '../api/client'

const DISMISS_MS = 5000

interface ToastApi {
  show(message: string): void
}

const ToastContext = createContext<ToastApi | null>(null)

export function errorMessage(e: unknown): string {
  if (e instanceof ApiRequestError) return e.body.message
  if (e instanceof TypeError) return "Can't reach the server"
  return e instanceof Error ? e.message : 'Something went wrong'
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<{ id: number; message: string }[]>([])
  const nextId = useRef(0)

  const show = useCallback((message: string) => {
    const id = nextId.current++
    // identical messages (e.g. repeated poll failures) collapse into one
    setToasts((t) => (t.some((x) => x.message === message) ? t : [...t, { id, message }]))
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), DISMISS_MS)
  }, [])

  const api = useMemo(() => ({ show }), [show])

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2" role="region" aria-label="Notifications">
        {toasts.map((t) => (
          <div key={t.id} role="alert" className="max-w-xs rounded-md bg-gray-900 px-4 py-3 text-sm text-white shadow-lg">
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside ToastProvider')
  return ctx
}
