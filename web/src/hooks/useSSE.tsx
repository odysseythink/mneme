import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import type { Event } from '../api/types'

type Subscriber = (e: Event) => void

type SSEContextValue = {
  connected: boolean
  latencyMs: number | null
  subscribe: (types: string[] | null, fn: Subscriber) => () => void
}

const SSEContext = createContext<SSEContextValue | null>(null)

export function SSEProvider({ children }: { children: ReactNode }): JSX.Element {
  const [connected, setConnected] = useState(false)
  const [latencyMs, setLatencyMs] = useState<number | null>(null)
  const lastEventTimeRef = useRef<number | null>(null)
  const subsRef = useRef<Set<{ types: string[] | null; fn: Subscriber }>>(new Set())

  useEffect(() => {
    let es: EventSource | null = null
    let cancelled = false
    let backoffMs = 1000

    const open = () => {
      if (cancelled) return
      es = new EventSource('/events')
      es.onopen = () => {
        setConnected(true)
        backoffMs = 1000
      }
      es.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data) as Event
          lastEventTimeRef.current = Date.now()
          for (const sub of subsRef.current) {
            if (sub.types === null || sub.types.includes(data.type)) {
              sub.fn(data)
            }
          }
        } catch { /* ignore parse errors */ }
      }
      es.onerror = () => {
        setConnected(false)
        es?.close()
        if (!cancelled) {
          setTimeout(open, backoffMs)
          backoffMs = Math.min(backoffMs * 2, 10_000)
        }
      }
    }

    open()
    const tick = setInterval(() => {
      if (lastEventTimeRef.current === null) {
        setLatencyMs(null)
      } else {
        setLatencyMs(Date.now() - lastEventTimeRef.current)
      }
    }, 1000)

    return () => { cancelled = true; es?.close(); clearInterval(tick) }
  }, [])

  const value: SSEContextValue = {
    connected,
    latencyMs,
    subscribe: useCallback((types, fn) => {
      const entry = { types, fn }
      subsRef.current.add(entry)
      return () => { subsRef.current.delete(entry) }
    }, []),
  }
  return <SSEContext.Provider value={value}>{children}</SSEContext.Provider>
}

export function useSSE(types: string[] | null, fn: Subscriber): void {
  const ctx = useContext(SSEContext)
  if (!ctx) throw new Error('useSSE must be used inside SSEProvider')
  const fnRef = useRef(fn)
  fnRef.current = fn
  useEffect(() => {
    return ctx.subscribe(types, (e) => fnRef.current(e))
  }, [ctx, types])
}

export function useSSEStatus(): { connected: boolean; latencyMs: number | null } {
  const ctx = useContext(SSEContext)
  return { connected: ctx?.connected ?? false, latencyMs: ctx?.latencyMs ?? null }
}
