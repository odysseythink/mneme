# Dashboard redesign — Grafana/Datadog cockpit aesthetic

**Status:** Spec · awaiting implementation plan
**Date:** 2026-04-28
**Owner:** ranwei
**Scope:** `web/` (React 19 + Tailwind 4 SPA). No backend changes except where noted.

---

## 1. Why

The current dashboard renders default Tailwind utility classes with no design system: a flat 240px gray sidebar, a `max-w-2xl` body rule that conflicts with the flex shell, and ten nav items in a flat list. There is no visual hierarchy, no color system, no theme support, and inconsistent typography. User feedback: "no aesthetic at all."

This spec replaces the visual layer with a coherent system aligned to Grafana/Datadog cockpit aesthetic: dark-primary, data-dense, monospace for numbers/paths/time, restrained blue accent. Light theme is provided as an opt-in via the same tokens. Functional behavior of every panel is preserved — only the look changes (plus minor IA cleanup in the sidebar).

## 2. Scope

**In scope (this spec):**
- Design tokens (color × 2 themes, typography, spacing, radii, density params)
- App shell (sidebar, top bar, main area)
- Component primitives shared across panels (stat / pill / dot / button / input / table / empty / toast / modal)
- Theme toggle (dark / light / system) with persistence and first-paint flash prevention
- Motion behavior for SSE indicator, stat updates, new feed rows, sparklines, route changes, loading states
- Sidebar IA: split flat list into three sections (`GLOBAL` / `PROJECT` / `TOOLS`)
- Top bar: project picker, search affordance (`⌘K`), SSE indicator with latency, theme toggle, settings
- Footer in sidebar: build SHA + uptime

**Out of scope (deferred or rejected):**
- Functional backend search behind `⌘K` — affordance is shown in this spec but the search endpoint and results UI are deferred to a separate spec. The `⌘K` button opens an empty modal with "Search not yet implemented."
- Per-panel content redesigns — each existing panel adopts the new shell + atoms, but their data layouts are not re-architected. The Overview screen in Section 5 is the reference.
- Tabs primitive, loader/skeleton variants beyond the basic shimmer row, tooltip primitive, collapsible primitive — added as needed during implementation.
- Mobile/responsive layout below 800px — dashboard is desktop-first; small viewports get a single-column fallback that is not a redesign.

## 3. Design tokens

Tokens are defined as CSS custom properties on `:root`. The dark theme is the default; the light theme overrides values under `[data-theme="light"]`. Tailwind 4's `@theme` directive registers the same tokens so utility classes (`bg-surface`, `text-muted`) work alongside raw `var(--*)`.

### 3.1 Color · dark theme (default)

| Token            | Value                | Use                                            |
|------------------|----------------------|------------------------------------------------|
| `--bg-base`      | `#0d1017`            | App background, sidebar background             |
| `--bg-surface`   | `#11141b`            | Panels, top bar                                |
| `--bg-raised`    | `#161a23`            | Hover, raised cells, modal background          |
| `--bg-selected`  | `rgba(108,182,255,.08)` | Selected nav item, selected row             |
| `--border-default` | `#1c2230`          | Default 1px borders                            |
| `--border-strong`| `#232a3a`            | Modal borders, focus container borders         |
| `--text-strong`  | `#ffffff`            | Page titles, primary numbers                   |
| `--text-body`    | `#c5c8d6`            | Default text                                   |
| `--text-muted`   | `#6e7686`            | Labels, timestamps, secondary text             |
| `--text-faint`   | `#4a5163`            | Disabled, placeholder, group labels            |
| `--accent`       | `#6cb6ff`            | Active nav, primary buttons, links             |
| `--ok`           | `#6dd47e`            | Success, healthy, additions                    |
| `--warn`         | `#ffaa3d`            | Warning, throttled                             |
| `--err`          | `#f4747a`            | Error, deletions, downtime                     |
| `--info`         | `#b48ead`            | Info hooks, neutral notifications              |
| `--neutral`      | `#8b95a7`            | Skip, no-op, neutral pills                     |

### 3.2 Color · light theme overrides

Only the values change; token names are identical.

