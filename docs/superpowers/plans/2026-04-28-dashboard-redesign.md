# Dashboard redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dashboard's default-Tailwind visual layer with a coherent Grafana/Datadog cockpit aesthetic — design tokens, dark/light themes, an atomic component library, and migrated panels — without changing dashboard functionality.

**Architecture:** Token-driven CSS custom properties (`:root` for dark, `[data-theme="light"]` for light overrides) registered with Tailwind 4's `@theme` so utility classes resolve to the same vars used by component primitives. A new `Sidebar` / `TopBar` / `PageHead` shell wraps every panel; ~10 atom primitives compose every panel's body. A `useTheme` hook handles persistence + system listener with an inline `<head>` script preventing first-paint flash.

**Tech Stack:** React 19 · Tailwind 4 · TypeScript · Vite 6 · Vitest + React Testing Library · `@fontsource/inter` + `@fontsource/jetbrains-mono`

**Spec:** `docs/superpowers/specs/2026-04-28-dashboard-redesign-design.md`

---

## File Structure

**New files:**
- `web/src/styles/tokens.css` — `:root` + `[data-theme="light"]` CSS variable declarations
- `web/src/styles/base.css` — html/body resets, body color transition
- `web/src/styles/fonts.css` — `@import` for `@fontsource/*`
- `web/src/components/Sidebar.tsx` — extracted nav, three groups
- `web/src/components/TopBar.tsx` — project picker + search + SSE + theme + settings
- `web/src/components/PageHead.tsx` — title + meta + actions row
- `web/src/components/primitives/Stat.tsx`
- `web/src/components/primitives/Pill.tsx`
- `web/src/components/primitives/Dot.tsx`
- `web/src/components/primitives/Kbd.tsx`
- `web/src/components/primitives/Button.tsx`
- `web/src/components/primitives/Input.tsx`
- `web/src/components/primitives/Table.tsx`
- `web/src/components/primitives/Empty.tsx`
- `web/src/components/primitives/Skeleton.tsx`
- `web/src/components/primitives/Toast.tsx`
- `web/src/components/primitives/Modal.tsx`
- `web/src/components/primitives/Sparkline.tsx`
- `web/src/components/primitives/index.ts` — re-export barrel
- `web/src/hooks/useTheme.tsx` — storage + system listener + resolved value
- `web/src/__tests__/useTheme.test.tsx`
- `web/src/__tests__/primitives/Stat.test.tsx`
- `web/src/__tests__/primitives/Pill.test.tsx`
- `web/src/__tests__/primitives/Button.test.tsx`
- `web/src/__tests__/primitives/Table.test.tsx`
- `web/src/__tests__/primitives/Sparkline.test.tsx`
- `web/src/__tests__/primitives/Modal.test.tsx`
- `web/src/__tests__/PageHead.test.tsx`

**Modified files:**
- `web/index.html` — inline anti-flash script in `<head>`
- `web/package.json` — add `@fontsource/*` deps
- `web/src/styles.css` — rewrite to import order + `@theme` block only
- `web/src/components/AppShell.tsx` — rewrite to compose `Sidebar` + `TopBar` + `<Outlet>`
- `web/src/hooks/useSSE.tsx` — extend `useSSEConnected` signature to return `{ connected, latencyMs }`
- All 10 `web/src/panels/*.tsx` — wrap in `PageHead`, replace ad-hoc cards/tables with primitives, swap utility colors for token classes

**Removed files:**
- `web/src/components/HealthCard.tsx` — replaced by `Stat` + `Dot` composition in Overview
- `web/src/components/Sparkline.tsx` — replaced by primitive (the existing 22-line file becomes the basis for the new primitive at `primitives/Sparkline.tsx`, but moved + extended)

---

## Task 1: Install font packages

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install dependencies**

```bash
cd web && pnpm add @fontsource/inter@^5 @fontsource/jetbrains-mono@^5
```

Expected: two `dependencies` entries appear in `web/package.json`; `pnpm-lock.yaml` updates.

- [ ] **Step 2: Verify install**

```bash
cd web && ls node_modules/@fontsource/inter/400.css node_modules/@fontsource/jetbrains-mono/400.css
```

Expected: both files exist.

- [ ] **Step 3: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml
git commit -m "feat(web): add Inter and JetBrains Mono font packages"
```

---

## Task 2: Create design tokens CSS

**Files:**
- Create: `web/src/styles/tokens.css`

- [ ] **Step 1: Create the file**

Create `web/src/styles/tokens.css` with:

```css
/* Dark theme is the default. */
:root {
  /* Surfaces */
  --bg-base: #0d1017;
  --bg-surface: #11141b;
  --bg-raised: #161a23;
  --bg-selected: rgba(108, 182, 255, 0.08);
  --border-default: #1c2230;
  --border-strong: #232a3a;

  /* Text */
  --text-strong: #ffffff;
  --text-body: #c5c8d6;
  --text-muted: #6e7686;
  --text-faint: #4a5163;

  /* Accents · status */
  --accent: #6cb6ff;
  --ok: #6dd47e;
  --warn: #ffaa3d;
  --err: #f4747a;
  --info: #b48ead;
  --neutral: #8b95a7;

  /* Spacing (4px base) */
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-6: 24px;
  --space-8: 32px;

  /* Radii */
  --radius-0: 0;
  --radius-1: 2px;
  --radius-2: 4px;
  --radius-3: 6px;
  --radius-4: 8px;

  /* Density */
  --row-height: 28px;
  --panel-pad: 16px;
  --sidebar-width: 200px;
  --header-height: 44px;

  /* Type families */
  --font-sans: "Inter", system-ui, -apple-system, "Segoe UI", sans-serif;
  --font-mono: "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace;
}

