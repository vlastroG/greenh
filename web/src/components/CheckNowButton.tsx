import { useCheckNow } from '../api/hooks'

export function CheckNowButton({ potId }: { potId: number }) {
  const { mutate, isPending } = useCheckNow(potId)
  return (
    <button
      type="button"
      disabled={isPending}
      onClick={() => mutate()}
      className="rounded-md bg-green-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-green-800 disabled:opacity-60"
    >
      {isPending ? 'Checking…' : 'Check now'}
    </button>
  )
}
