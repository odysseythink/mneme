import type { ReactNode } from 'react'

export type Column<T> = {
  key: string
  header: ReactNode
  width?: number | string
  align?: 'left' | 'right' | 'center'
  render?: (row: T) => ReactNode
}

type Props<T> = {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  empty?: ReactNode
}

export function Table<T extends Record<string, unknown>>({
  columns,
  rows,
  rowKey,
  empty,
}: Props<T>): JSX.Element {
  if (rows.length === 0 && empty) {
    return <>{empty}</>
  }
  return (
    <table style={{ width: '100%', borderCollapse: 'collapse' }}>
      <thead>
        <tr>
          {columns.map((c) => (
            <th
              key={c.key}
              style={{
                width: c.width,
                textAlign: c.align ?? 'left',
                fontSize: 9,
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                color: 'var(--text-muted)',
                fontWeight: 600,
                padding: '6px 10px',
                borderBottom: '1px solid var(--border-default)',
              }}
            >
              {c.header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={rowKey(r)}>
            {columns.map((c) => (
              <td
                key={c.key}
                style={{
                  padding: '6px 10px',
                  fontSize: 11,
                  textAlign: c.align ?? 'left',
                  borderBottom: '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)',
                }}
              >
                {c.render ? c.render(r) : (r[c.key] as ReactNode)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