/* Light theme overrides — only color values change. */
[data-theme="light"] {
  --bg-base: #fafbfc;
  --bg-surface: #ffffff;
  --bg-raised: #f4f6f8;
  --bg-selected: rgba(0, 102, 204, 0.08);
  --border-default: #e5e8ed;
  --border-strong: #d1d5db;

  --text-strong: #0a0d12;
  --text-body: #1a1d22;
  --text-muted: #6e7686;
  --text-faint: #a1a7b3;

  --accent: #0066cc;
  --ok: #2e9c47;
  --warn: #d97706;
  --err: #dc2626;
  --info: #7c3aed;
  --neutral: #4b5563;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/tokens.css
git commit -m "feat(web): add design tokens CSS for dark and light themes"
```

---

## Task 3: Create base CSS

**Files:**
- Create: `web/src/styles/base.css`

- [ ] **Step 1: Create the file**

Create `web/src/styles/base.css` with:

```css
html {
  color-scheme: dark light;
  background: var(--bg-base);
  color: var(--text-body);
  font-family: var(--font-sans);
  font-size: 12px;
  line-height: 1.5;
}

body {
  margin: 0;
  background: var(--bg-base);
  color: var(--text-body);
  transition: background-color 200ms ease, color 200ms ease;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

#root {
  min-height: 100vh;
}

/* Tabular numerics by default for any element with .tabular */
.tabular {
  font-variant-numeric: tabular-nums;
}

/* Mono utility for elements that should always use the mono family */
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

/* Skeleton shimmer used by Skeleton primitive */
@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

/* Breathing dot used by Dot primitive when status === "ok" */
@keyframes breathe {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/base.css
git commit -m "feat(web): add base CSS with theme transition and shared keyframes"
```

---

## Task 4: Create fonts CSS

**Files:**
- Create: `web/src/styles/fonts.css`

- [ ] **Step 1: Create the file**

Create `web/src/styles/fonts.css` with:

```css
/* Inter — UI text. Only weights we actually use. */
@import "@fontsource/inter/400.css";
@import "@fontsource/inter/500.css";
@import "@fontsource/inter/600.css";

/* JetBrains Mono — data, code, time. */
@import "@fontsource/jetbrains-mono/400.css";
@import "@fontsource/jetbrains-mono/500.css";
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/fonts.css
git commit -m "feat(web): import Inter and JetBrains Mono font weights used by tokens"
```

---

## Task 5: Rewrite styles.css with @theme block

**Files:**
- Modify: `web/src/styles.css` (full rewrite — current contents are 6 lines that conflict with the new shell)

- [ ] **Step 1: Replace file contents**

Replace `web/src/styles.css` entirely with:

```css
@import "tailwindcss";
@import "./styles/fonts.css";
@import "./styles/tokens.css";
@import "./styles/base.css";

/* Register tokens with Tailwind 4 so utility classes (bg-base, text-muted,
   border-default, etc.) resolve to the same CSS variables used by primitives. */
@theme {
  --color-base: var(--bg-base);
  --color-surface: var(--bg-surface);
  --color-raised: var(--bg-raised);
  --color-selected: var(--bg-selected);
  --color-border-default: var(--border-default);
  --color-border-strong: var(--border-strong);

  --color-strong: var(--text-strong);
  --color-body: var(--text-body);
  --color-muted: var(--text-muted);
  --color-faint: var(--text-faint);

  --color-accent: var(--accent);
  --color-ok: var(--ok);
  --color-warn: var(--warn);
  --color-err: var(--err);
  --color-info: var(--info);
  --color-neutral: var(--neutral);

  --font-sans: var(--font-sans);
  --font-mono: var(--font-mono);
}
```

- [ ] **Step 2: Verify build still works**

```bash
cd web && pnpm build 2>&1 | tail -10
```

Expected: build succeeds, `dist/index.html` and `dist/assets/*` produced. Some Tailwind warnings about unused variants are acceptable; **no errors**.

- [ ] **Step 3: Commit**

```bash
git add web/src/styles.css
git commit -m "feat(web): wire tokens into Tailwind 4 @theme registry"
```

---

## Task 6: Add anti-flash script to index.html

**Files:**
- Modify: `web/index.html` (add `<script>` to `<head>` before any stylesheet)

- [ ] **Step 1: Update index.html**

Replace the contents of `web/index.html` with:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>mneme dashboard</title>
    <script>
      (function () {
        var s = localStorage.getItem('mneme.theme') || 'system';
        var t = s === 'system'
          ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
          : s;
        document.documentElement.setAttribute('data-theme', t);
      })();
    </script>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Smoke test in dev**

```bash
cd web && pnpm dev &
sleep 3
curl -s http://localhost:5173 | grep -q 'data-theme' && echo "anti-flash script present"
kill %1
```

The grep won't match because the attribute is set at runtime, but the literal string `data-theme` should appear inside the inline `<script>`. The expected output is `anti-flash script present`.

- [ ] **Step 3: Commit**

```bash
git add web/index.html
git commit -m "feat(web): inline anti-flash theme script in index.html head"
```

---

## Task 7: useTheme hook — failing test

**Files:**
- Create: `web/src/__tests__/useTheme.test.tsx`

- [ ] **Step 1: Write the test file**

Create `web/src/__tests__/useTheme.test.tsx`:

```tsx
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useTheme } from '../hooks/useTheme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })
  afterEach(() => { vi.restoreAllMocks() })

  function mockMatchMedia(prefersDark: boolean) {
    const listeners: ((e: MediaQueryListEvent) => void)[] = []
    window.matchMedia = vi.fn().mockReturnValue({
      matches: prefersDark,
      addEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) => listeners.push(fn),
      removeEventListener: vi.fn(),
    })
    return { fireChange: (next: boolean) => listeners.forEach(fn => fn({ matches: next } as MediaQueryListEvent)) }
  }

  it('defaults to "system" when nothing in storage', () => {
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('system')
    expect(result.current.resolved).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('reads stored "light" preference', () => {
    localStorage.setItem('mneme.theme', 'light')
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('light')
    expect(result.current.resolved).toBe('light')
  })

  it('cycles dark → light → system on setNext', () => {
    localStorage.setItem('mneme.theme', 'dark')
    mockMatchMedia(false)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('dark')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('light')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('system')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('dark')
  })

  it('persists preference to localStorage', () => {
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    act(() => result.current.setPreference('light'))
    expect(localStorage.getItem('mneme.theme')).toBe('light')
  })

  it('reacts to OS theme change in system mode', () => {
    const mq = mockMatchMedia(false)
    const { result } = renderHook(() => useTheme())
    expect(result.current.resolved).toBe('light')
    act(() => mq.fireChange(true))
    expect(result.current.resolved).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })
})
```

- [ ] **Step 2: Run the test (should fail — hook doesn't exist)**

```bash
cd web && pnpm test useTheme 2>&1 | tail -10
```

Expected: FAIL with module-not-found for `../hooks/useTheme`.

---

## Task 8: useTheme hook — implementation

**Files:**
- Create: `web/src/hooks/useTheme.tsx`

- [ ] **Step 1: Write the hook**

Create `web/src/hooks/useTheme.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react'

export type ThemePreference = 'dark' | 'light' | 'system'
export type ThemeResolved = 'dark' | 'light'

const STORAGE_KEY = 'mneme.theme'
const CYCLE: ThemePreference[] = ['dark', 'light', 'system']

function readPref(): ThemePreference {
  const raw = localStorage.getItem(STORAGE_KEY)
  return raw === 'dark' || raw === 'light' || raw === 'system' ? raw : 'system'
}

function resolve(pref: ThemePreference): ThemeResolved {
  if (pref === 'dark' || pref === 'light') return pref
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const [preference, setPreferenceState] = useState<ThemePreference>(readPref)
  const [resolved, setResolved] = useState<ThemeResolved>(() => resolve(readPref()))

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', resolved)
  }, [resolved])

  useEffect(() => {
    if (preference !== 'system') {
      setResolved(preference)
      return
    }
    const mq = matchMedia('(prefers-color-scheme: dark)')
    const update = (e: MediaQueryListEvent) => setResolved(e.matches ? 'dark' : 'light')
    setResolved(mq.matches ? 'dark' : 'light')
    mq.addEventListener('change', update)
    return () => mq.removeEventListener('change', update)
  }, [preference])

  const setPreference = useCallback((p: ThemePreference) => {
    localStorage.setItem(STORAGE_KEY, p)
    setPreferenceState(p)
  }, [])

  const setNext = useCallback(() => {
    setPreferenceState(prev => {
      const next = CYCLE[(CYCLE.indexOf(prev) + 1) % CYCLE.length]
      localStorage.setItem(STORAGE_KEY, next)
      return next
    })
  }, [])

  return { preference, resolved, setPreference, setNext }
}
```

- [ ] **Step 2: Run the test**

```bash
cd web && pnpm test useTheme 2>&1 | tail -10
```

Expected: PASS, all 5 tests green.

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useTheme.tsx web/src/__tests__/useTheme.test.tsx
git commit -m "feat(web): useTheme hook with persistence and system listener"
```

---

## Task 9: Extend useSSEConnected with latency

**Files:**
- Modify: `web/src/hooks/useSSE.tsx`
- Modify: `web/src/components/AppShell.tsx` (the only existing caller — temporary; will be rewritten in Task 26)

- [ ] **Step 1: Extend the SSE provider to track lastEventTime**

Edit `web/src/hooks/useSSE.tsx`:

Change the imports at the top (line 1) to add `useCallback`:

```tsx
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
```

Change the `SSEContextValue` type (line 6-9) to:

```tsx
type SSEContextValue = {
  connected: boolean
  latencyMs: number | null
  subscribe: (types: string[] | null, fn: Subscriber) => () => void
}
```

In `SSEProvider` (line 13), add a `lastEventTimeRef` and a `latencyMs` state immediately after the existing `setConnected` state (around line 14):

```tsx
const [latencyMs, setLatencyMs] = useState<number | null>(null)
const lastEventTimeRef = useRef<number | null>(null)
```

Inside `es.onmessage` (around line 29), set the timestamp on every event arrival, immediately after `JSON.parse`:

```tsx
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
```

Inside the same `useEffect` (around line 49 after `open()`), add a 1-second polling interval before the cleanup `return`:

```tsx
const tick = setInterval(() => {
  if (lastEventTimeRef.current === null) {
    setLatencyMs(null)
  } else {
    setLatencyMs(Date.now() - lastEventTimeRef.current)
  }
}, 1000)

return () => { cancelled = true; es?.close(); clearInterval(tick) }
```

In the `value` object (around line 53), add `latencyMs`:

```tsx
const value: SSEContextValue = {
  connected,
  latencyMs,
  subscribe: useCallback((types, fn) => {
    const entry = { types, fn }
    subsRef.current.add(entry)
    return () => { subsRef.current.delete(entry) }
  }, []),
}
```

Replace the existing `useSSEConnected` (lines 74-77) with:

```tsx
export function useSSEStatus(): { connected: boolean; latencyMs: number | null } {
  const ctx = useContext(SSEContext)
  return { connected: ctx?.connected ?? false, latencyMs: ctx?.latencyMs ?? null }
}
```

- [ ] **Step 2: Update the lone caller in AppShell**

In `web/src/components/AppShell.tsx`, change line 2 from:

```tsx
import { useSSEConnected } from '../hooks/useSSE'
```

to:

```tsx
import { useSSEStatus } from '../hooks/useSSE'
```

And line 7 from:

```tsx
const connected = useSSEConnected()
```

to:

```tsx
const { connected } = useSSEStatus()
```

- [ ] **Step 3: Run the existing tests to verify nothing broke**

```bash
cd web && pnpm test 2>&1 | tail -20
```

Expected: all currently-passing tests still pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useSSE.tsx web/src/components/AppShell.tsx
git commit -m "feat(web): extend SSE hook to expose latencyMs from event timestamps"
```

---

## Task 10: Stat primitive with sparkline support

**Files:**
- Create: `web/src/components/primitives/Stat.tsx`
- Create: `web/src/__tests__/primitives/Stat.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/__tests__/primitives/Stat.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Stat } from '../../components/primitives/Stat'

describe('Stat', () => {
  it('renders label and value', () => {
    render(<Stat label="Turns 24h" value="1,284" />)
    expect(screen.getByText('Turns 24h')).toBeInTheDocument()
    expect(screen.getByText('1,284')).toBeInTheDocument()
  })

  it('renders delta with up arrow when positive', () => {
    render(<Stat label="Turns" value="1,284" delta={{ value: '12%', direction: 'up' }} />)
    const delta = screen.getByText(/12%/)
    expect(delta.textContent).toContain('↑')
  })

  it('renders delta with down arrow when negative', () => {
    render(<Stat label="Tokens" value="847k" delta={{ value: '3%', direction: 'down' }} />)
    expect(screen.getByText(/3%/).textContent).toContain('↓')
  })

  it('renders breakdown text when provided', () => {
    render(<Stat label="Bugs" value="3" breakdown="2 medium · 1 high" />)
    expect(screen.getByText('2 medium · 1 high')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test Stat 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement the primitive**

Create `web/src/components/primitives/Stat.tsx`:

```tsx
import type { ReactNode } from 'react'

export type StatDelta = { value: string; direction: 'up' | 'down' }

export function Stat(props: {
  label: string
  value: ReactNode
  delta?: StatDelta
  breakdown?: ReactNode
  sparkline?: ReactNode
}): JSX.Element {
  const { label, value, delta, breakdown, sparkline } = props
  return (
    <div
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: 'var(--radius-3)',
        padding: 'var(--space-3)',
        minWidth: 120,
      }}
    >
      <div
        style={{
          fontSize: 9,
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
          color: 'var(--text-muted)',
          fontWeight: 600,
          marginBottom: 6,
        }}
      >
        {label}
      </div>
      <div
        className="mono"
        style={{
          fontSize: 20,
          fontWeight: 500,
          color: 'var(--text-strong)',
        }}
      >
        {value}
        {delta && (
          <span
            className="mono"
            style={{
              fontSize: 10,
              color: delta.direction === 'up' ? 'var(--ok)' : 'var(--err)',
              marginLeft: 6,
            }}
          >
            {delta.direction === 'up' ? '↑' : '↓'}
            {delta.value}
          </span>
        )}
      </div>
      {breakdown && (
        <div
          className="mono"
          style={{
            fontSize: 9,
            color: 'var(--text-muted)',
            marginTop: 6,
          }}
        >
          {breakdown}
        </div>
      )}
      {sparkline && <div style={{ marginTop: 6, height: 18 }}>{sparkline}</div>}
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web && pnpm test Stat 2>&1 | tail -5
```

Expected: PASS, 4 green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/primitives/Stat.tsx web/src/__tests__/primitives/Stat.test.tsx
git commit -m "feat(web): Stat primitive with delta and optional sparkline"
```

---

## Task 11: Pill, Dot, Kbd primitives (label atoms)

**Files:**
- Create: `web/src/components/primitives/Pill.tsx`
- Create: `web/src/components/primitives/Dot.tsx`
- Create: `web/src/components/primitives/Kbd.tsx`
- Create: `web/src/__tests__/primitives/Pill.test.tsx`

- [ ] **Step 1: Write the test (only Pill — Dot and Kbd are too trivial to warrant individual tests)**

Create `web/src/__tests__/primitives/Pill.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Pill } from '../../components/primitives/Pill'

describe('Pill', () => {
  it('renders children', () => {
    render(<Pill variant="write">WRITE</Pill>)
    expect(screen.getByText('WRITE')).toBeInTheDocument()
  })

  it.each(['write', 'read', 'warn', 'err', 'info', 'neutral'] as const)(
    'applies %s variant color',
    (variant) => {
      render(<Pill variant={variant}>{variant}</Pill>)
      const el = screen.getByText(variant)
      expect(el.style.color).toMatch(/var\(--/)
    }
  )

  it('renders solid variant with accent background', () => {
    render(<Pill variant="solid">LIVE</Pill>)
    expect(screen.getByText('LIVE').style.background).toContain('var(--accent)')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test Pill 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement Pill**

Create `web/src/components/primitives/Pill.tsx`:

```tsx
import type { ReactNode } from 'react'

export type PillVariant = 'write' | 'read' | 'warn' | 'err' | 'info' | 'neutral' | 'solid'

const COLOR: Record<Exclude<PillVariant, 'solid'>, string> = {
  write: 'var(--accent)',
  read: 'var(--ok)',
  warn: 'var(--warn)',
  err: 'var(--err)',
  info: 'var(--info)',
  neutral: 'var(--neutral)',
}

export function Pill(props: { variant: PillVariant; children: ReactNode }): JSX.Element {
  const base = {
    display: 'inline-block',
    padding: '1px 6px',
    borderRadius: 'var(--radius-1)',
    fontFamily: 'var(--font-mono)',
    fontSize: 9,
    fontWeight: 500 as const,
    lineHeight: 1.4,
  }
  if (props.variant === 'solid') {
    return (
      <span style={{ ...base, background: 'var(--accent)', color: 'var(--bg-base)', fontWeight: 600 }}>
        {props.children}
      </span>
    )
  }
  const c = COLOR[props.variant]
  return (
    <span
      style={{
        ...base,
        color: c,
        background: `color-mix(in srgb, ${c} 15%, transparent)`,
      }}
    >
      {props.children}
    </span>
  )
}
```

- [ ] **Step 4: Implement Dot**

Create `web/src/components/primitives/Dot.tsx`:

```tsx
export type DotStatus = 'ok' | 'warn' | 'err' | 'offline'

const COLOR: Record<DotStatus, string> = {
  ok: 'var(--ok)',
  warn: 'var(--warn)',
  err: 'var(--err)',
  offline: 'var(--text-faint)',
}

export function Dot({ status }: { status: DotStatus }): JSX.Element {
  const color = COLOR[status]
  const isOk = status === 'ok'
  return (
    <span
      style={{
        display: 'inline-block',
        width: 6,
        height: 6,
        borderRadius: '50%',
        background: color,
        boxShadow: isOk ? `0 0 6px color-mix(in srgb, ${color} 50%, transparent)` : 'none',
        animation: isOk ? 'breathe 2s ease-in-out infinite' : 'none',
        verticalAlign: 'middle',
      }}
    />
  )
}
```

- [ ] **Step 5: Implement Kbd**

Create `web/src/components/primitives/Kbd.tsx`:

```tsx
import type { ReactNode } from 'react'

export function Kbd({ children }: { children: ReactNode }): JSX.Element {
  return (
    <kbd
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: 9,
        padding: '1px 5px',
        borderRadius: 'var(--radius-1)',
        background: 'var(--bg-base)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-muted)',
      }}
    >
      {children}
    </kbd>
  )
}
```

- [ ] **Step 6: Run tests**

```bash
cd web && pnpm test Pill 2>&1 | tail -5
```

Expected: PASS, all variants green.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/primitives/Pill.tsx web/src/components/primitives/Dot.tsx web/src/components/primitives/Kbd.tsx web/src/__tests__/primitives/Pill.test.tsx
git commit -m "feat(web): Pill / Dot / Kbd label primitives"
```

---

## Task 12: Button primitive

**Files:**
- Create: `web/src/components/primitives/Button.tsx`
- Create: `web/src/__tests__/primitives/Button.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/__tests__/primitives/Button.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '../../components/primitives/Button'

describe('Button', () => {
  it('renders label and fires onClick', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Save</Button>)
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('respects disabled state', async () => {
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>X</Button>)
    await userEvent.click(screen.getByRole('button'))
    expect(onClick).not.toHaveBeenCalled()
  })

  it.each(['default', 'primary', 'danger', 'ghost'] as const)(
    'renders %s variant',
    (variant) => {
      render(<Button variant={variant}>btn</Button>)
      expect(screen.getByRole('button', { name: 'btn' })).toBeInTheDocument()
    }
  )
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test Button 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement the primitive**

Create `web/src/components/primitives/Button.tsx`:

```tsx
import type { ButtonHTMLAttributes, ReactNode } from 'react'

export type ButtonVariant = 'default' | 'primary' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'icon'

type Props = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> & {
  variant?: ButtonVariant
  size?: ButtonSize
  children: ReactNode
}

const VARIANT_STYLE: Record<ButtonVariant, React.CSSProperties> = {
  default: {
    background: 'var(--bg-base)',
    color: 'var(--text-body)',
    border: '1px solid var(--border-default)',
  },
  primary: {
    background: 'var(--accent)',
    color: 'var(--bg-base)',
    border: '1px solid var(--accent)',
    fontWeight: 500,
  },
  danger: {
    background: 'transparent',
    color: 'var(--err)',
    border: '1px solid color-mix(in srgb, var(--err) 30%, transparent)',
  },
  ghost: {
    background: 'transparent',
    color: 'var(--text-muted)',
    border: '1px solid transparent',
  },
}

const SIZE_STYLE: Record<ButtonSize, React.CSSProperties> = {
  md: { padding: '5px 12px', fontSize: 11 },
  sm: { padding: '2px 8px', fontSize: 10 },
  icon: { width: 26, height: 26, padding: 0, fontSize: 13 },
}

export function Button({ variant = 'default', size = 'md', style, ...rest }: Props): JSX.Element {
  return (
    <button
      {...rest}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 6,
        borderRadius: 'var(--radius-2)',
        fontFamily: 'var(--font-sans)',
        cursor: rest.disabled ? 'not-allowed' : 'pointer',
        opacity: rest.disabled ? 0.5 : 1,
        ...VARIANT_STYLE[variant],
        ...SIZE_STYLE[size],
        ...style,
      }}
    />
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web && pnpm test Button 2>&1 | tail -5
```

Expected: PASS, 6 green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/primitives/Button.tsx web/src/__tests__/primitives/Button.test.tsx
git commit -m "feat(web): Button primitive with 4 variants and 3 sizes"
```

---

## Task 13: Input primitive

**Files:**
- Create: `web/src/components/primitives/Input.tsx`

- [ ] **Step 1: Implement (no test — pass-through wrapper)**

Create `web/src/components/primitives/Input.tsx`:

```tsx
import { forwardRef, type InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement> & { mono?: boolean }

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { mono, style, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      {...rest}
      style={{
        padding: '5px 10px',
        borderRadius: 'var(--radius-2)',
        border: '1px solid var(--border-default)',
        background: 'var(--bg-base)',
        color: 'var(--text-body)',
        fontSize: 11,
        fontFamily: mono ? 'var(--font-mono)' : 'var(--font-sans)',
        outline: 'none',
        ...style,
      }}
      onFocus={(e) => {
        e.currentTarget.style.borderColor = 'var(--accent)'
        e.currentTarget.style.boxShadow = '0 0 0 2px color-mix(in srgb, var(--accent) 15%, transparent)'
        rest.onFocus?.(e)
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = 'var(--border-default)'
        e.currentTarget.style.boxShadow = 'none'
        rest.onBlur?.(e)
      }}
    />
  )
})
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/primitives/Input.tsx
git commit -m "feat(web): Input primitive with optional mono variant and focus ring"
```

---

## Task 14: Table primitive

**Files:**
- Create: `web/src/components/primitives/Table.tsx`
- Create: `web/src/__tests__/primitives/Table.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/__tests__/primitives/Table.test.tsx`:

```tsx
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test Table 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement the primitive**

Create `web/src/components/primitives/Table.tsx`:

```tsx
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web && pnpm test Table 2>&1 | tail -5
```

Expected: PASS, 3 green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/primitives/Table.tsx web/src/__tests__/primitives/Table.test.tsx
git commit -m "feat(web): Table primitive with typed columns and empty slot"
```

---

## Task 15: Empty + Skeleton primitives

**Files:**
- Create: `web/src/components/primitives/Empty.tsx`
- Create: `web/src/components/primitives/Skeleton.tsx`

- [ ] **Step 1: Implement Empty**

Create `web/src/components/primitives/Empty.tsx`:

```tsx
import type { ReactNode } from 'react'

export function Empty(props: { icon?: ReactNode; title: ReactNode; hint?: ReactNode }): JSX.Element {
  return (
    <div
      style={{
        textAlign: 'center',
        padding: '32px 16px',
        color: 'var(--text-muted)',
        fontSize: 11,
      }}
    >
      {props.icon !== undefined && (
        <div style={{ fontSize: 24, opacity: 0.3, marginBottom: 8 }}>{props.icon}</div>
      )}
      <div>{props.title}</div>
      {props.hint && (
        <div style={{ fontSize: 10, marginTop: 6, color: 'var(--text-faint)' }}>{props.hint}</div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Implement Skeleton**

Create `web/src/components/primitives/Skeleton.tsx`:

```tsx
export function Skeleton({ rows = 3 }: { rows?: number }): JSX.Element {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          style={{
            height: 28,
            borderRadius: 'var(--radius-2)',
            background:
              'linear-gradient(90deg, var(--bg-raised) 0%, var(--bg-surface) 50%, var(--bg-raised) 100%)',
            backgroundSize: '200% 100%',
            animation: 'shimmer 1.5s linear infinite',
          }}
        />
      ))}
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/primitives/Empty.tsx web/src/components/primitives/Skeleton.tsx
git commit -m "feat(web): Empty and Skeleton primitives"
```

---

## Task 16: Toast primitive

**Files:**
- Create: `web/src/components/primitives/Toast.tsx`

- [ ] **Step 1: Implement (used standalone — caller manages state and dismissal)**

Create `web/src/components/primitives/Toast.tsx`:

```tsx
import type { ReactNode } from 'react'

export type ToastTone = 'ok' | 'err' | 'warn' | 'info'

const COLOR: Record<ToastTone, string> = {
  ok: 'var(--ok)',
  err: 'var(--err)',
  warn: 'var(--warn)',
  info: 'var(--info)',
}

export function Toast(props: { tone?: ToastTone; children: ReactNode }): JSX.Element {
  const tone = props.tone ?? 'ok'
  return (
    <div
      style={{
        padding: '10px 14px',
        borderRadius: 'var(--radius-3)',
        background: 'var(--bg-raised)',
        border: '1px solid var(--border-default)',
        borderLeft: `3px solid ${COLOR[tone]}`,
        fontSize: 11,
        color: 'var(--text-body)',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
      }}
    >
      {props.children}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/primitives/Toast.tsx
git commit -m "feat(web): Toast primitive with tone-coded left border"
```

---

## Task 17: Modal primitive

**Files:**
- Create: `web/src/components/primitives/Modal.tsx`
- Create: `web/src/__tests__/primitives/Modal.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/__tests__/primitives/Modal.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Modal } from '../../components/primitives/Modal'

describe('Modal', () => {
  it('renders title, body, and actions when open', () => {
    render(
      <Modal open onClose={() => {}} title="Confirm">
        <div>Are you sure?</div>
      </Modal>,
    )
    expect(screen.getByText('Confirm')).toBeInTheDocument()
    expect(screen.getByText('Are you sure?')).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    render(
      <Modal open={false} onClose={() => {}} title="Hidden">
        <div>body</div>
      </Modal>,
    )
    expect(screen.queryByText('Hidden')).not.toBeInTheDocument()
  })

  it('calls onClose when backdrop clicked', async () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="x">
        <div>body</div>
      </Modal>,
    )
    await userEvent.click(screen.getByTestId('modal-backdrop'))
    expect(onClose).toHaveBeenCalled()
  })

  it('does NOT call onClose when modal body clicked', async () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="x">
        <div>body</div>
      </Modal>,
    )
    await userEvent.click(screen.getByText('body'))
    expect(onClose).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test Modal 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement the primitive**

Create `web/src/components/primitives/Modal.tsx`:

```tsx
import type { ReactNode } from 'react'

type Props = {
  open: boolean
  onClose: () => void
  title: ReactNode
  children: ReactNode
  actions?: ReactNode
}

export function Modal({ open, onClose, title, children, actions }: Props): JSX.Element | null {
  if (!open) return null
  return (
    <div
      data-testid="modal-backdrop"
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border-strong)',
          borderRadius: 'var(--radius-4)',
          padding: 20,
          boxShadow: '0 20px 50px rgba(0,0,0,0.5)',
          maxWidth: 400,
          width: '90%',
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-strong)', marginBottom: 8 }}>
          {title}
        </div>
        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 14, lineHeight: 1.5 }}>
          {children}
        </div>
        {actions && (
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>{actions}</div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web && pnpm test Modal 2>&1 | tail -5
```

Expected: PASS, 4 green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/primitives/Modal.tsx web/src/__tests__/primitives/Modal.test.tsx
git commit -m "feat(web): Modal primitive with backdrop click-to-close"
```

---

## Task 18: Sparkline primitive

**Files:**
- Create: `web/src/components/primitives/Sparkline.tsx`
- Create: `web/src/__tests__/primitives/Sparkline.test.tsx`
- Delete: `web/src/components/Sparkline.tsx` (replaced by primitive)

- [ ] **Step 1: Inspect existing Sparkline for behavior to preserve**

```bash
cat web/src/components/Sparkline.tsx
```

Note any callers via grep:

```bash
grep -rn "from.*components/Sparkline" web/src/
```

Record callers — they'll need their import paths updated.

- [ ] **Step 2: Write the failing test**

Create `web/src/__tests__/primitives/Sparkline.test.tsx`:

```tsx
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Sparkline } from '../../components/primitives/Sparkline'

describe('Sparkline', () => {
  it('renders a polyline with one point per data value', () => {
    const { container } = render(<Sparkline data={[1, 2, 3, 4, 5]} />)
    const polyline = container.querySelector('polyline')
    expect(polyline).not.toBeNull()
    const points = polyline!.getAttribute('points')!.split(' ')
    expect(points).toHaveLength(5)
  })

  it('renders nothing when data is empty', () => {
    const { container } = render(<Sparkline data={[]} />)
    expect(container.querySelector('polyline')).toBeNull()
  })

  it('uses up color when last value > first value', () => {
    const { container } = render(<Sparkline data={[1, 5]} />)
    expect(container.querySelector('polyline')!.getAttribute('stroke')).toBe('var(--ok)')
  })

  it('uses down color when last value < first value', () => {
    const { container } = render(<Sparkline data={[5, 1]} />)
    expect(container.querySelector('polyline')!.getAttribute('stroke')).toBe('var(--err)')
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd web && pnpm test Sparkline 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 4: Implement the primitive**

Create `web/src/components/primitives/Sparkline.tsx`:

```tsx
type Props = {
  data: number[]
  height?: number
  stroke?: string  /* override; otherwise auto by trend */
}

export function Sparkline({ data, height = 18, stroke }: Props): JSX.Element | null {
  if (data.length === 0) return null
  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1
  const stepX = data.length === 1 ? 0 : 100 / (data.length - 1)
  const points = data
    .map((v, i) => `${(i * stepX).toFixed(2)},${(height - ((v - min) / range) * height).toFixed(2)}`)
    .join(' ')
  const trend = data[data.length - 1] - data[0]
  const auto = trend >= 0 ? 'var(--ok)' : 'var(--err)'
  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      style={{ width: '100%', height }}
    >
      <polyline fill="none" stroke={stroke ?? auto} strokeWidth="1" points={points} />
    </svg>
  )
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd web && pnpm test Sparkline 2>&1 | tail -5
```

Expected: PASS, 4 green.

- [ ] **Step 6: Update callers and remove old file**

For each caller found in Step 1, update the import path from `'../components/Sparkline'` to `'../components/primitives/Sparkline'`. Then:

```bash
rm web/src/components/Sparkline.tsx
cd web && pnpm test 2>&1 | tail -10
```

Expected: all tests still pass.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/primitives/Sparkline.tsx web/src/__tests__/primitives/Sparkline.test.tsx
git add -u web/src/components/Sparkline.tsx
git add web/src/  # any caller updates
git commit -m "feat(web): Sparkline primitive replacing standalone component"
```

---

## Task 19: Primitives barrel export

**Files:**
- Create: `web/src/components/primitives/index.ts`

- [ ] **Step 1: Create the barrel**

Create `web/src/components/primitives/index.ts`:

```ts
export { Stat, type StatDelta } from './Stat'
export { Pill, type PillVariant } from './Pill'
export { Dot, type DotStatus } from './Dot'
export { Kbd } from './Kbd'
export { Button, type ButtonVariant, type ButtonSize } from './Button'
export { Input } from './Input'
export { Table, type Column } from './Table'
export { Empty } from './Empty'
export { Skeleton } from './Skeleton'
export { Toast, type ToastTone } from './Toast'
export { Modal } from './Modal'
export { Sparkline } from './Sparkline'
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/primitives/index.ts
git commit -m "feat(web): primitives barrel export"
```

---

## Task 20: PageHead component

**Files:**
- Create: `web/src/components/PageHead.tsx`
- Create: `web/src/__tests__/PageHead.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/__tests__/PageHead.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHead } from '../components/PageHead'

describe('PageHead', () => {
  it('renders title only', () => {
    render(<PageHead title="Overview" />)
    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  it('renders title + meta', () => {
    render(<PageHead title="Activity" meta="window=24h" />)
    expect(screen.getByText('window=24h')).toBeInTheDocument()
  })

  it('renders title + actions', () => {
    render(<PageHead title="X" actions={<button>Scan</button>} />)
    expect(screen.getByRole('button', { name: 'Scan' })).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test PageHead 2>&1 | tail -5
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement**

Create `web/src/components/PageHead.tsx`:

```tsx
import type { ReactNode } from 'react'

export function PageHead(props: { title: ReactNode; meta?: ReactNode; actions?: ReactNode }): JSX.Element {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 4 }}>
      <h1
        style={{
          fontSize: 22,
          fontWeight: 600,
          color: 'var(--text-strong)',
          letterSpacing: '-0.01em',
          margin: 0,
        }}
      >
        {props.title}
      </h1>
      {props.meta && (
        <div className="mono" style={{ fontSize: 11, color: 'var(--text-muted)' }}>
          {props.meta}
        </div>
      )}
      {props.actions && (
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>{props.actions}</div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web && pnpm test PageHead 2>&1 | tail -5
```

Expected: PASS, 3 green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/PageHead.tsx web/src/__tests__/PageHead.test.tsx
git commit -m "feat(web): PageHead component (title + meta + right-aligned actions)"
```

---

## Task 21: Sidebar component

**Files:**
- Create: `web/src/components/Sidebar.tsx`

- [ ] **Step 1: Implement Sidebar**

Create `web/src/components/Sidebar.tsx`:

```tsx
import { Link, NavLink } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useActiveProject } from '../hooks/useActiveProject'
import { ProjectPicker } from './ProjectPicker'

type NavItem = { to: string; label: string; icon?: string; badge?: ReactNode }

export function Sidebar(): JSX.Element {
  const { active } = useActiveProject()
  const q = active ? `?project=${encodeURIComponent(active)}` : ''

  const groups: { label: string; items: NavItem[] }[] = [
    {
      label: 'GLOBAL',
      items: [
        { to: '/', label: 'Overview', icon: '▣' },
        { to: '/activity', label: 'Activity', icon: '≣' },
        { to: '/cron', label: 'Cron', icon: '⏱' },
      ],
    },
    {
      label: 'PROJECT',
      items: [
        { to: '/cerebrum' + q, label: 'Cerebrum', icon: '⊕' },
        { to: '/memory', label: 'Memory', icon: '▭' },
        { to: '/anatomy' + q, label: 'Anatomy', icon: '⌘' },
        { to: '/buglog' + q, label: 'BugLog', icon: '!' },
        { to: '/suggestions' + q, label: 'Suggestions', icon: '∴' },
      ],
    },
    {
      label: 'TOOLS',
      items: [
        { to: '/token' + q, label: 'Token', icon: '◐' },
        { to: '/designqc' + q, label: 'DesignQC', icon: '◭' },
      ],
    },
  ]

  return (
    <aside
      style={{
        width: 'var(--sidebar-width)',
        background: 'var(--bg-base)',
        borderRight: '1px solid var(--border-default)',
        padding: '14px 0',
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        minHeight: '100vh',
      }}
    >
      <div
        style={{
          padding: '0 14px 14px',
          fontWeight: 600,
          color: 'var(--text-strong)',
          fontSize: 13,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          borderBottom: '1px solid var(--border-default)',
          marginBottom: 8,
        }}
      >
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'inherit', textDecoration: 'none' }}>
          <span
            style={{
              width: 14,
              height: 14,
              background: 'linear-gradient(135deg, var(--accent), color-mix(in srgb, var(--accent) 70%, black))',
              borderRadius: 3,
              display: 'inline-block',
            }}
          />
          mneme
        </Link>
      </div>
      <div style={{ padding: '0 14px 8px' }}>
        <ProjectPicker />
      </div>
      {groups.map((g) => (
        <div key={g.label}>
          <div
            style={{
              fontSize: 9,
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              color: 'var(--text-faint)',
              fontWeight: 600,
              padding: '12px 14px 4px',
            }}
          >
            {g.label}
          </div>
          {g.items.map((it) => (
            <NavLink
              key={it.to}
              to={it.to}
              end={it.to === '/'}
              style={({ isActive }) => ({
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '5px 14px',
                paddingLeft: isActive ? 12 : 14,
                color: isActive ? 'var(--accent)' : 'var(--text-muted)',
                background: isActive ? 'var(--bg-selected)' : 'transparent',
                borderLeft: isActive ? '2px solid var(--accent)' : 'none',
                fontSize: 11,
                textDecoration: 'none',
              })}
            >
              {it.icon && (
                <span className="mono" style={{ width: 12, opacity: 0.7, fontSize: 10 }}>
                  {it.icon}
                </span>
              )}
              {it.label}
              {it.badge && <span style={{ marginLeft: 'auto' }}>{it.badge}</span>}
            </NavLink>
          ))}
        </div>
      ))}
    </aside>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Sidebar.tsx
git commit -m "feat(web): Sidebar component with three-group nav and ProjectPicker"
```

---

## Task 22: TopBar component

**Files:**
- Create: `web/src/components/TopBar.tsx`

- [ ] **Step 1: Implement TopBar**

Create `web/src/components/TopBar.tsx`:

```tsx
import { useState } from 'react'
import { useSSEStatus } from '../hooks/useSSE'
import { useTheme } from '../hooks/useTheme'
import { Modal } from './primitives/Modal'
import { Button } from './primitives/Button'
import { Kbd } from './primitives/Kbd'
import { Dot } from './primitives/Dot'

export function TopBar(): JSX.Element {
  const { connected, latencyMs } = useSSEStatus()
  const { preference, setNext } = useTheme()
  const [searchOpen, setSearchOpen] = useState(false)

  const status = !connected ? 'offline' : latencyMs !== null && latencyMs > 30_000 ? 'err' : latencyMs !== null && latencyMs > 5_000 ? 'warn' : 'ok'
  const label = !connected ? 'SSE · offline' : latencyMs === null ? 'SSE · waiting' : `SSE · ${latencyMs}ms`
  const labelColor = status === 'err' || status === 'offline' ? 'var(--err)' : status === 'warn' ? 'var(--warn)' : 'var(--text-muted)'

  return (
    <header
      style={{
        height: 'var(--header-height)',
        borderBottom: '1px solid var(--border-default)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 14px',
        gap: 14,
        background: 'var(--bg-surface)',
      }}
    >
      <button
        onClick={() => setSearchOpen(true)}
        style={{
          flex: 1,
          maxWidth: 300,
          padding: '5px 10px',
          borderRadius: 'var(--radius-2)',
          border: '1px solid var(--border-default)',
          background: 'var(--bg-base)',
          fontSize: 11,
          color: 'var(--text-muted)',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          cursor: 'pointer',
          textAlign: 'left',
        }}
      >
        ⌕ Search files, rules, bugs…
        <span style={{ marginLeft: 'auto' }}>
          <Kbd>⌘K</Kbd>
        </span>
      </button>

      <div style={{ flex: 1 }} />

      <div className="mono" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 10, color: labelColor }}>
        <Dot status={status === 'offline' ? 'offline' : status === 'err' ? 'err' : status === 'warn' ? 'warn' : 'ok'} />
        {label}
      </div>

      <button
        onClick={setNext}
        title={`Theme: ${preference}`}
        style={{
          width: 26,
          height: 26,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: 'var(--radius-2)',
          color: 'var(--text-muted)',
          background: 'transparent',
          border: 'none',
          cursor: 'pointer',
          fontSize: 13,
        }}
      >
        ◐
      </button>

      <Modal
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        title="Search"
        actions={<Button onClick={() => setSearchOpen(false)}>Close</Button>}
      >
        Search not yet implemented — coming in a follow-up spec.
      </Modal>
    </header>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/TopBar.tsx
git commit -m "feat(web): TopBar with SSE status, theme toggle, and ⌘K placeholder modal"
```

---

## Task 23: Rewrite AppShell

**Files:**
- Modify: `web/src/components/AppShell.tsx` (full rewrite)

- [ ] **Step 1: Replace AppShell.tsx contents**

Replace `web/src/components/AppShell.tsx` entirely with:

```tsx
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'

export function AppShell(): JSX.Element {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'var(--sidebar-width) 1fr',
        gridTemplateRows: 'var(--header-height) 1fr',
        minHeight: '100vh',
      }}
    >
      <div style={{ gridColumn: 1, gridRow: '1 / 3' }}>
        <Sidebar />
      </div>
      <div style={{ gridColumn: 2, gridRow: 1 }}>
        <TopBar />
      </div>
      <main
        style={{
          gridColumn: 2,
          gridRow: 2,
          padding: 'var(--panel-pad)',
          overflow: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
        }}
      >
        <Outlet />
      </main>
    </div>
  )
}
```

- [ ] **Step 2: Run all tests**

```bash
cd web && pnpm test 2>&1 | tail -15
```

Expected: all currently-passing tests still pass. Existing AppShell tests in `__tests__/App.test.tsx` may need selector updates — fix any that fail.

- [ ] **Step 3: Smoke test in dev**

```bash
cd web && pnpm dev &
sleep 3
curl -s http://localhost:5173 | grep -q '<div id="root">' && echo "ok"
kill %1 2>/dev/null; wait 2>/dev/null
```

Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/AppShell.tsx web/src/__tests__/  # any test fixes
git commit -m "feat(web): rewrite AppShell as grid with Sidebar + TopBar"
```

---

## Task 24: Migrate Overview panel

**Files:**
- Modify: `web/src/panels/Overview.tsx` (full rewrite)
- Delete: `web/src/components/HealthCard.tsx` (functionality folded into Overview using Stat + Dot)
- Modify: `web/src/components/ProjectCard.tsx` (restyle inline using tokens)

- [ ] **Step 1: Inspect existing components for behavior to preserve**

```bash
cat web/src/panels/Overview.tsx
cat web/src/components/HealthCard.tsx
cat web/src/components/ProjectCard.tsx
```

Record what fields `getOverview()` returns (`data.totals`, `data.projects`) and what `HealthCard` displays. The new Overview will replace `<HealthCard />` with one or more `<Stat>` cards using the same data, and `<ProjectCard>` will be restyled to use token CSS variables instead of utility classes.

- [ ] **Step 2: Restyle ProjectCard with tokens**

Edit `web/src/components/ProjectCard.tsx` — replace any Tailwind utility classes (`bg-white`, `text-gray-500`, `rounded`, `border`, `p-3`, etc.) with inline style objects referencing tokens. The exact replacement depends on current contents; example pattern:

```tsx
// Before:
<div className="rounded border p-3 bg-white">…</div>
// After:
<div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-3)', padding: 'var(--space-3)' }}>…</div>
```

- [ ] **Step 3: Rewrite Overview**

Replace `web/src/panels/Overview.tsx` entirely with:

```tsx
import { ProjectCard } from '../components/ProjectCard'
import { PageHead } from '../components/PageHead'
import { Stat, Empty, Skeleton } from '../components/primitives'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getOverview } from '../api/overview'

