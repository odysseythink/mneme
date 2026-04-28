import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getSuggestions, dismissSuggestion } from '../api/suggestions'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton, Pill } from '../components/primitives'

export function Suggestions(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getSuggestions(active) : () => Promise.resolve({ suggestions: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['suggestion.new', 'suggestion.dismissed'], () => refetch())

  if (!active) {
    return (
      <>
        <PageHead title="Suggestions" />
        <Empty title="No project selected" hint="Pick one in the sidebar." />
      </>
    )
  }
  if (error) {
    return (
      <>
        <PageHead title="Suggestions" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Suggestions" />
        <Skeleton rows={4} />
      </>
    )
  }

  if (data.suggestions.length === 0) {
    return (
      <>
        <PageHead title="Suggestions" />
        <Empty icon="∴" title="No suggestions" hint="mneme will surface optimization candidates here." />
      </>
    )
  }

  return (
    <>
      <PageHead title="Suggestions" meta={`${data.suggestions.length} open`} />
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {data.suggestions.map(s => (
          <div
            key={s.id}
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-3)',
              padding: 'var(--space-3)',
            }}
          >
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 6 }}>
              <Pill variant="info">{s.type}</Pill>
              <span className="mono" style={{ fontSize: 11, color: 'var(--text-muted)' }}>{s.generated_at}</span>
            </div>
            <div style={{ fontWeight: 500, color: 'var(--text-strong)', fontSize: 13, marginBottom: 4 }}>
              {s.title}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', whiteSpace: 'pre-wrap', marginBottom: 12 }}>
              {s.detail}
            </div>
            <ConfirmButton
              label="Dismiss"
              onConfirm={async () => { await dismissSuggestion(active, s.id); refetch() }}
            />
          </div>
        ))}
      </div>
    </>
  )
}
