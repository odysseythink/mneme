import { describe, expect, test, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route, useSearchParams } from 'react-router-dom'
import { ActiveProjectProvider } from '../hooks/useActiveProject'
import { ProjectPicker } from '../components/ProjectPicker'

vi.stubGlobal('fetch', vi.fn(async (url: string) => {
  if (url.includes('/api/projects')) {
    return new Response(JSON.stringify({
      projects: [
        { id: 'p1', origin: '/work/a', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 },
        { id: 'p2', origin: '/work/b', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 },
      ],
    }), { headers: { 'Content-Type': 'application/json' } })
  }
  return new Response('{}')
}))

function ShowQuery(): JSX.Element {
  const [params] = useSearchParams()
  return <div data-testid="q">{params.get('project') ?? 'none'}</div>
}

describe('ProjectPicker', () => {
  test('writes project to URL on change', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <ActiveProjectProvider>
          <Routes>
            <Route path="/" element={<><ProjectPicker /><ShowQuery /></>} />
          </Routes>
        </ActiveProjectProvider>
      </MemoryRouter>
    )
    await screen.findByText('/work/a')
    const select = await screen.findByRole('combobox')
    await userEvent.selectOptions(select, 'p2')
    expect(screen.getByTestId('q').textContent).toBe('p2')
  })
})