export function Overview(): JSX.Element {
  const { data, error, refetch } = useFetch(getOverview, { intervalMs: 10_000 })
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => refetch())

  if (error) {
    return (
      <>
        <PageHead title="Overview" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Overview" />
        <Skeleton rows={4} />
      </>
    )
  }

  const t = data.totals
  return (
    <>
      <PageHead title="Overview" meta={`${data.projects.length} project${data.projects.length === 1 ? '' : 's'}`} />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
        <Stat label="Projects" value={String(t.projects)} />
        <Stat label="Anatomy files" value={String(t.anatomy_files)} />
        <Stat label="Cerebrum pending" value={String(t.cerebrum_pending)} />
        <Stat label="Open suggestions" value={String(t.open_suggestions)} />
      </div>
      <div>
        <h2 style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-strong)', margin: '12px 0 8px' }}>
          Projects
        </h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 10 }}>
          {data.projects.map((p) => <ProjectCard key={p.id} p={p} />)}
        </div>
      </div>
    </>
  )
}
```

- [ ] **Step 4: Delete HealthCard**

```bash
rm web/src/components/HealthCard.tsx
grep -rn "HealthCard" web/src/  # should return no matches
```

- [ ] **Step 5: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

Expected: tests pass, build succeeds.

- [ ] **Step 6: Commit**

```bash
git add web/src/panels/Overview.tsx web/src/components/ProjectCard.tsx
git add -u web/src/components/HealthCard.tsx
git commit -m "feat(web): migrate Overview to PageHead + Stat primitives"
```

---

## Task 25: Migrate Activity panel

**Files:**
- Modify: `web/src/panels/Activity.tsx`
- Modify: `web/src/components/EventRow.tsx` (restyle using tokens)

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Activity.tsx
cat web/src/components/EventRow.tsx
```

