import type { Pot } from '../api/client'
import { badgeFor, type Tone } from '../lib/status'

const TONES: Record<Tone, string> = {
  ok: 'bg-green-100 text-green-800',
  water: 'bg-blue-100 text-blue-800',
  warn: 'bg-amber-100 text-amber-800',
  neutral: 'bg-gray-100 text-gray-700',
}

export function StatusBadge({ pot }: { pot: Pot }) {
  const { label, tone } = badgeFor(pot)
  return <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${TONES[tone]}`}>{label}</span>
}
