import { useEffect, useState } from 'react'

interface HealthResponse {
  pid: number
  uptime_s: number
  version: string
}

export function Health(): JSX.Element {
  const [data, setData] = useState<HealthResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/health', { credentials: 'same-origin' })
      .then((r) => {
        if (!r.ok) throw new Error('HTTP ' + r.status)
        return r.json() as Promise<HealthResponse>
      })
      .then(setData)
      .catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <p>error: {error}</p>
  if (!data) return <p>checking…</p>
  return (
    <p>
      pid={data.pid} uptime={data.uptime_s}s version={data.version}
    </p>
  )
}