The Activity panel renders an event feed. Key replacements: wrap in `<PageHead>`, use `<Table>` with custom render functions, use `<Pill>` for event types, use `<Dot>` for status.

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Activity" meta="…" />` (use latest event timestamp or count as meta).
2. Replace ad-hoc rendering with `<Table>` from `components/primitives`.
3. Each event type column uses `<Pill variant="write|read|warn|err|info|neutral">`. Map event type strings to variants — if Activity has its own event-type list, define a small `typeToVariant()` mapping inline.
4. Replace any utility color classes with `style={{ color: 'var(--text-muted)' }}` and similar.
5. Empty state: `<Empty title="No activity yet" hint="Events appear as Claude Code makes tool calls." />`.
6. Loading state: `<Skeleton rows={6} />`.

EventRow.tsx: similar utility-class → token-style replacement as in Task 24 Step 2 for ProjectCard.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Activity.tsx web/src/components/EventRow.tsx
git commit -m "feat(web): migrate Activity to PageHead + Table + Pill primitives"
```

---

## Task 26: Migrate BugLog panel

**Files:**
- Modify: `web/src/panels/BugLog.tsx`
- Modify: `web/src/components/ConfirmButton.tsx` (replace internal button styling with `<Button>` primitive variants)

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/BugLog.tsx
cat web/src/components/ConfirmButton.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="BugLog" meta={\`${bugs.length} entries\`} actions={<ConfirmButton ...>Delete all</ConfirmButton>} />`.
2. Replace expandable row HTML with `<Table>` plus a row-state hook for which row is expanded.
3. Rewrite `ConfirmButton` internals to compose `<Button variant="danger">` and `<Modal>` from primitives instead of any inline button/dialog HTML it currently contains.
4. Empty state: `<Empty icon="∅" title="No bugs logged yet" hint={<>Run <Kbd>mneme buglog add</Kbd> to record one.</>} />`.
5. Loading state: `<Skeleton rows={6} />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/BugLog.tsx web/src/components/ConfirmButton.tsx
git commit -m "feat(web): migrate BugLog to primitives, ConfirmButton uses Modal+Button"
```

