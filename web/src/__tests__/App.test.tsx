import { describe, expect, test, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { App } from '../App'

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.includes('/api/overview')) {
      return new Response(JSON.stringify({
        daemon: { pid: 1, uptime_s: 1, version: 't', started_at: 0 },
        totals: { projects: 0, anatomy_files: 0, cerebrum_pending: 0, open_suggestions: 0 },
        projects: [],
      }), { headers: { 'Content-Type': 'application/json' } })
    }
    if (url.includes('/api/projects')) {
      return new Response(JSON.stringify({
        projects: [{ id: 'p1', origin: '/work/a', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 }],
      }), { headers: { 'Content-Type': 'application/json' } })
    }
    if (url.includes('/api/designqc')) {
      return new Response(JSON.stringify({ available: false, reason: 'no captures yet' }),
        { headers: { 'Content-Type': 'application/json' } })
    }
    return new Response('{}', { headers: { 'Content-Type': 'application/json' } })
  }))
})

describe('App', () => {
  test('sidebar shows all 10 panel links', async () => {
    render(<App />)
    await waitFor(() => {
      for (const label of ['Overview', 'Activity', 'Cron', 'Cerebrum', 'Memory', 'Anatomy', 'BugLog', 'Suggestions', 'Token', 'DesignQC']) {
        expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
      }
    })
  })
})
