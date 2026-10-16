import { Link } from 'react-router'
import { usePots } from '../api/hooks'
import { PotCard } from '../components/PotCard'

export default function Overview() {
  const { data: pots, isPending } = usePots()

  if (isPending) return <p className="text-gray-500">Loading…</p>
  // on a failed first load the toast explains it
  if (!pots) return null
  if (pots.length === 0) {
    return (
      <p className="text-gray-600">
        No pots yet. Go to{' '}
        <Link to="/nodes" className="text-green-700 underline">
          Nodes
        </Link>{' '}
        to create one from a sensor channel.
      </p>
    )
  }
  const now = new Date()
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {pots.map((pot) => (
        <PotCard key={pot.id} pot={pot} now={now} />
      ))}
    </div>
  )
}