---

## Task 27: Migrate Cerebrum panel

**Files:**
- Modify: `web/src/panels/Cerebrum.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Cerebrum.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Cerebrum" meta={\`${rules.length} rules\`} actions={<Button variant="primary" onClick={addRule}>Add rule</Button>} />`.
2. Render rules as `<Table>` with columns: Pattern (mono), Message, Comment, Actions (`<Button size="sm" variant="danger">Remove</Button>`).
3. Form for adding a rule: `<Input mono placeholder="pattern: pkg/dashboard/**" />`, `<Input placeholder="message" />`, `<Input placeholder="comment (optional)" />`, `<Button variant="primary">Add</Button>`. Wrap in a `<div>` with `style={{ display: 'flex', gap: 8 }}`.
4. Empty state: `<Empty title="No rules yet" hint="Cerebrum rules guide Claude Code's behavior on this project." />`.
5. Loading state: `<Skeleton rows={4} />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Cerebrum.tsx
git commit -m "feat(web): migrate Cerebrum to primitives"
```

---

## Task 28: Migrate Memory panel

**Files:**
- Modify: `web/src/panels/Memory.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Memory.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Memory" />`.
2. Replace ad-hoc cards/lists with appropriate primitives — typically a `<Table>` with mono content cells for memory entries.
3. Empty state and loading state as in earlier tasks.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Memory.tsx
git commit -m "feat(web): migrate Memory to primitives"
```

---

## Task 29: Migrate Anatomy panel

**Files:**
- Modify: `web/src/panels/Anatomy.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Anatomy.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Anatomy" meta={\`${files.length} files\`} actions={<Button variant="primary" onClick={rescan}>Rescan</Button>} />`.
2. Filename filter: `<Input placeholder="Filter files…" />`.
3. Tree/list of files: keep existing collapsible logic, but restyle the wrapper div with `style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--panel-pad)' }}`. Each file path uses `className="mono"` and `style={{ color: 'var(--text-body)' }}`.
4. Token counts use `<Pill variant="neutral">~N tok</Pill>`.
5. Empty state: `<Empty icon="∅" title="No anatomy map" hint={<>Run <Kbd>mneme scan</Kbd> to generate one.</>} />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Anatomy.tsx
git commit -m "feat(web): migrate Anatomy to primitives with token-styled tree"
```