| Token              | Light value              |
|--------------------|--------------------------|
| `--bg-base`        | `#fafbfc`                |
| `--bg-surface`     | `#ffffff`                |
| `--bg-raised`      | `#f4f6f8`                |
| `--bg-selected`    | `rgba(0,102,204,.08)`    |
| `--border-default` | `#e5e8ed`                |
| `--border-strong`  | `#d1d5db`                |
| `--text-strong`    | `#0a0d12`                |
| `--text-body`      | `#1a1d22`                |
| `--text-muted`     | `#6e7686`                |
| `--text-faint`     | `#a1a7b3`                |
| `--accent`         | `#0066cc`                |
| `--ok`             | `#2e9c47`                |
| `--warn`           | `#d97706`                |
| `--err`            | `#dc2626`                |
| `--info`           | `#7c3aed`                |
| `--neutral`        | `#4b5563`                |

Status colors (`--ok` / `--warn` / `--err` / `--info` / `--neutral`) are slightly darker in light theme to preserve contrast on white surfaces. Pills and dot indicators use `color-mix(in srgb, var(--ok) 15%, transparent)` for backgrounds so they automatically darken in light theme without per-theme overrides.

### 3.3 Typography

Two families: **Inter** for UI (system-ui fallback) and **JetBrains Mono** for data (numbers, timestamps, file paths, code, command keys).

| Token                | Family    | Size | Weight | Letter-spacing | Use                                  |
|----------------------|-----------|------|--------|----------------|--------------------------------------|
| `--text-display`     | Inter     | 22px | 600    | -0.01em        | Page titles                          |
| `--text-h1`          | Inter     | 16px | 600    | 0              | Section titles inside panels         |
| `--text-h2`          | Inter     | 13px | 600    | 0              | Subsection titles                    |
| `--text-body`        | Inter     | 12px | 400    | 0              | Default body text                    |
| `--text-small`       | Inter     | 11px | 400    | 0              | Secondary, hints                     |
| `--text-label`       | Inter     | 9px  | 600    | +0.08em uppercase | Group labels (`PROJECT`, `GLOBAL`) |
| `--mono-stat-large`  | JetBrains | 20px | 500    | tabular-nums   | Stat card primary numbers            |
| `--mono-stat`        | JetBrains | 14px | 400    | tabular-nums   | Inline metrics                       |
| `--mono-code`        | JetBrains | 11px | 400    | 0              | File paths, code, command names      |
| `--mono-time`        | JetBrains | 10px | 400    | tabular-nums   | Timestamps                           |

All `*` numeric tokens enable `font-variant-numeric: tabular-nums` for column alignment.

### 3.4 Spacing scale (4px base)

`--space-1` = 4px, `--space-2` = 8px, `--space-3` = 12px, `--space-4` = 16px (default panel padding), `--space-6` = 24px (section gap), `--space-8` = 32px.

### 3.5 Radii

`--radius-0` = 0 (ticks, hairlines), `--radius-1` = 2px (pills, badges), `--radius-2` = 4px (inputs, buttons), `--radius-3` = 6px (cards, table rows), `--radius-4` = 8px (panels, modals).

### 3.6 Density parameters

| Token              | Value | Note                                                      |
|--------------------|-------|-----------------------------------------------------------|
| `--row-height`     | 28px  | Table row, sidebar item                                   |
| `--panel-pad`      | 16px  | Default inner padding for panels                          |
| `--sidebar-width`  | 200px | Reduced from current 240px                                |
| `--header-height`  | 44px  | Top bar fixed height                                      |

## 4. App shell

CSS grid two-column × two-row: sidebar spans both rows; top bar in `(2,1)`; main in `(2,2)`.

### 4.1 Sidebar (200px)

- **Brand block** at top: 14px Inter 600 logo "mneme" with a 14px gradient square mark (linear-gradient 135deg `#6cb6ff` → `#4d8fcc`). Right side shows app version in `--mono-time` color `--text-muted`. Bottom border `--border-default`.
- **Three groups** below brand, each prefixed with a `--text-label` heading:
  - `GLOBAL` — Overview, Activity, Cron
  - `PROJECT` — Cerebrum, Memory, Anatomy, BugLog, Suggestions
  - `TOOLS` — Token, DesignQC
- **Nav item**: 11px Inter, `--text-muted` default, 5px vertical / 14px horizontal padding, optional 12px icon (mono glyph) on the left and optional badge on the right. Active state: `--accent` text, `--bg-selected` background, 2px left border in `--accent` (compensated by reducing left padding to 12px). Hover: `--bg-raised` background, `--text-body` text.
- **Badge variants**: live indicator (`--accent` solid pill, "live" lowercase), count (`--bg-raised` background, `--text-muted` text), severity count (`color-mix` background with `--err` for bug count, `--warn` for warnings).
- **Footer block** pinned to bottom: 9px JetBrains Mono in `--text-muted`, two lines: `build <sha> · go <version>` and `uptime <h>h <m>m`. Top border `--border-default`.

