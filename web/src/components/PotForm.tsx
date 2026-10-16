import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiRequestError, type Node, type Pot } from '../api/client'
import { useToast } from './Toast'

export type PotFormValues = {
  name: string
  species: string
  category: Pot['category']
  pot_ml: number
  soil_type: string
  placement: string
  node_id: string | null
  channel: number | null
  threshold_pct: number | null
  amount_fraction: number | null
  check_interval_min: number
}

export const emptyPotValues: PotFormValues = {
  name: '',
  species: '',
  category: 'foliage',
  pot_ml: 2000,
  soil_type: '',
  placement: '',
  node_id: null,
  channel: null,
  threshold_pct: null,
  amount_fraction: null,
  check_interval_min: 360,
}

const CATEGORIES: Pot['category'][] = ['succulent', 'foliage', 'flowering', 'herb']
const ADVISOR_HINT = 'Sent to the advisor — avoid personal details.'
const SEP = '|'
const input = 'w-full rounded-md border border-gray-300 px-2 py-1.5'

const FIELDS = new Set([
  'name', 'species', 'category', 'pot_ml', 'soil_type', 'placement',
  'node_id', 'channel', 'threshold_pct', 'amount_fraction', 'check_interval_min',
])

// the global handler skips 400 and 409, so this toasts those unless field messages cover the 400;
// other statuses are left to the global toast
export function useSaveErrors() {
  const { show } = useToast()
  const [errors, setErrors] = useState<Record<string, string>>({})
  return {
    errors,
    clear: () => setErrors({}),
    onError(e: unknown) {
      if (!(e instanceof ApiRequestError)) return
      if (e.status === 400) {
        const mapped = (e.body.details ?? []).filter((d) => FIELDS.has(d.field))
        if (mapped.length === 0) show(e.body.message)
        else setErrors(Object.fromEntries(mapped.map((d) => [d.field, d.message])))
      } else if (e.status === 409) {
        show(e.body.message)
      }
    },
  }
}

const linkKey = (id: string | null, ch: number | null) => (id === null || ch === null ? '' : `${id}${SEP}${ch}`)
const optionalNumber = (s: string) => (s.trim() === '' ? null : Number(s))

function Field({ label, error, hint, children }: { label: string; error?: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block text-sm text-gray-700">
      <span className="mb-1 block">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-gray-500">{hint}</span>}
      {error && <span className="mt-1 block text-xs text-red-600">{error}</span>}
    </label>
  )
}

export function PotForm({
  initial,
  nodes,
  mode,
  onSubmit,
  busy,
  errors = {},
}: {
  initial: PotFormValues
  nodes: Node[]
  mode: 'create' | 'edit'
  onSubmit(v: PotFormValues): void
  busy: boolean
  errors?: Record<string, string>
}) {
  const [name, setName] = useState(initial.name)
  const [species, setSpecies] = useState(initial.species)
  const [category, setCategory] = useState(initial.category)
  const [potMl, setPotMl] = useState(String(initial.pot_ml))
  const [soil, setSoil] = useState(initial.soil_type)
  const [placement, setPlacement] = useState(initial.placement)
  const [link, setLink] = useState(linkKey(initial.node_id, initial.channel))
  const [threshold, setThreshold] = useState(initial.threshold_pct === null ? '' : String(initial.threshold_pct))
  const [fraction, setFraction] = useState(
    initial.amount_fraction === null ? '' : String(Math.round(initial.amount_fraction * 1000) / 10),
  )
  const [hours, setHours] = useState(String(initial.check_interval_min / 60))

  const ownLink = linkKey(initial.node_id, initial.channel)
  const ruleHint = mode === 'create' ? 'Leave empty to use the category default' : undefined
  const placeholder = mode === 'create' ? 'Category default' : undefined

  function submit(e: FormEvent) {
    e.preventDefault()
    const sep = link.lastIndexOf(SEP)
    const pct = optionalNumber(fraction)
    onSubmit({
      name: name.trim(),
      species,
      category,
      pot_ml: Number(potMl),
      soil_type: soil,
      placement,
      node_id: link === '' ? null : link.slice(0, sep),
      channel: link === '' ? null : Number(link.slice(sep + 1)),
      threshold_pct: optionalNumber(threshold),
      amount_fraction: pct === null ? null : pct / 100,
      check_interval_min: Math.round(Number(hours) * 60),
    })
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <Field label="Name" error={errors.name}>
        <input required value={name} onChange={(e) => setName(e.target.value)} className={input} />
      </Field>
      <Field label="Species" error={errors.species} hint={ADVISOR_HINT}>
        <input value={species} onChange={(e) => setSpecies(e.target.value)} className={input} />
      </Field>
      <Field label="Category" error={errors.category}>
        <select value={category} onChange={(e) => setCategory(e.target.value as Pot['category'])} className={input}>
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Pot volume (ml)" error={errors.pot_ml}>
        <input type="number" required value={potMl} onChange={(e) => setPotMl(e.target.value)} className={input} />
      </Field>
      <Field label="Soil type" error={errors.soil_type} hint={ADVISOR_HINT}>
        <input value={soil} onChange={(e) => setSoil(e.target.value)} className={input} />
      </Field>
      <Field label="Placement" error={errors.placement} hint={ADVISOR_HINT}>
        <input value={placement} onChange={(e) => setPlacement(e.target.value)} className={input} />
      </Field>
      <Field label="Sensor" error={errors.node_id ?? errors.channel}>
        <select value={link} onChange={(e) => setLink(e.target.value)} className={input}>
          <option value="">Not linked</option>
          {nodes.flatMap((n) =>
            n.channels.map((c) => {
              const key = linkKey(n.id, c.ch)
              return (
                <option key={key} value={key} disabled={c.pot_id !== null && key !== ownLink}>
                  {`${n.name || n.id} · ch ${c.ch}`}
                </option>
              )
            }),
          )}
        </select>
      </Field>
      <Field label="Dry threshold (%)" error={errors.threshold_pct} hint={ruleHint}>
        <input
          type="number"
          value={threshold}
          placeholder={placeholder}
          onChange={(e) => setThreshold(e.target.value)}
          className={input}
        />
      </Field>
      <Field label="Water amount (% of pot volume)" error={errors.amount_fraction} hint={ruleHint}>
        <input
          type="number"
          value={fraction}
          placeholder={placeholder}
          onChange={(e) => setFraction(e.target.value)}
          className={input}
        />
      </Field>
      <Field label="Check every (hours)" error={errors.check_interval_min}>
        <input
          type="number"
          step="any"
          required
          value={hours}
          onChange={(e) => setHours(e.target.value)}
          className={input}
        />
      </Field>
      <button
        type="submit"
        disabled={busy}
        className="rounded-md bg-green-700 px-4 py-2 text-sm font-medium text-white hover:bg-green-800 disabled:opacity-60"
      >
        {mode === 'create' ? 'Create pot' : 'Save'}
      </button>
    </form>
  )
}
