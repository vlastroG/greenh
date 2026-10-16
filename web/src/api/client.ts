import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

export type Pot = components['schemas']['Pot']
export type Advice = components['schemas']['Advice']
export type Node = components['schemas']['Node']
export type HistoryPoint = components['schemas']['HistoryPoint']
export type Watering = components['schemas']['Watering']
export type ApiError = components['schemas']['Error']

// resolve fetch per call so tests can stub it; absolute base because Request rejects relative URLs
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  fetch: (req) => globalThis.fetch(req),
})

export class ApiRequestError extends Error {
  status: number
  body: ApiError

  constructor(status: number, body: ApiError) {
    super(body.message)
    this.status = status
    this.body = body
  }
}

export function unwrap<T>(r: { data?: T; error?: ApiError; response: Response }): T {
  if (r.error !== undefined || r.data === undefined) {
    const body = r.error ?? { code: 'internal', message: r.response.statusText }
    throw new ApiRequestError(r.response.status, body)
  }
  return r.data
}
