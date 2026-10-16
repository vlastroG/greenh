import { useState } from 'react'

const btn = 'rounded-md px-3 py-1.5 text-sm font-medium disabled:opacity-60'

export function CalibrationWizard({
  latestRaw,
  onSave,
  onCancel,
}: {
  latestRaw: number | null
  onSave(dry: number, wet: number): void
  onCancel(): void
}) {
  const [dry, setDry] = useState<number | null>(null)
  const [wet, setWet] = useState<number | null>(null)

  const step = dry === null ? 1 : wet === null ? 2 : 3
  const equal = dry !== null && dry === wet

  return (
    <div className="space-y-3 rounded-md border border-gray-200 p-4">
      {step < 3 && (
        <>
          <p className="text-sm text-gray-900">
            {step === 1
              ? 'Hold the sensor in the air, then press Capture dry'
              : 'Put the sensor in water up to the line, then press Capture wet'}
          </p>
          {latestRaw === null ? (
            <p className="text-sm text-gray-500">Waiting for a reading from the sensor</p>
          ) : (
            <p className="text-sm text-gray-700">
              Current raw value: <span className="font-mono">{latestRaw}</span>
            </p>
          )}
          <button
            type="button"
            disabled={latestRaw === null}
            onClick={() => (step === 1 ? setDry(latestRaw) : setWet(latestRaw))}
            className={`${btn} bg-green-700 text-white hover:bg-green-800`}
          >
            {step === 1 ? 'Capture dry' : 'Capture wet'}
          </button>
        </>
      )}
      {step === 3 && (
        <>
          <p className="text-sm text-gray-900">{`Dry ${dry} · Wet ${wet}`}</p>
          {equal && <p className="text-sm text-red-600">Dry and wet values must differ</p>}
          <div className="flex gap-2">
            <button
              type="button"
              disabled={equal}
              onClick={() => onSave(dry as number, wet as number)}
              className={`${btn} bg-green-700 text-white hover:bg-green-800`}
            >
              Save
            </button>
            <button
              type="button"
              onClick={() => {
                setDry(null)
                setWet(null)
              }}
              className={`${btn} border border-gray-300 text-gray-700`}
            >
              Start over
            </button>
          </div>
        </>
      )}
      <button type="button" onClick={onCancel} className="text-sm text-gray-500 underline">
        Cancel
      </button>
    </div>
  )
}