### 4.2 Top bar (44px)

Left to right:

1. **Project picker** — pill button with breadcrumb `~/workspace / mneme` and `▾` caret. Click opens a dropdown of recently active projects (existing `ProjectPicker.tsx` content, restyled).
2. **Search affordance** — 300px input-styled button "⌕ Search files, rules, bugs…" with `⌘K` kbd badge on right. Click opens a modal that is empty for v1 ("Search not yet implemented — coming in a follow-up spec."). Out-of-scope for behavior; in-scope for layout.
3. **Spacer** — `flex: 1`.
4. **SSE indicator** — 6px dot with breathing animation (see Section 6) + monospace label `SSE · <latency>ms`. Latency comes from existing `useSSEConnected` hook (extended to expose latency — minor backend change: SSE messages already carry timestamps, client computes `now - lastEventTime`).
5. **Theme toggle** — 26×26px icon button, `◐` glyph. Cycles `dark → light → system`. Tooltip shows current state.
6. **Settings** — 26×26px icon button, `⚙` glyph. Opens a settings modal (existing `Token` panel content can move here later; v1 just routes to current `/token`).

### 4.3 Main area

Per-page header pattern:

```
┌─ page-head ─────────────────────────────────────────┐
│  Overview                  window=24h · refresh=30s │
│                                  [Export] [Scan now]│
└─────────────────────────────────────────────────────┘
```

- **Title** (`--text-display`, `--text-strong`)
- **Meta line** (`--mono-time`, `--text-muted`) inline to the right of the title
- **Page actions** (right-aligned): zero or more buttons. The rightmost is typically the primary action (`btn-primary` style).

Below the page head, content panels use 12px gap. Panels use `--panel-pad` (16px) and `--bg-surface` background with `--border-default` 1px border, `--radius-4` (8px).

## 5. Component primitives

### 5.1 Stat card

```
┌──────────────────────┐
│ TURNS 24H            │  ← --text-label, --text-muted
│ 1,284  ↑12%          │  ← --mono-stat-large + delta
│ ▁▂▃▄▅▆▇             │  ← optional 18px sparkline
└──────────────────────┘
```

- 12px padding, `--radius-3`, `--bg-surface`.
- Delta arrow: `↑` `--ok`, `↓` `--err`. Percentage uses `--mono-time` size.
- Sparkline: SVG polyline, stroke = trend color (matches delta direction), 1px stroke, no fill. Last 60 data points. Optional — omit when no time-series data exists; replace with one-line breakdown text in `--mono-time`.

### 5.2 Pill

`1px 6px` padding, `--radius-1`, `--text-label` size + weight (9px / 600), uppercase. Background uses `color-mix(in srgb, <color> 15%, transparent)`, foreground uses the color directly. Six variants: `write`/`read`/`warn`/`err`/`info`/`neutral` mapping to status colors. One solid variant (`pill-solid`) on `--accent` for "LIVE" indicator.

### 5.3 Dot

6×6px circle. Healthy state (`--ok`) gets `box-shadow: 0 0 6px color-mix(in srgb, var(--ok) 50%, transparent)` plus breathing animation. Other states are flat.

### 5.4 Button

| Variant      | Background       | Border               | Text             |
|--------------|------------------|----------------------|------------------|
| Default      | `--bg-base`      | `--border-default`   | `--text-body`    |
| Primary      | `--accent`       | `--accent`           | `--bg-base` 500w |
| Danger       | transparent      | `color-mix(--err 30%, transparent)` | `--err` |
| Ghost        | transparent      | transparent          | `--text-muted`   |

5px / 12px padding, `--radius-2`, 11px Inter. Sizes: default + `btn-sm` (2px / 8px / 10px) + `btn-icon` (26×26px square).

### 5.5 Input

5px / 10px padding, `--radius-2`, `--bg-base` background, `--border-default` border. Focus: `--accent` border + `0 0 0 2px color-mix(--accent 15%, transparent)` ring. Placeholder `--text-faint`. Mono variant swaps font to JetBrains.

### 5.6 Table

