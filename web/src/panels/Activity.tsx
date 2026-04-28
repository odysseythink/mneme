import { useEffect, useState } from 'react'
import { getActivity } from '../api/activity'
import { useSSE } from '../hooks/useSSE'
import { PageHead } from '../components/PageHead'
import { Table, Pill, Empty, type PillVariant } from '../components/primitives'
import type { Event } from '../api/types'

const MAX_ROWS = 200

function variantFor(type: string): PillVariant {
  if (type.includes('error') || type.includes('fail')) return 'err'
  if (type.includes('warn')) return 'warn'
  if (type.includes('write') || type.includes('post')) return 'write'
  if (type.includes('read') || type.includes('pre')) return 'read'
  if (type.includes('hook') || type.includes('session')) return 'info'
  return 'neutral'
}

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

  if (error) {
    return (
      <>
        <PageHead title="Activity" />
        <Empty title={`failed to load: ${error}`} />
      </>
    )
  }

  return (
    <>
      <PageHead title="Activity" meta={`${items.length} event${items.length === 1 ? '' : 's'}`} />
      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--space-4)' }}>
        <Table
          columns={[
            {
              key: 'ts',
              header: 'Time',
              width: 90,
              render: (e: Event) => (
                <span className="mono" style={{ color: 'var(--text-muted)', fontSize: 11 }}>
                  {new Date(e.ts).toLocaleTimeString()}
                </span>
              ),
            },
            {
              key: 'type',
              header: 'Type',
              width: 120,
              render: (e: Event) => <Pill variant={variantFor(e.type)}>{e.type.toUpperCase()}</Pill>,
            },
            {
              key: 'data',
              header: 'Detail',
              render: (e: Event) => (
                <span className="mono" style={{ color: 'var(--text-body)', fontSize: 12 }}>
                  {e.project_id ? `[${e.project_id.slice(0, 8)}] ` : ''}
                  {e.data ? JSON.stringify(e.data) : ''}
                </span>
              ),
            },
          ]}
          rows={items}
          rowKey={(e) => `${e.ts}-${e.type}-${e.project_id ?? ''}`}
          empty={<Empty icon="≣" title="No activity yet" hint="Events appear as Claude Code makes tool calls." />}
        />
      </div>
    </>
  )
}
