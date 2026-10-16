# Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the React dashboard (overview, pot detail, pot settings with calibration wizard, nodes) and embed it in the `waterer` binary, so `docker compose up` serves the full prototype at `http://localhost:8080`.

**Architecture:** A Vite React TypeScript app in `web/`. Typed API access uses types generated from `api/openapi.yaml` (`openapi-typescript`) and the `openapi-fetch` client, wrapped in TanStack Query hooks that poll every 30 s. Pure display logic (badges, staleness, chart shaping) lives in `src/lib/` and has unit tests. Components get Testing Library tests. The release build is embedded in Go with `go:embed` and served with an SPA fallback.

**Tech Stack:** React 18, TypeScript, Vite 6, Tailwind CSS 4 (`@tailwindcss/vite`), TanStack Query 5, React Router 7 (`react-router`), Recharts 2, `openapi-typescript` 7 + `openapi-fetch`, Vitest + jsdom + `@testing-library/react` + `@testing-library/user-event`. Node 22.

**Spec:** [docs/plant-waterer-design.md](../../plant-waterer-design.md) (§9 Dashboard, §10 dashboard rows)

**Depends on:** [2026-10-16-backend-and-simulator.md](2026-10-16-backend-and-simulator.md), complete (the API contract and `cmd/waterer`).

## Global Constraints

- Generated API types are the only source of response shapes. Never hand-write duplicates. Regenerate with `npm run gen` after any `openapi.yaml` change.
- Polling is TanStack Query `refetchInterval: 30_000`. No WebSockets.
- No login. Metric units (ml, °C, %, lux).
- In dev, the Vite server proxies `/api` to `http://localhost:8080`. In release, the build is embedded in the Go binary.
- Badge copy is exactly: `OK`, `Water ~250 ml` (the amount from advice), `Sensor offline`, `Needs calibration`. When there's no advice yet: `No check yet`.
- The disagreement marker reads `LLM and rules disagree`.
- Failed requests show a non-blocking toast. The page keeps showing the last good data.
- Category defaults for rules live only in the backend. The create form leaves threshold and fraction empty, so the server fills them.

## Review Focus

1. **A pot that has never been checked** (`latest_advice: null`) must render a card with `No check yet`, not crash. The test is in Task 2.
2. **An uncalibrated pot** (`moisture_pct: null`) shows `—` in the gauge, the pot detail chart says `Calibrate the sensor to see moisture %`, and raw values aren't plotted as a percentage. Tests are in Task 2 and Task 3.
3. **A slow "Check now"** (the LLM can take up to about 60 s with a retry): the button shows `Checking…` and is disabled until the request settles, so there are no duplicate checks. The test is in Task 3.
4. **The calibration wizard on a channel with no reading yet:** the capture buttons are disabled and the wizard shows `Waiting for a reading from the sensor`. The test is in Task 4.
5. **Backend unreachable during polling:** a toast appears, and the cards already on screen stay visible. The test is in Task 2.

---

## File Structure

```
web/  package.json  vite.config.ts  tsconfig.json  index.html  dist/.gitkeep  embed.go  embed_test.go
web/src/  main.tsx  App.tsx  index.css  test/setup.ts  test/fixtures.ts
web/src/api/  schema.ts (generated)  client.ts  hooks.ts
web/src/lib/  format.ts  status.ts  chart.ts  (+ *.test.ts)
web/src/components/  Toast.tsx  Layout.tsx  PotCard.tsx  MoistureGauge.tsx  StatusBadge.tsx  PotCharts.tsx  AdviceHistory.tsx  LogWatering.tsx  CheckNowButton.tsx  CalibrationWizard.tsx  PotForm.tsx  (+ *.test.tsx)
web/src/pages/  Overview.tsx  PotDetail.tsx  PotSettings.tsx  Nodes.tsx
```

---

