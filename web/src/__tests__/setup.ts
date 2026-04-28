import '@testing-library/jest-dom'

// Stub EventSource — the smoke test never connects.
class StubEventSource {
  url: string
  readyState = 0
  onopen: any = null
  onmessage: any = null
  onerror: any = null
  constructor(url: string) { this.url = url }
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
;(globalThis as any).EventSource = StubEventSource
