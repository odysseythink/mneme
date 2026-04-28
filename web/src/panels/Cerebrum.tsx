import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCerebrum, approveCandidate, rejectCandidate } from '../api/cerebrum'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function Cerebrum(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getCerebrum(active) : () => Promise.resolve({ rules: [], pending: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['cerebrum.candidate', 'cerebrum.approved', 'cerebrum.rejected'], () => refetch())

  if (!active) return <NoProject />
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">cerebrum</h1>

      <section>
        <h2 className="text-lg font-medium mb-2">active rules ({data.rules.length})</h2>
        {data.rules.length === 0 ? (
          <div className="text-sm text-gray-500">no rules yet</div>
        ) : (
          <ul className="rounded border bg-white">
            {data.rules.map((r, i) => (
              <li key={i} className="border-b last:border-b-0 px-3 py-2 text-sm">
                <div className="text-gray-500 text-xs mb-1">{r.comment}</div>
                <div><span className="font-mono bg-gray-100 px-1 rounded">{r.pattern}</span> → {r.message}</div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="text-lg font-medium mb-2">pending review ({data.pending.length})</h2>
        {data.pending.length === 0 ? (
          <div className="text-sm text-gray-500">no candidates pending</div>
        ) : (
          <div className="flex flex-col gap-3">
            {data.pending.map((c) => (
              <div key={c.id} className="rounded border bg-white p-3">
                <div className="text-xs text-gray-500 mb-1">trigger</div>
                <div className="italic mb-2">"{c.trigger.phrase}"</div>
                {c.trigger.prior_asst && (
                  <>
                    <div className="text-xs text-gray-500 mb-1">context</div>
                    <div className="text-sm text-gray-700 mb-2 line-clamp-3">{c.trigger.prior_asst}</div>
                  </>
                )}
                <div className="text-xs text-gray-500 mb-1">draft rule</div>
                <div className="text-sm mb-3">
                  <span className="font-mono bg-gray-100 px-1 rounded">{c.draft_rule.pattern}</span> → {c.draft_rule.message}
                </div>
                <div className="flex gap-2">
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
    </div>
  )
}

function NoProject(): JSX.Element {
  return <div className="text-gray-500">No project selected. Pick one in the sidebar.</div>
}