### Task 1: Scaffold, typed client, display helpers

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx` (a router with placeholder pages), `web/src/index.css` (`@import "tailwindcss";`), `web/src/test/setup.ts`, `web/src/test/fixtures.ts`, `web/src/api/client.ts`, `web/src/api/hooks.ts`, `web/src/lib/format.ts`, `web/src/lib/status.ts`
- Generated: `web/src/api/schema.ts`
- Test: `web/src/lib/format.test.ts`, `web/src/lib/status.test.ts`

**Interfaces:**
- Produces:
  ```ts
  // api/client.ts
  export type Pot = components['schemas']['Pot']; export type Advice = …; export type Node = …; export type HistoryPoint = …; export type Watering = …; export type ApiError = components['schemas']['Error']
  export const api = createClient<paths>({ baseUrl: '/api/v1' })
  export class ApiRequestError extends Error { constructor(public status: number, public body: ApiError) }
  export function unwrap<T>(r: { data?: T; error?: ApiError; response: Response }): T   // throws ApiRequestError
  // api/hooks.ts (all queries refetchInterval 30_000)
  usePots(), usePot(id: number), useAdvice(id: number), useHistory(id: number, range: Range), useWaterings(id: number), useNodes(opts?: { refetchInterval?: number })
  useCheckNow(id), useLogWatering(id), useCreatePot(), useUpdatePot(id), useDeletePot(), useRenameNode()   // invalidate ['pots'], ['pot', id], ['advice', id], ['history', id], ['nodes'] as relevant
  // lib/format.ts
  export function formatAgo(iso: string, now: Date): string      // "just now" (<1 min), "5 min ago", "2 h ago", "3 d ago"
  export function formatPct(v: number | null | undefined): string // "42%" (rounded) or "—"
  // lib/status.ts
  export type Tone = 'ok' | 'water' | 'warn' | 'neutral'
  export function badgeFor(pot: Pot): { label: string; tone: Tone }
  export function staleHint(pot: Pot, now: Date): string | null  // linked & no reading → "No readings yet"; last reading > 15 min → "Last reading 2 h ago"; else null
  export type Range = '24h' | '7d' | '30d'
  export function rangeQuery(range: Range, now: Date): { from: string; to: string; step: number }  // steps: 24h→300, 7d→3600, 30d→10800
  ```
- `test/fixtures.ts`: `makePot(overrides?: Partial<Pot>): Pot` and `makeAdvice(overrides?: Partial<Advice>): Advice`, with valid defaults (calibrated foliage pot, `ok` advice that needs no water).

- [ ] **Step 1: Scaffold**

Run: `npm create vite@latest web -- --template react-ts`, then in `web/`: `npm i @tanstack/react-query react-router recharts openapi-fetch` and `npm i -D openapi-typescript tailwindcss @tailwindcss/vite vitest jsdom @testing-library/react @testing-library/user-event @testing-library/jest-dom`.
Scripts: `dev`, `build` (`tsc -b && vite build`), `typecheck` (`tsc -b --noEmit`), `test` (`vitest`), `gen` (`openapi-typescript ../api/openapi.yaml -o src/api/schema.ts`).
`vite.config.ts`: react + tailwind plugins, `server.proxy['/api'] = 'http://localhost:8080'`, `test: { environment: 'jsdom', setupFiles: './src/test/setup.ts' }`. Keep `web/dist/.gitkeep` (Vite's `emptyOutDir` removes it, so set `build.emptyOutDir: true` and recreate `.gitkeep` in a `postbuild` script: `node -e "require('fs').writeFileSync('dist/.gitkeep','')"`).

- [ ] **Step 2: Generate types**

Run: `npm run gen`
Expected: `src/api/schema.ts` exports `paths` and `components`.

- [ ] **Step 3: Write the failing tests**

```ts
test('formatAgo', () => {
  const now = new Date('2026-10-16T12:00:00Z')
  expect(formatAgo('2026-10-16T11:59:40Z', now)).toBe('just now')
  expect(formatAgo('2026-10-16T11:55:00Z', now)).toBe('5 min ago')
  expect(formatAgo('2026-10-16T10:00:00Z', now)).toBe('2 h ago')
  expect(formatAgo('2026-10-13T12:00:00Z', now)).toBe('3 d ago')
})
test('badgeFor', () => {
  expect(badgeFor(makePot({ latest_advice: null })).label).toBe('No check yet')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ status: 'sensor_offline', recommendation: null }) })).label).toBe('Sensor offline')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ status: 'needs_calibration', recommendation: null }) })).label).toBe('Needs calibration')
  expect(badgeFor(makePot({ latest_advice: makeAdvice({ recommendation: { needs_water: true, amount_ml: 250, confidence: 'medium', reason: 'x' } }) })))
    .toEqual({ label: 'Water ~250 ml', tone: 'water' })
  expect(badgeFor(makePot()).label).toBe('OK')
})
test('staleHint', () => { /* null when reading 5 min old; "Last reading 2 h ago"; "No readings yet" when linked and last_reading_at null; null when unlinked */ })
test('rangeQuery', () => { /* 7d → from = now-7d ISO, step 3600 */ })
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `npm test -- --run`
Expected: FAIL

