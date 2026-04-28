import { useEffect, useState } from 'react'
import { EventRow } from '../components/EventRow'
import { getActivity } from '../api/activity'
import { useSSE } from '../hooks/useSSE'
import type { Event } from '../api/types'

const MAX_ROWS = 200

export function Activity(): JSX.Element {
  const [items, setItems] = useState<Event[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getActivity({ limit: 100 })
      .then(r => setItems(r.events))
      .catch(e => setError((e as Error).message))
  }, [])

  useSSE(null, (e) => {
    setItems(prev => {
      const next = [e, ...prev]
      return next.length > MAX_ROWS ? next.slice(0, MAX_ROWS) : next
    })
  })

  if (error) return <div className="text-red-700">failed to load: {error}</div>
  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">activity</h1>
      <ul className="rounded border bg-white">
        {items.length === 0 && <li className="py-3 px-3 text-gray-500">no events yet</li>}
        {items.map(e => <EventRow key={`${e.ts}-${e.type}-${e.project_id ?? ''}`} e={e} />)}
      </ul>
    </div>
  )
}
