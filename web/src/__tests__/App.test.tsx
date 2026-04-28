import React from "react"
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
    return new Response('{}', { headers: { 'Content-Type': 'application/json' } })
  }))
})

describe('App', () => {
  test('renders the AppShell + Overview without throwing', async () => {
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText('overview')).toBeInTheDocument()
    })
  })
})
