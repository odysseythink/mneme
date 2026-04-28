import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCerebrum, approveCandidate, rejectCandidate } from '../api/cerebrum'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton, Kbd } from '../components/primitives'

export function Cerebrum(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getCerebrum(active) : () => Promise.resolve({ rules: [], pending: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['cerebrum.candidate', 'cerebrum.approved', 'cerebrum.rejected'], () => refetch())

  if (!active) {
    return (
      <>
        <PageHead title="Cerebrum" />
        <Empty title="No project selected" hint="Pick one in the sidebar." />
      </>
    )
  }
  if (error) {
    return (
      <>
        <PageHead title="Cerebrum" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Cerebrum" />
        <Skeleton rows={5} />
      </>
    )
  }

  const sectionStyle = { background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--space-4)' }
  const h2Style = { fontSize: 14, fontWeight: 600 as const, color: 'var(--text-strong)', margin: '0 0 8px' }
  const labelStyle = { fontSize: 10, textTransform: 'uppercase' as const, letterSpacing: '0.06em', color: 'var(--text-muted)', fontWeight: 600 as const, marginBottom: 4 }

  return (
    <>
      <PageHead title="Cerebrum" meta={`${data.rules.length} active · ${data.pending.length} pending`} />

      <section style={sectionStyle}>
        <h2 style={h2Style}>Active rules ({data.rules.length})</h2>
        {data.rules.length === 0 ? (
          <Empty title="No rules yet" hint="Cerebrum rules guide Claude Code's behavior on this project." />
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {data.rules.map((r, i) => (
              <li
                key={i}
                style={{
                  padding: '8px 0',
                  borderBottom: i === data.rules.length - 1 ? 'none' : '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)',
                  fontSize: 12,
                }}
              >
                {r.comment && (
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>{r.comment}</div>
                )}
                <div style={{ color: 'var(--text-body)' }}>
                  <Kbd>{r.pattern}</Kbd>
                  <span style={{ color: 'var(--text-muted)', margin: '0 6px' }}>→</span>
                  {r.message}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section style={sectionStyle}>
        <h2 style={h2Style}>Pending review ({data.pending.length})</h2>
        {data.pending.length === 0 ? (
          <Empty title="No candidates pending" />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {data.pending.map((c) => (
              <div
                key={c.id}
                style={{
                  background: 'var(--bg-raised)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-3)',
                  padding: 'var(--space-3)',
                }}
              >
                <div style={labelStyle}>Trigger</div>
                <div style={{ fontStyle: 'italic', marginBottom: 8, fontSize: 12, color: 'var(--text-body)' }}>
                  "{c.trigger.phrase}"
                </div>
                {c.trigger.prior_asst && (
                  <>
                    <div style={labelStyle}>Context</div>
                    <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8, display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                      {c.trigger.prior_asst}
                    </div>
                  </>
                )}
                <div style={labelStyle}>Draft rule</div>
                <div style={{ fontSize: 12, marginBottom: 12, color: 'var(--text-body)' }}>
                  <Kbd>{c.draft_rule.pattern}</Kbd>
                  <span style={{ color: 'var(--text-muted)', margin: '0 6px' }}>→</span>
                  {c.draft_rule.message}
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <ConfirmButton
                    label="Approve"
                    onConfirm={async () => { await approveCandidate(active, c.id); refetch() }}
                  />
                  <ConfirmButton
                    label="Reject"
                    onConfirm={async () => { await rejectCandidate(active, c.id); refetch() }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  )
}