- Header row: 9px uppercase `--text-label`, `--text-muted`, 6px / 10px padding, bottom border `--border-default`.
- Body row: 11px Inter, 6px / 10px padding, bottom border `color-mix(--border-default 40%, transparent)`. `--row-height` controls minimum height.
- Hover row: `color-mix(--accent 4%, transparent)` background + `inset 2px 0 0 --accent` shadow on first cell (left edge accent).
- Data cells with monospace content (paths, deltas, timestamps) use `--mono-code` or `--mono-time`.

### 5.7 Empty state

Centered 32px / 16px padding block with: 24px glyph at 30% opacity, 11px primary line, optional 10px hint with `kbd` badges for relevant CLI commands.

### 5.8 Toast

10px / 14px padding, `--bg-raised` background, 1px `--border-default` border, 3px left border in status color. 11px Inter. Used for transient confirmations / errors. Auto-dismiss 4s, max 3 stacked top-right.

### 5.9 Modal

`--bg-surface` background, `--border-strong` 1px border, `--radius-4`, 20px padding, `0 20px 50px rgba(0,0,0,.5)` shadow, max-width 400px (auto-resize for content). Title 14px / 600 / `--text-strong`. Body 11px / `--text-muted` line-height 1.5. Actions row right-aligned with 8px gap.

### 5.10 Kbd

`--mono-time` size, 1px / 5px padding, `--radius-1`, `--bg-base` background, `--border-default` 1px border, `--text-muted` color.

## 6. Theme toggle

- **Storage key**: `mneme.theme`. Values: `"dark"` | `"light"` | `"system"`. Default when key absent: `"system"`.
- **Resolved theme**: when storage value is `"system"`, resolved theme is derived from `window.matchMedia('(prefers-color-scheme: dark)')` and updated reactively when the OS theme changes.
- **DOM contract**: `<html data-theme="dark">` or `<html data-theme="light">`. Tokens scope under these attribute selectors.
- **First-paint flash prevention**: in `web/index.html` `<head>` (before any `<link rel="stylesheet">`), inline this script literally:

  ```html
  <script>
    (function () {
      var s = localStorage.getItem('mneme.theme') || 'system';
      var t = s === 'system'
        ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
        : s;
      document.documentElement.setAttribute('data-theme', t);
    })();
  </script>
  ```

  This script must run before any CSS request — kept inline in `index.html`, not imported.
- **Toggle button cycles**: `dark → light → system → dark → …`. The button glyph stays `◐`; tooltip text reflects current selection ("Theme: dark", etc.).
- **CSS transition**: `html { color-scheme: dark light; } body { transition: background-color 200ms ease, color 200ms ease; }`. No transition on `data-theme` attribute itself — only on color properties to avoid layout jank.

## 7. Motion

- **SSE dot**: 2s ease-in-out infinite breathing — `opacity: 0.5 → 1.0 → 0.5`. When `connected === false`, dot becomes `--text-faint` and animation pauses.
- **SSE latency label**: refreshes every 1s from `now - lastEventTimestamp`. When > 5s, label text turns `--warn`. When > 30s or disconnected, label text turns `--err` and reads `SSE · offline`.
- **Stat number transition**: when a stat value updates and the delta is small enough to look animated (numerically: |new - old| ≤ 1000 OR new value < 10000), use a 200ms CSS counter animation via `animation-timing-function: steps(20)`. For larger deltas, swap value directly and flash background `color-mix(--accent 8%, transparent)` for 400ms.
- **New feed row insertion**: prepend at top with starting state `background-color: color-mix(--accent 8%, transparent)` then transition to `transparent` over 600ms. The list does not auto-scroll — preserve user scroll position; new rows appear at top regardless of scroll.
- **Sparkline update**: re-render entire polyline when data changes. No interpolation between renders — react state-driven, simple and fast.
- **Route transition**: no transition on container; main content area `opacity: 0 → 1` over 100ms on each route mount. Sidebar and top bar do not re-render.
- **Loading state**: render skeleton rows of `--row-height` height with `--bg-raised` background and a 1.5s linear shimmer (CSS gradient sweep). No spinners anywhere.

## 8. Migration approach

### 8.1 File structure changes

