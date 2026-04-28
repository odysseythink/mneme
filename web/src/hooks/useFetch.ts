import { useCallback, useEffect, useRef, useState } from 'react'

export type FetchState<T> = {
  data: T | null
  error: Error | null
  loading: boolean
  refetch: () => void
}

export function useFetch<T>(
  fn: () => Promise<T>,
  opts?: { intervalMs?: number },
): FetchState<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)
  const [loading, setLoading] = useState(true)
  const fnRef = useRef(fn)
  fnRef.current = fn

  const run = useCallback(async () => {
    try {
      const v = await fnRef.current()
      setData(v)
      setError(null)
    } catch (e) {
      setError(e as Error)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    run()
    if (!opts?.intervalMs) return
    const id = setInterval(run, opts.intervalMs)
    return () => clearInterval(id)
  }, [run, opts?.intervalMs])

  return { data, error, loading, refetch: run }
}
