import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { Pot } from '../api/client'
import { useCheckNow, useDeletePot, useNodes, usePot, useUpdatePot } from '../api/hooks'
import { CalibrationWizard } from '../components/CalibrationWizard'
import { PotForm, useSaveErrors, type PotFormValues } from '../components/PotForm'
import { errorMessage, useToast } from '../components/Toast'

// polls nodes fast only while mounted
function OpenWizard({ pot, onSave, onCancel }: { pot: Pot; onSave(d: number, w: number): void; onCancel(): void }) {
  const nodes = useNodes({ refetchInterval: 5000 })
  const channel = nodes.data?.find((n) => n.id === pot.node_id)?.channels.find((c) => c.ch === pot.channel)
  return <CalibrationWizard latestRaw={channel?.last_raw ?? null} onSave={onSave} onCancel={onCancel} />
}

function Calibration({ pot }: { pot: Pot }) {
  const [open, setOpen] = useState(false)
  const update = useUpdatePot(pot.id)
  const checkNow = useCheckNow(pot.id)
  const { show } = useToast()

  if (pot.node_id == null || pot.channel == null) {
    return <p className="text-sm text-gray-500">Link a sensor first to calibrate it.</p>
  }
  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-700"
      >
        Calibrate sensor
      </button>
    )
  }
  return (
    <OpenWizard
      pot={pot}
      onSave={(dry, wet) =>
        update.mutate(
          { dry_raw: dry, wet_raw: wet },
          {
            onSuccess: () => {
              setOpen(false)
              show('Calibration saved — checking…')
              checkNow.mutate()
            },
            onError: (e) => show(errorMessage(e)),
          },
        )
      }
      onCancel={() => setOpen(false)}
    />
  )
}

export default function PotSettings() {
  const { id } = useParams()
  const potId = Number(id)
  const navigate = useNavigate()
  const pot = usePot(potId)
  const nodes = useNodes()
  const update = useUpdatePot(potId)
  const checkNow = useCheckNow(potId)
  const del = useDeletePot()
  const { errors, clear, onError } = useSaveErrors()

  if (pot.isPending || nodes.isPending) return <p className="text-gray-500">Loading…</p>
  if (!pot.data) return null
  const p = pot.data

  const initial: PotFormValues = {
    name: p.name,
    species: p.species,
    category: p.category,
    pot_ml: p.pot_ml,
    soil_type: p.soil_type,
    placement: p.placement,
    node_id: p.node_id ?? null,
    channel: p.channel ?? null,
    threshold_pct: p.threshold_pct,
    amount_fraction: p.amount_fraction,
    check_interval_min: p.check_interval_min,
  }

  function submit(v: PotFormValues) {
    clear()
    // a new sensor link changes the live status, so refresh the advice right away
    const relinked = v.node_id !== null && (v.node_id !== initial.node_id || v.channel !== initial.channel)
    update.mutate(
      { ...v, threshold_pct: v.threshold_pct ?? undefined, amount_fraction: v.amount_fraction ?? undefined },
      {
        onSuccess: () => {
          if (relinked) checkNow.mutate()
          navigate(`/pots/${potId}`)
        },
        onError,
      },
    )
  }

  function remove() {
    if (!confirm(`Delete ${p.name}? This removes its history too.`)) return
    del.mutate(potId, { onSuccess: () => navigate('/') })
  }

  return (
    <div className="max-w-xl space-y-8">
      <div className="space-y-4">
        <h1 className="text-xl font-semibold text-gray-900">{p.name} settings</h1>
        <PotForm initial={initial} nodes={nodes.data ?? []} mode="edit" onSubmit={submit} busy={update.isPending} errors={errors} />
        <Link to={`/pots/${potId}`} className="text-sm text-green-700 underline">
          Back to pot
        </Link>
      </div>
      <section className="space-y-3">
        <h2 className="text-lg font-medium text-gray-900">Calibration</h2>
        <Calibration pot={p} />
      </section>
      <button
        type="button"
        onClick={remove}
        disabled={del.isPending}
        className="rounded-md border border-red-300 px-3 py-1.5 text-sm text-red-700 disabled:opacity-60"
      >
        Delete pot
      </button>
    </div>
  )
}
