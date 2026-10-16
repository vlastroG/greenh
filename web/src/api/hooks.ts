import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { components } from './schema'
import { ApiRequestError, api, unwrap } from './client'
import { rangeQuery, type Range } from '../lib/status'

const POLL_MS = 30_000

// the form handles these statuses itself, so the global error toast skips them
const FORM_ERRORS = { handledErrors: [400, 409] }

type PotCreate = components['schemas']['PotCreate']
type PotPatch = components['schemas']['PotPatch']
type WateringIn = components['schemas']['WateringIn']

export function usePots() {
  return useQuery({
    queryKey: ['pots'],
    queryFn: async () => unwrap(await api.GET('/pots')),
    refetchInterval: POLL_MS,
  })
}

export function usePot(id: number) {
  return useQuery({
    queryKey: ['pot', id],
    queryFn: async () => unwrap(await api.GET('/pots/{id}', { params: { path: { id } } })),
    refetchInterval: POLL_MS,
  })
}

export function useAdvice(id: number) {
  return useQuery({
    queryKey: ['advice', id],
    queryFn: async () => unwrap(await api.GET('/pots/{id}/advice', { params: { path: { id } } })),
    refetchInterval: POLL_MS,
  })
}

export function useHistory(id: number, range: Range) {
  return useQuery({
    queryKey: ['history', id, range],
    queryFn: async () => {
      const query = rangeQuery(range, new Date())
      return unwrap(await api.GET('/pots/{id}/readings', { params: { path: { id }, query } }))
    },
    refetchInterval: POLL_MS,
  })
}

export function useWaterings(id: number) {
  return useQuery({
    queryKey: ['waterings', id],
    queryFn: async () => unwrap(await api.GET('/pots/{id}/waterings', { params: { path: { id } } })),
    refetchInterval: POLL_MS,
  })
}

export function useNodes(opts: { refetchInterval?: number } = {}) {
  return useQuery({
    queryKey: ['nodes'],
    queryFn: async () => unwrap(await api.GET('/nodes')),
    refetchInterval: opts.refetchInterval ?? POLL_MS,
  })
}

export function useCheckNow(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/pots/{id}/advice', { params: { path: { id } } })),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: ['pots'] }),
        qc.invalidateQueries({ queryKey: ['pot', id] }),
        qc.invalidateQueries({ queryKey: ['advice', id] }),
      ]),
  })
}

export function useLogWatering(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: WateringIn) =>
      unwrap(await api.POST('/pots/{id}/waterings', { params: { path: { id } }, body })),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: ['waterings', id] }),
        qc.invalidateQueries({ queryKey: ['pot', id] }),
      ]),
  })
}

export function useCreatePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: PotCreate) => unwrap(await api.POST('/pots', { body })),
    meta: FORM_ERRORS,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pots'] }),
  })
}

export function useUpdatePot(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: PotPatch) =>
      unwrap(await api.PATCH('/pots/{id}', { params: { path: { id } }, body })),
    meta: FORM_ERRORS,
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: ['pots'] }),
        qc.invalidateQueries({ queryKey: ['pot', id] }),
        qc.invalidateQueries({ queryKey: ['history', id] }),
        qc.invalidateQueries({ queryKey: ['nodes'] }),
      ]),
  })
}

export function useDeletePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) => {
      const r = await api.DELETE('/pots/{id}', { params: { path: { id } } })
      if (r.error !== undefined) throw new ApiRequestError(r.response.status, r.error)
    },
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: ['pots'] }),
        qc.invalidateQueries({ queryKey: ['nodes'] }),
      ]),
  })
}

export function useRenameNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, name }: { id: string; name: string }) =>
      unwrap(await api.PATCH('/nodes/{id}', { params: { path: { id } }, body: { name } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['nodes'] }),
  })
}