- [ ] **Step 5: Implement the client, hooks, `format.ts` and `status.ts`**

- [ ] **Step 6: Run the tests and typecheck**

Run: `npm test -- --run && npm run typecheck`
Expected: PASS, no type errors.

- [ ] **Step 7: Commit**

```bash
git add web/   # node_modules and dist/* are already in .gitignore (dist/.gitkeep is allowed)
git commit -m "feat(web): scaffold dashboard with typed API client and display helpers"
```

---

### Task 2: Layout, toasts, Overview

**Files:**
- Create: `web/src/components/Toast.tsx`, `Layout.tsx`, `StatusBadge.tsx`, `MoistureGauge.tsx`, `PotCard.tsx`, `web/src/pages/Overview.tsx`
- Modify: `web/src/main.tsx` (QueryClient whose `QueryCache`/`MutationCache` `onError` calls the toast), `web/src/App.tsx`
- Test: `web/src/components/PotCard.test.tsx`, `web/src/pages/Overview.test.tsx`

**Interfaces:**
- Consumes: Task 1 hooks, `badgeFor`, `staleHint`, `formatAgo`, `formatPct`, and fixtures.
- Produces:
  ```ts
  export function ToastProvider(props: { children: ReactNode }): JSX.Element
  export function useToast(): { show(message: string): void }   // auto-dismiss after 5 s, stacked bottom-right
  export function errorMessage(e: unknown): string               // ApiRequestError → body.message; TypeError (network) → "Can't reach the server"
  export function PotCard(props: { pot: Pot; now: Date }): JSX.Element   // links to /pots/:id
  export function MoistureGauge(props: { pct: number | null }): JSX.Element
  export function StatusBadge(props: { pot: Pot }): JSX.Element
  ```
  Layout: a top nav with `Pots` (`/`) and `Nodes` (`/nodes`), and a centered content column that stays responsive down to phone width. Routes: `/`, `/pots/new`, `/pots/:id`, `/pots/:id/settings`, `/nodes`.
  PotCard shows: name, species, gauge, badge, `Checked 2 h ago` (from `latest_advice.created_at`, omitted when there's no advice), the disagreement marker when `latest_advice.disagree`, and the stale hint.
  The overview empty state (no pots): `No pots yet. Go to Nodes to create one from a sensor channel.` with a link.

- [ ] **Step 1: Write the failing tests**

Render with a helper `renderWithProviders(ui, { client })` placed in `src/test/setup.ts` exports (QueryClient with `retry: false`, ToastProvider, MemoryRouter).

```tsx
test('card shows OK pot', …)                         // name, "42%", "OK", "Checked 2 h ago"
test('card never checked', …)                         // Review Focus 1: latest_advice null → "No check yet", no "Checked"
test('card uncalibrated shows dash', …)               // Review Focus 2: moisture_pct null → "—"
test('card shows disagreement marker', …)             // disagree true → "LLM and rules disagree"
test('card shows stale hint', …)                      // last_reading_at 2h ago → "Last reading 2 h ago"
test('overview lists pots from API', …)               // mock fetch GET /api/v1/pots → 2 cards
test('overview keeps cards and toasts on failure', …) // Review Focus 5: first fetch OK, refetch rejects TypeError → toast "Can't reach the server", both cards still present
```
Mock `fetch` with `vi.spyOn(globalThis, 'fetch')`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement the components and the page**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `npm test -- --run && npm run typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(web): layout, toasts and pot overview"
```

---

### Task 3: Pot detail

**Files:**
- Create: `web/src/lib/chart.ts`, `web/src/components/PotCharts.tsx`, `CheckNowButton.tsx`, `LogWatering.tsx`, `AdviceHistory.tsx`, `web/src/pages/PotDetail.tsx`
- Test: `web/src/lib/chart.test.ts`, `web/src/components/CheckNowButton.test.tsx`, `LogWatering.test.tsx`, `AdviceHistory.test.tsx`

**Interfaces:**
- Consumes: `usePot`, `useHistory`, `useAdvice`, `useWaterings`, `useCheckNow`, `useLogWatering`, `rangeQuery`.
- Produces:
  ```ts
  // lib/chart.ts
  export type ChartRow = { t: number; moisture: number | null; temp: number | null; humidity: number | null; light: number | null }
  export function toRows(points: HistoryPoint[]): ChartRow[]                 // t = epoch ms
  export function markers(waterings: Watering[], advice: Advice[], from: number, to: number):
    { waterings: { t: number; ml: number }[]; advice: { t: number; needsWater: boolean }[] }   // only within [from, to], advice only status ok
  export function PotCharts(props: { pot: Pot; range: Range; onRangeChange(r: Range): void }): JSX.Element
  export function CheckNowButton(props: { potId: number }): JSX.Element
  export function LogWatering(props: { potId: number; suggestedMl: number | null }): JSX.Element
  export function AdviceHistory(props: { items: Advice[] }): JSX.Element
  ```
  Charts (Recharts `ResponsiveContainer`, X axis time): (1) moisture % with a `ReferenceLine` per watering and a dot per advice (filled when `needsWater`); (2) temperature and humidity on two Y axes; (3) light. Range tabs: `24 h`, `7 d`, `30 d`. When the pot isn't calibrated, the moisture chart is replaced by `Calibrate the sensor to see moisture %` with a link to settings.
  The latest advice panel shows the reason, `provider · model`, a `Fallback to rules` tag with `fallback_reason` when `fallback`, and the disagreement marker.
  Check now: the label is `Check now`, or `Checking…` while pending (disabled). On success it invalidates the advice and pot queries.
  Log watering: an amount input in ml, prefilled with `suggestedMl` (from the latest `ok` advice that needs water, otherwise empty), and a `Log watering` submit. It shows `Logged 250 ml` on success. Valid amounts are 1–20000.
  Advice history: one row per advice (time, badge-style status, amount, provider). Clicking a row toggles a `<pre>` with the pretty-printed `summary` JSON (or `No summary (status: sensor_offline)`).

- [ ] **Step 1: Write the failing tests**

```ts
test('toRows maps nulls through', …)
test('markers filters to range and ok advice', …)
test('check now disables while pending', …)   // Review Focus 3: fetch promise unresolved → button text "Checking…", disabled; second click doesn't call fetch again; resolve → "Check now"
test('log watering prefilled and posts amount', …) // suggestedMl 250 → input value "250"; submit → POST /api/v1/pots/1/waterings body {amount_ml:250}; "Logged 250 ml"
test('log watering rejects 0', …)              // input 0 → submit disabled
test('advice history expands summary', …)      // click row → shows `"moisture_pct": 18`
test('detail page uncalibrated message', …)    // Review Focus 2: pot dry_raw null → "Calibrate the sensor to see moisture %"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement `chart.ts`, the components and `PotDetail`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `npm test -- --run && npm run typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(web): pot detail with charts, check now, watering log and advice history"
```

---

### Task 4: Pot settings, create form, calibration wizard

**Files:**
- Create: `web/src/components/PotForm.tsx`, `CalibrationWizard.tsx`, `web/src/pages/PotSettings.tsx`
- Test: `web/src/components/PotForm.test.tsx`, `CalibrationWizard.test.tsx`

**Interfaces:**
- Consumes: `usePot`, `useNodes({ refetchInterval: 5000 })` while the wizard is open, `useCreatePot`, `useUpdatePot`, `useDeletePot`.
- Produces:
  ```ts
  export type PotFormValues = { name: string; species: string; category: Pot['category']; pot_ml: number; soil_type: string; placement: string;
    node_id: string | null; channel: number | null; threshold_pct: number | null; amount_fraction: number | null; check_interval_min: number }
  export function PotForm(props: { initial: PotFormValues; nodes: Node[]; mode: 'create' | 'edit'; onSubmit(v: PotFormValues): void; busy: boolean }): JSX.Element
  export function CalibrationWizard(props: { latestRaw: number | null; onSave(dry: number, wet: number): void; onCancel(): void }): JSX.Element
  ```
  PotForm: the node/channel select lists `node · ch N` for each channel plus `Not linked`, and channels linked to another pot are disabled. In create mode, threshold and fraction show the placeholder `Category default` and are sent only when filled. The fraction field shows `% of pot volume`, so the UI shows 20 for 0.2. The check interval is entered in hours (default 6) and sent as minutes. The page `/pots/new?node=…&ch=…` preselects the link. A 400 response with `details` shows each message under its field, and a 409 shows its message in a toast.
  Wizard steps: (1) `Hold the sensor in the air, then press Capture dry` with the current raw value shown live; (2) `Put the sensor in water up to the line, then press Capture wet`; (3) a review of `Dry 540 · Wet 270` with `Save` and `Start over`. When `latestRaw` is null, the capture buttons are disabled and the wizard shows `Waiting for a reading from the sensor`. When dry equals wet, Save is disabled and the wizard shows `Dry and wet values must differ`. Saving sends `PATCH {dry_raw, wet_raw}`.
  The settings page also has a `Delete pot` button with a `confirm()` prompt that navigates to `/`.

- [ ] **Step 1: Write the failing tests**

```tsx
test('wizard captures dry then wet and saves', …)  // latestRaw 540 → Capture dry; rerender latestRaw 270 → Capture wet; Save → onSave(540, 270)
test('wizard waits for a reading', …)              // Review Focus 4: latestRaw null → capture disabled, "Waiting for a reading from the sensor"
test('wizard blocks equal values', …)              // same raw for both → Save disabled, "Dry and wet values must differ"
test('form create omits empty rules', …)           // submit → onSubmit with threshold_pct null; page-level: request body has no threshold_pct key
test('form shows field errors from 400', …)        // mocked 400 details [{field:"pot_ml", message:"must be between 50 and 50000"}] → message rendered near Pot volume
test('form converts fraction percent and hours', …) // enter 25 (%) and 12 (h) → amount_fraction 0.25, check_interval_min 720
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement the form, the wizard and the settings/create pages**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `npm test -- --run && npm run typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(web): pot settings, create form and calibration wizard"
```

---

### Task 5: Nodes page

**Files:**
- Create: `web/src/pages/Nodes.tsx`
- Test: `web/src/pages/Nodes.test.tsx`

**Interfaces:**
- Consumes: `useNodes`, `useRenameNode`, `formatAgo`.
- Produces: the Nodes page. Each node shows its name (or ID when the name is empty), an inline rename, and `Last seen 3 min ago`. A channel table lists `ch`, the latest raw value and its age, and the linked pot (a link to `/pots/:id`) or a `Create pot from this channel` button that navigates to `/pots/new?node=<id>&ch=<n>`. The empty state reads `No nodes have reported yet. Start the simulator or a sensor node.`

- [ ] **Step 1: Write the failing tests**

```tsx
test('lists nodes and channels', …)          // node with ch0 linked (pot link shown) and ch1 unlinked ("Create pot from this channel")
test('create pot button navigates with query', …) // click → location "/pots/new?node=windowsill-1&ch=1"
test('rename node', …)                       // edit name → PATCH /api/v1/nodes/windowsill-1 {name}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement `Nodes.tsx`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `npm test -- --run && npm run typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(web): nodes page with create-pot-from-channel"
```

---

### Task 6: Embed in the Go binary and ship

**Files:**
- Create: `web/embed.go`, `web/embed_test.go`
- Modify: `cmd/waterer/main.go` (mount `/` → `web.Handler()`, with `/api/v1/` kept first), `Dockerfile` (Node stage), `Makefile` (`test`, `generate`), `.dockerignore` (`web/node_modules`), `CLAUDE.md` (dashboard commands)

**Interfaces:**
- Produces:
  ```go
  package web
  //go:embed all:dist
  var dist embed.FS
  func Handler() http.Handler
  ```
  Behavior: serve files from `dist`. Unknown paths that don't start with `/api/` and have no file extension get `index.html` (SPA routes). Missing files with an extension give 404. Files under `/assets/` get `Cache-Control: public, max-age=31536000, immutable`, and `index.html` gets `no-cache`. When `dist/index.html` is absent (a Go build without a frontend build), every non-asset path returns 503 `dashboard not built: run npm run build in web/`.

- [ ] **Step 1: Write the failing test**

```go
func TestHandlerServesIndexAndFallback(t *testing.T) // uses fstest.MapFS injected via an unexported newHandler(fsys fs.FS) used by Handler(); "/" and "/pots/3" → index body, no-cache; "/assets/a.js" → immutable; "/missing.js" → 404
func TestHandlerNotBuilt(t *testing.T)               // MapFS without index.html → 503 with message
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./web/...`
Expected: FAIL

- [ ] **Step 3: Implement `Handler` and mount it in `main`**

- [ ] **Step 4: Update the Dockerfile, Makefile and CLAUDE.md**

- Dockerfile: a new first stage `web`, `FROM --platform=$BUILDPLATFORM node:22-alpine`, `WORKDIR /src/web`, copy `web/package*.json` and `npm ci`, copy `web/` and `api/openapi.yaml` (to `/src/api/`), `npm run build`. The Go build stage copies `/src/web/dist` to `web/dist` before `go build`.
- Makefile: `test` adds `cd web && npm ci && npm run typecheck && npm test -- --run`, and `generate` adds `cd web && npm run gen`.
- CLAUDE.md: `cd web && npm run dev` (with `go run ./cmd/waterer` running), `npm test -- --run`, a single test `npx vitest run src/lib/status.test.ts`, and `npm run gen` after an `openapi.yaml` change.

- [ ] **Step 5: Verify the full stack**

Run: `go test ./... && (cd web && npm test -- --run)`, then `docker compose up --build -d`, then open `http://localhost:8080` in the browser pane.
Expected: tests pass. The Nodes page shows `windowsill-1` with 3 unlinked channels. Creating a pot from ch0 and running the calibration wizard (dry 540, wet 270) gives an Overview card with a moisture %. `Check now` produces advice. `Log watering` 400 ml makes the moisture chart rise within about 1 min, and the next check changes the recommendation (spec DoD 1–5).

- [ ] **Step 6: Commit**

```bash
git add web/embed.go web/embed_test.go cmd/waterer/main.go Dockerfile Makefile .dockerignore CLAUDE.md
git commit -m "feat: embed dashboard in waterer binary and docker image"
```
