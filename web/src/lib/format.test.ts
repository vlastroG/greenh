import { expect, test } from 'vitest'
import { formatAgo, formatPct } from './format'

test('formatAgo', () => {
  const now = new Date('2026-10-16T12:00:00Z')
  expect(formatAgo('2026-10-16T11:59:40Z', now)).toBe('just now')
  expect(formatAgo('2026-10-16T11:55:00Z', now)).toBe('5 min ago')
  expect(formatAgo('2026-10-16T10:00:00Z', now)).toBe('2 h ago')
  expect(formatAgo('2026-10-13T12:00:00Z', now)).toBe('3 d ago')
})

test('formatAgo treats future timestamps as just now', () => {
  expect(formatAgo('2026-10-16T12:00:30Z', new Date('2026-10-16T12:00:00Z'))).toBe('just now')
})

test('formatPct', () => {
  expect(formatPct(41.6)).toBe('42%')
  expect(formatPct(0)).toBe('0%')
  expect(formatPct(null)).toBe('—')
  expect(formatPct(undefined)).toBe('—')
})
