import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getSuggestions, dismissSuggestion } from '../api/suggestions'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function Suggestions(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getSuggestions(active) : () => Promise.resolve({ suggestions: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['suggestion.new', 'suggestion.dismissed'], () => refetch())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">suggestions</h1>
      {data.suggestions.length === 0 ? (
        <div className="text-gray-500">no open suggestions</div>
      ) : (
        <div className="flex flex-col gap-3">
          {data.suggestions.map(s => (
            <div key={s.id} className="rounded border bg-white p-3">
              <div className="flex gap-2 items-center mb-1">
                <span className="text-xs bg-gray-100 rounded px-2 py-0.5">{s.type}</span>
                <span className="text-xs text-gray-500">{s.generated_at}</span>
              </div>
              <div className="font-medium">{s.title}</div>
              <div className="text-sm text-gray-700 mt-1 mb-3 whitespace-pre-wrap">{s.detail}</div>
              <ConfirmButton
                label="Dismiss"
                onConfirm={async () => { await dismissSuggestion(active, s.id); refetch() }}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
