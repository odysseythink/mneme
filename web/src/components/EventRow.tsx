import type { Event } from '../api/types'

export function EventRow({ e }: { e: Event }): JSX.Element {
  const when = new Date(e.ts).toLocaleTimeString()
  return (
    <li className="border-b py-2 px-3 text-sm flex gap-3">
      <span className="text-gray-500 w-20">{when}</span>
      <span className="font-medium w-40 truncate">{e.type}</span>
      <span className="text-gray-700 truncate">
        {e.project_id ? `[${e.project_id.slice(0, 8)}] ` : ''}
        {e.data ? JSON.stringify(e.data) : ''}
      </span>
    </li>
  )
}
