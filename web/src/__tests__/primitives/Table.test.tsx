import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Table } from '../../components/primitives/Table'

describe('Table', () => {
  it('renders headers and rows', () => {
    render(
      <Table
        columns={[
          { key: 'time', header: 'Time', width: 80 },
          { key: 'path', header: 'Path' },
        ]}
        rows={[
          { time: '20:24:32', path: 'cmd/foo.go' },
          { time: '20:24:18', path: 'pkg/bar.go' },
        ]}
        rowKey={(r) => r.time}
      />,
    )
    expect(screen.getByText('Time')).toBeInTheDocument()
    expect(screen.getByText('Path')).toBeInTheDocument()
    expect(screen.getByText('20:24:32')).toBeInTheDocument()
    expect(screen.getByText('cmd/foo.go')).toBeInTheDocument()
  })

  it('uses custom cell renderer', () => {
    render(
      <Table
        columns={[
          { key: 'value', header: 'Value', render: (r: { value: number }) => <strong>v{r.value}</strong> },
        ]}
        rows={[{ value: 7 }]}
        rowKey={(r) => String(r.value)}
      />,
    )
    expect(screen.getByText('v7').tagName).toBe('STRONG')
  })

  it('renders empty state when no rows', () => {
    render(
      <Table
        columns={[{ key: 'a', header: 'A' }]}
        rows={[]}
        rowKey={() => '0'}
        empty={<div>nothing here</div>}
      />,
    )
    expect(screen.getByText('nothing here')).toBeInTheDocument()
  })
})