---

## Task 30: Migrate Suggestions panel

**Files:**
- Modify: `web/src/panels/Suggestions.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Suggestions.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Suggestions" meta={\`${suggestions.length} open\`} />`.
2. Render suggestions as a `<Table>` with columns: When (`mono` time), Type (`<Pill>`), Message, Action (`<Button size="sm">Dismiss</Button>`).
3. Empty state: `<Empty icon="∴" title="No suggestions" hint="mneme will surface optimization candidates here." />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Suggestions.tsx
git commit -m "feat(web): migrate Suggestions to primitives"
```

---

## Task 31: Migrate Cron panel

**Files:**
- Modify: `web/src/panels/Cron.tsx`
- Modify: `web/src/components/CronTaskRow.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Cron.tsx
cat web/src/components/CronTaskRow.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Cron" meta={\`${tasks.length} tasks · next at ${nextRunTime}\`} />`.
2. Render tasks as `<Table>`. CronTaskRow restyling: replace utility classes with token-style inline styles like in Task 24 Step 2.
3. Status indicators: `<Dot status="ok|warn|err|offline" />` next to task name.
4. Empty state: `<Empty icon="⏱" title="No cron tasks scheduled" />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Cron.tsx web/src/components/CronTaskRow.tsx
git commit -m "feat(web): migrate Cron to primitives with Dot status indicators"
```