```
web/src/
  styles/
    tokens.css         ← new: :root + [data-theme="light"] var declarations
    base.css           ← new: html/body resets, theme transition
    fonts.css          ← new: @font-face for Inter + JetBrains Mono
  components/
    AppShell.tsx       ← rewrite: grid layout, no Tailwind utility soup
    Sidebar.tsx        ← new: extracted from AppShell; takes nav config
    TopBar.tsx         ← new: project picker + search + SSE + theme + settings
    PageHead.tsx       ← new: title/meta/actions row used by every panel
    primitives/        ← new directory
      Stat.tsx
      Pill.tsx
      Dot.tsx
      Button.tsx
      Input.tsx
      Table.tsx
      Empty.tsx
      Toast.tsx
      Modal.tsx
      Kbd.tsx
      Sparkline.tsx
      Skeleton.tsx
  hooks/
    useTheme.tsx       ← new: storage + system listener + resolved value
    useSSEConnected.tsx ← extend: also expose latencyMs
  styles.css           ← rewrite: only @import statements, no per-element rules
```

The current `body { @apply max-w-2xl mx-auto px-4 py-16 }` rule in `styles.css` is removed — it conflicts with the flex shell.

### 8.2 Tailwind 4 token registration

`web/src/styles/tokens.css` declares CSS custom properties on `:root` and `[data-theme="light"]`. `styles.css` registers the same names with Tailwind 4's `@theme`:

```css
@import "tailwindcss";
@import "./styles/tokens.css";

@theme {
  --color-bg-base: var(--bg-base);
  --color-bg-surface: var(--bg-surface);
  --color-text-body: var(--text-body);
  --color-text-muted: var(--text-muted);
  --color-accent: var(--accent);
  /* …one entry per token */
}
```

This makes utility classes like `bg-bg-surface`, `text-text-muted`, `border-border-default` resolve to the CSS variables, so panel code can mix utility classes with semantic primitives without duplicating values.

### 8.3 Per-panel migration

Each panel keeps its data fetching and logic. Only the rendering layer changes:

1. Wrap content in `<PageHead title="…" meta="…" actions={…} />` followed by panel content.
2. Replace ad-hoc cards with `<Stat>`, `<Table>`, etc. primitives.
3. Replace inline Tailwind utility colors (`bg-gray-50`, `text-gray-500`) with token utilities (`bg-surface`, `text-muted`).

Panels migrated in order: Overview → Activity → BugLog → Cerebrum → Memory → Anatomy → Suggestions → Cron → Token → DesignQC. Each panel is a separate commit; tests for each panel re-run before moving on.

### 8.4 Backward compatibility

None required. The dashboard is internal tooling shipped as embedded assets — no consumers depend on its DOM structure or class names. Existing tests in `web/src/__tests__/` may need selectors updated; this is part of the implementation plan.

## 9. Acceptance criteria

The redesign is done when:

1. All ten existing panels render without console errors in both themes.
2. `mneme dashboard` end-to-end: opens browser, loads index, all assets 200, no MIME warnings, no auth 401s. (Already fixed; verify regression-free.)
3. Theme toggle persists across reload; `system` mode tracks OS preference change live without reload.
4. No first-paint flash when reloading on either theme — verified by recording a screen capture and inspecting first 200ms.
5. SSE indicator: dot breathes when connected, freezes gray when disconnected, shows latency.
6. Stat values animate on change (within the rules in Section 7); new feed rows highlight then fade.
7. Sidebar three groups render with correct labels; active item shows accent left border.
8. All `web/src/__tests__/` tests pass (selector updates accepted as part of migration).
9. New tests added for: `useTheme` hook (storage + system listener), `<Sparkline>` with empty / single / many points, `<PageHead>` rendering with and without actions.
10. Bundle size after build is within 50kB of current size (no major dependency added).
11. `⌘K` button opens a modal containing the placeholder text "Search not yet implemented — coming in a follow-up spec." with a Close action.

## 10. Open questions / accepted scope cuts

- **`⌘K` search behavior** — affordance only, no functionality. Follow-up spec required.
- **Tabs / tooltip / collapsible primitives** — added during implementation if a panel needs them. Not pre-emptively designed.
- **Mobile** — not designed; dashboard is desktop-only. Below 800px the sidebar overlays the content (existing CSS) but is not redesigned.
- **Settings modal** — Section 4.2 says the gear button "routes to current `/token`"; full settings consolidation is future work.
- **SSE latency exposure** — requires extending `useSSEConnected` to compute and surface `latencyMs`. Backend already emits timestamps; this is a client-only change but listed for visibility.
