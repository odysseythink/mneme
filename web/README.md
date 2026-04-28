# mneme dashboard frontend

Vite + React + TypeScript + Tailwind v4. Built artifacts go in `web/dist/`,
which is committed (the placeholder version) and copied into
`pkg/dashboard/dist/` so `go:embed` snapshots them into the binary.

## Build

From the repo root:

    make build           # full release: pnpm install + pnpm build + go build
    make web-build       # frontend only (also copies dist into pkg/dashboard/dist)
    make go-build        # backend only
    make web-clean       # restore dist/index.html to placeholder

## Dev mode A — Vite dev server (recommended)

Run two terminals:

    # Terminal 1 — daemon with dev-token endpoint enabled
    mneme daemon start --dev

    # Terminal 2 — Vite dev server on :5173
    make web-dev

In your browser, visit `http://localhost:5173/`. You'll see "Unauthorized"
until you set the cookie. Open DevTools console and run:

    fetch('/dev-token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: '<paste contents of ~/.mneme/daemon/token>' }),
    }).then(r => r.json()).then(console.log)

The cookie now scopes to `:5173` (because the request went through Vite's
proxy) and HMR works against live daemon data.

## Dev mode B — `MNEME_DASHBOARD_DEV_DIR` filesystem override

If you'd rather build the frontend in watch mode and skip the proxy:

    cd web && pnpm build --watch &
    MNEME_DASHBOARD_DEV_DIR=$PWD/dist mneme daemon start

The daemon serves directly from `web/dist/` — no `go build` needed on
each change.
