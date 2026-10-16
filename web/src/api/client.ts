import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

export type Pot = components['schemas']['Pot']
export type Advice = components['schemas']['Advice']
export type Node = components['schemas']['Node']
export type HistoryPoint = components['schemas']['HistoryPoint']
export type Watering = components['schemas']['Watering']
export type ApiError = components['schemas']['Error']

export const api = createClient<paths>({ baseUrl: '/api/v1' })

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
