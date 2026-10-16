import { useNavigate, useSearchParams } from 'react-router'
import { useCreatePot, useNodes } from '../api/hooks'
import { emptyPotValues, PotForm, useSaveErrors, type PotFormValues } from '../components/PotForm'

export default function PotNew() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const nodes = useNodes()
  const create = useCreatePot()
  const { errors, clear, onError } = useSaveErrors()

  const ch = params.get('ch')
  const initial: PotFormValues = {
    ...emptyPotValues,
    node_id: params.get('node'),
    channel: ch === null || ch === '' ? null : Number(ch),
  }

  function submit(v: PotFormValues) {
    clear()
    create.mutate(
      {
        name: v.name,
        species: v.species,
        category: v.category,
        pot_ml: v.pot_ml,
        soil_type: v.soil_type,
        placement: v.placement,
        check_interval_min: v.check_interval_min,
        ...(v.node_id !== null && v.channel !== null && { node_id: v.node_id, channel: v.channel }),
        ...(v.threshold_pct !== null && { threshold_pct: v.threshold_pct }),
        ...(v.amount_fraction !== null && { amount_fraction: v.amount_fraction }),
      },
      { onSuccess: (pot) => navigate(`/pots/${pot.id}`), onError },
    )
  }

  // wait for nodes so a preselected link has its option
  if (nodes.isPending) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="max-w-xl space-y-4">
      <h1 className="text-xl font-semibold text-gray-900">New pot</h1>
      <PotForm initial={initial} nodes={nodes.data ?? []} mode="create" onSubmit={submit} busy={create.isPending} errors={errors} />
    </div>
  )
}