---

## Task 32: Migrate Token panel

**Files:**
- Modify: `web/src/panels/Token.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/Token.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="Token" meta={\`${total} total · ${windowDesc}\`} />`.
2. Top: `<Stat>` cards for total, window-summary numbers (with `<Sparkline>` if time-series data exists).
3. Breakdown table: `<Table>` with mono numeric columns.
4. Loading state: `<Skeleton rows={3} />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/Token.tsx
git commit -m "feat(web): migrate Token to Stat + Sparkline + Table primitives"
```

---

## Task 33: Migrate DesignQC panel

**Files:**
- Modify: `web/src/panels/DesignQC.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/panels/DesignQC.tsx
```

- [ ] **Step 2: Apply migration pattern**

Apply the canonical pattern from Task 24:

1. Wrap in `<PageHead title="DesignQC" meta={\`${routes.length} routes\`} actions={<Button variant="primary" onClick={runDesignQC}>Run capture</Button>} />`.
2. Route grid: keep grid layout but restyle each cell with token CSS variables for background, border, radius.
3. Click-to-expand overlay: rewrite using `<Modal>` primitive instead of any inline overlay HTML.
4. Empty state: `<Empty icon="◭" title="No design report yet" hint={<>Run <Kbd>mneme designqc</Kbd> first.</>} />`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/panels/DesignQC.tsx
git commit -m "feat(web): migrate DesignQC to primitives, overlay uses Modal"
```

---

## Task 34: Restyle ProjectPicker

**Files:**
- Modify: `web/src/components/ProjectPicker.tsx`

- [ ] **Step 1: Inspect existing**

```bash
cat web/src/components/ProjectPicker.tsx
```

- [ ] **Step 2: Restyle**

Replace utility classes with token-styled inline styles. The picker should render as a pill-button matching the spec Section 4.2 #1: breadcrumb-style label `~/workspace / mneme` with `▾` caret. Use `style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11, color: 'var(--text-body)', padding: '5px 8px', borderRadius: 'var(--radius-2)', border: '1px solid var(--border-default)', background: 'var(--bg-base)', cursor: 'pointer' }}`.

If the picker opens a dropdown, style the dropdown with `style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-strong)', borderRadius: 'var(--radius-3)', boxShadow: '0 8px 20px rgba(0,0,0,0.3)' }}`.

