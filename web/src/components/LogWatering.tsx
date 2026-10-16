import { useState, type FormEvent } from 'react'
import { useLogWatering } from '../api/hooks'

const MIN_ML = 1
const MAX_ML = 20000

export function LogWatering({ potId, suggestedMl }: { potId: number; suggestedMl: number | null }) {
  // until the user types, follow the suggestion, which may arrive after first render
  const [typed, setTyped] = useState<string | null>(null)
  const text = typed ?? (suggestedMl == null ? '' : String(suggestedMl))
  const { mutate, isPending, data, reset } = useLogWatering(potId)

  const amount = text === '' ? NaN : Number(text)
  const valid = Number.isInteger(amount) && amount >= MIN_ML && amount <= MAX_ML

  function submit(e: FormEvent) {
    e.preventDefault()
    if (valid) mutate({ amount_ml: amount, source: 'manual' })
  }

  return (
    <form onSubmit={submit} className="flex flex-wrap items-end gap-2">
      <label className="text-sm text-gray-700">
        <span className="mb-1 block">Amount (ml)</span>
        <input
          type="number"
          step={1}
          min={MIN_ML}
          max={MAX_ML}
          value={text}
          onChange={(e) => {
            setTyped(e.target.value)
            reset()
          }}
          className="w-28 rounded-md border border-gray-300 px-2 py-1.5"
        />
      </label>
      <button
        type="submit"
        disabled={!valid || isPending}
        className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-60"
      >
        Log watering
      </button>
      {data && <span className="text-sm text-green-700">Logged {data.amount_ml} ml</span>}
    </form>
  )
}