- [ ] **Step 3: Run tests + smoke**

```bash
cd web && pnpm test ProjectPicker 2>&1 | tail -10
cd web && pnpm build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ProjectPicker.tsx
git commit -m "feat(web): restyle ProjectPicker as token-driven pill button"
```

---

## Task 35: Build, install, and end-to-end smoke

**Files:**
- (no source changes — verification only)

- [ ] **Step 1: Full build**

```bash
make web-build 2>&1 | tail -10
```

Expected: `web/dist/` and `pkg/dashboard/dist/` both populated; build succeeds.

- [ ] **Step 2: Rebuild Go binary (embeds new dist)**

```bash
go build -o bin/mneme ./cmd && cp bin/mneme ~/.local/bin/mneme
```

Expected: binary built and installed.

- [ ] **Step 3: Restart daemon and open dashboard**

```bash
mneme daemon restart
sleep 2
mneme dashboard --no-open
```

Take note of the URL printed.

- [ ] **Step 4: Manual smoke checklist**

Open the URL in a browser. Verify:

- [ ] Page renders with no console errors
- [ ] No first-paint flash on reload (try both `Cmd+R` and a fresh `Cmd+Shift+R`)
- [ ] All 10 sidebar items navigate correctly (`Overview`, `Activity`, `Cron`, `Cerebrum`, `Memory`, `Anatomy`, `BugLog`, `Suggestions`, `Token`, `DesignQC`)
- [ ] Sidebar shows three section labels: `GLOBAL`, `PROJECT`, `TOOLS`
- [ ] Active nav item shows accent left border
- [ ] Top bar SSE indicator: dot breathes, latency updates
- [ ] `⌘K` button opens modal with placeholder text and Close button works
- [ ] Theme toggle (◐) cycles dark → light → system; preference persists across reload
- [ ] In `system` mode, changing OS theme updates dashboard live without reload (test by toggling macOS appearance)
- [ ] All panels load without console errors in both themes

- [ ] **Step 5: Run all unit tests**

```bash
cd web && pnpm test 2>&1 | tail -15
```

Expected: all tests pass.

- [ ] **Step 6: Bundle size check**

```bash
du -sh web/dist/assets/
```

Expected: `<dist size>` should be within 50kB of pre-redesign. The added font packages dominate any growth — fonts subset to only loaded weights via `@fontsource` should keep this manageable. If size increased significantly (>200kB), revisit Task 1 to see if any unused weights were imported.

- [ ] **Step 7: Commit any test-selector fixes accumulated through migration**

```bash
git status
git diff
# stage and commit any remaining test fixes
git add -A
git commit -m "test(web): update selectors after dashboard primitive migration" || echo "nothing to commit"
```

---

## Task 36: Update CLAUDE.md if behavior changed

**Files:**
- Modify (maybe): `CLAUDE.md`

- [ ] **Step 1: Decide if CLAUDE.md needs updating**

Review the **Architecture** section of `CLAUDE.md`. If any of the following are now stale, update them:

- Mentions of `web/src/components/HealthCard.tsx` (removed)
- Mentions of `web/src/components/Sparkline.tsx` (moved to `primitives/`)
- Any reference to "default Tailwind utility classes" being the dashboard styling approach

If nothing is stale, **skip this task** — `CLAUDE.md` shouldn't be updated just because a feature shipped.

- [ ] **Step 2: Commit if changed**

```bash
git diff CLAUDE.md
git add CLAUDE.md && git commit -m "docs(claude): note dashboard token system and primitive library" || echo "no doc updates needed"
```

---

## Self-Review checklist

After implementing all tasks, verify against the spec:

| Spec section | Task(s) covering it |
|--------------|---------------------|
| §3.1 Dark color tokens | Task 2 |
| §3.2 Light color tokens | Task 2 |
| §3.3 Typography | Tasks 1, 4, 5, base.css in 3 |
| §3.4 Spacing scale | Task 2 |
| §3.5 Radii | Task 2 |
| §3.6 Density params | Task 2 |
| §4.1 Sidebar | Task 21 |
| §4.2 Top bar | Task 22 |
| §4.3 Main area / page head | Tasks 20, 23 |
| §5.1 Stat | Task 10 |
| §5.2 Pill | Task 11 |
| §5.3 Dot | Task 11 |
| §5.4 Button | Task 12 |
| §5.5 Input | Task 13 |
| §5.6 Table | Task 14 |
| §5.7 Empty | Task 15 |
| §5.8 Toast | Task 16 |
| §5.9 Modal | Task 17 |
| §5.10 Kbd | Task 11 |
| §6 Theme toggle | Tasks 6, 7, 8, 22 |
| §7 Motion (SSE breathe, skeleton, route fade, etc.) | Tasks 3, 9, 11, 15, 22 |
| §8.1 File structure | All file-create tasks |
| §8.2 Tailwind @theme | Task 5 |
| §8.3 Per-panel migration | Tasks 24-33 |
| §8.4 Backward compatibility | implicitly — no shims added |
| §9 Acceptance criteria | Task 35 |
| §10 Open questions / scope cuts | ⌘K placeholder modal in Task 22 |

All sections covered. No placeholders. Type names consistent (`useSSEStatus`, `ThemePreference`, `Column<T>`, `PillVariant`, `ButtonVariant`, `DotStatus` defined once and used consistently in later tasks).
