# Backend and Simulator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `waterer` Go backend (ingest, SQLite storage, advisor with LLM + rules fallback, scheduler, REST API) and the `simulator` Go program, so that `docker compose up` produces readings, advice and watering effects that can be checked with curl.

**Architecture:** One Go module with two binaries. `core/` holds pure domain logic and the store/advisor interfaces. `store/sqlite`, `advisor/openaicompat`, `ingest`, `scheduler` and `api` plug into it, and `cmd/waterer` wires them together. `sim/` holds the simulator's physics and runner. It talks to the backend only over HTTP. The dashboard is a separate plan (`2026-10-16-dashboard.md`) that embeds into this binary.

**Tech Stack:** Go 1.23+, `modernc.org/sqlite`, `github.com/pressly/goose/v3` (Provider API), `github.com/oapi-codegen/oapi-codegen/v2` (std `net/http` strict server, embedded spec), `github.com/getkin/kin-openapi` (response validation in tests), `gopkg.in/yaml.v3`, `log/slog`, Docker/Compose.

**Spec:** [docs/plant-waterer-design.md](../../plant-waterer-design.md)

## Global Constraints

- Module path: `github.com/vlastrog/greenh`. The `go` directive is `1.23`. If a dependency requires newer Go, let `GOTOOLCHAIN=auto` fetch it and keep the Dockerfile builder image tag equal to the `go` directive.
- Backend builds with `CGO_ENABLED=0`. SQLite opens in WAL mode, and `goose` migrations are embedded and run on startup.
- Logging is `log/slog` with a JSON handler. API keys are never logged.
- Every API error is `{ "code", "message" }`, with optional `details: [{field, message}]` for 400s. Codes are `invalid_request`, `not_found`, `conflict` and `internal`.
- The API base path is `/api/v1`.
- Advisor config comes only from env: `ADVISOR_PROVIDER` (`openaicompat` | `rules`), `ADVISOR_BASE_URL`, `ADVISOR_MODEL`, `ADVISOR_API_KEY`. With no key, start in `rules` mode and log a warning.
- The LLM receives only the `core.Summary`: no names, locations, node IDs or pot IDs.
- Advisor timeout is 30 s per attempt with one retry, then the rules fallback with `fallback=true`.
- Reading `ts` may be at most 7 days in the past and 5 minutes in the future. If it's missing, use the receive time.
- `OFFLINE_AFTER` defaults to 30 min. The pot check interval defaults to 6 h. The scheduler wakes every 1 min.
- No real LLM calls in automated tests. The live test sits behind `-tags live` and the smoke test behind `-tags e2e`.
- The simulator's `speed` compresses physics only. Timestamps stay real.
- Time is always injected as `now func() time.Time`. No package-level mutable state. Prefer structs with constructor-injected dependencies.
- `make` isn't installed on the dev machine (Windows), so every Makefile target's command is also given in the plan to run directly.

## Review Focus

1. **Calibration edge values:** raw readings beyond the captured dry/wet values, or a reversed calibration (wet captured as the higher number), must give 0–100% and never fail. `dry_raw == wet_raw` must be rejected with 400. Tests are in Task 1 and Task 10.
2. **Relinking a channel that's already linked** to another pot must give 409 `conflict`, not a 500 from the unique index. The test is in Task 10.
3. **LLM answers wrapped in markdown fences or surrounded by prose** (common with DeepSeek and Ollama) must parse when a single JSON object is present. Truly invalid answers fall back to rules. The test is in Task 7.
4. **Nodes without ambient sensors** (no `ambient` in the payload) must still get summaries and rules advice, with ambient values as `null`. The test is in Task 3.
5. **"Check now" on a pot that has no link or has never reported** must return an advice record with status `sensor_offline`, not an error. The test is in Task 8.

---

## File Structure

```
go.mod, Makefile, .gitignore, .dockerignore, Dockerfile, docker-compose.yml, .env.example
core/        types.go  calibration.go  rules.go  precheck.go  validate.go  summary.go  downsample.go  store.go  errors.go  (+ *_test.go)
store/sqlite/  store.go  readings.go  pots.go  waterings.go  advice.go  migrations/00001_init.sql  sqlitetest/sqlitetest.go
ingest/      ingest.go
advisor/openaicompat/  client.go  prompt_v1.txt  live_test.go (-tags live)
scheduler/   checker.go  scheduler.go
api/         openapi.yaml  oapi.cfg.yaml  gen.go  api.gen.go (generated)  server.go  convert.go  errors.go  (+ server_test.go, testhelpers_test.go)
cmd/waterer/   main.go  config.go
sim/         config.go  env.go  soil.go  sensor.go  runner.go  client.go
cmd/simulator/ main.go
deploy/simulator.yaml
e2e/         smoke_test.go (-tags e2e)
```

---

### Task 1: Module scaffold and core domain types

**Files:**
- Create: `go.mod`, `.gitignore` (`/bin/`, `*.db*`, `.env`, `web/node_modules/`, `web/dist/*` except `web/dist/.gitkeep`), `Makefile`
- Create: `core/types.go`, `core/calibration.go`, `core/rules.go`, `core/errors.go`
- Test: `core/calibration_test.go`, `core/rules_test.go`

**Interfaces:**
- Produces (in `core/types.go`):
  ```go
  type Category string   // CategorySucculent="succulent", CategoryFoliage="foliage", CategoryFlowering="flowering", CategoryHerb="herb"
  type Status string     // StatusOK="ok", StatusSensorOffline="sensor_offline", StatusNeedsCalibration="needs_calibration"
  type Confidence string // ConfidenceLow="low", ConfidenceMedium="medium", ConfidenceHigh="high"
  type Trigger string    // TriggerScheduled="scheduled", TriggerManual="manual"

  type Ambient struct{ TempC, HumidityPct, LightLux *float64 }
  type ChannelReading struct{ Ch, MoistureRaw int }
  type Reading struct{ NodeID string; TS time.Time; Ambient Ambient; Channels []ChannelReading }
  type RawPoint struct{ TS time.Time; MoistureRaw int; Ambient Ambient }   // one channel's sample joined with its node's ambient
  type Rules struct{ ThresholdPct, AmountFraction float64 }
  type Pot struct {
      ID int64; Name, Species string; Category Category; PotML int; SoilType, Placement string
      NodeID *string; Channel *int; DryRaw, WetRaw *int
      Rules Rules; CheckInterval time.Duration; CreatedAt time.Time
  }
  func (p Pot) Calibrated() bool                      // both DryRaw and WetRaw set
  func (p Pot) MoisturePct(raw int) (float64, bool)   // false when not calibrated
  type Watering struct{ ID, PotID int64; TS, CreatedAt time.Time; AmountML int; Source string }
  type WateringLink struct{ Watering; NodeID *string; Channel *int }  // pot's current link
  type Recommendation struct {
      NeedsWater bool `json:"needs_water"`; AmountML int `json:"amount_ml"`
      Confidence Confidence `json:"confidence"`; Reason string `json:"reason"`
  }
  type ProviderInfo struct{ Provider, Model, PromptVersion string }
  type Advice struct {
      ID, PotID int64; CreatedAt time.Time; Trigger Trigger; Status Status
      Recommendation, Rules *Recommendation      // nil unless Status==ok
      ProviderInfo
      Fallback bool; FallbackReason string; Disagree bool
      Summary json.RawMessage                    // exact input summary, nil unless Status==ok
  }
  type NodeChannel struct{ Ch int; LastRaw *int; LastAt *time.Time; PotID *int64 }
  type Node struct{ ID, Name string; LastSeen time.Time; Channels []NodeChannel }
  ```
- Produces: `func MoisturePct(raw, dryRaw, wetRaw int) float64` (`core/calibration.go`) and `func DefaultRules(c Category) Rules`, `func (c Category) Valid() bool` (`core/rules.go`).
- Produces (`core/errors.go`): `var ErrNotFound`, `var ErrConflict`, and `type ValidationError struct{ Fields []FieldError }` with `type FieldError struct{ Field, Message string }`, implementing `error`.

- [ ] **Step 1: Init the module and Makefile**

Run `go mod init github.com/vlastrog/greenh` and set the `go 1.23` line. Makefile targets: `generate` (`go generate ./...`), `test` (`go test ./...`, extended by the dashboard plan), `build` (`CGO_ENABLED=0 go build -o bin/ ./cmd/...`), `up` (`docker compose up --build`), `e2e` (`go test -tags e2e ./e2e/...`), `image` (see Task 14).

- [ ] **Step 2: Write the failing tests**

```go
func TestMoisturePct(t *testing.T) {
    cases := []struct{ raw, dry, wet int; want float64 }{
        {540, 540, 270, 0}, {270, 540, 270, 100}, {405, 540, 270, 50},
        {600, 540, 270, 0}, {200, 540, 270, 100},      // beyond calibration clamps
        {405, 270, 540, 50}, {540, 270, 540, 100},     // reversed calibration still works
    }
    for _, c := range cases {
        if got := MoisturePct(c.raw, c.dry, c.wet); math.Abs(got-c.want) > 0.01 { t.Errorf("%+v: got %v", c, got) }
    }
}
func TestPotMoisturePctUncalibrated(t *testing.T) { _, ok := (Pot{}).MoisturePct(400); if ok { t.Fatal("want !ok") } }
func TestDefaultRules(t *testing.T) {
    want := map[Category]Rules{
        CategorySucculent: {10, 0.10}, CategoryFoliage: {30, 0.20},
        CategoryFlowering: {35, 0.20}, CategoryHerb: {40, 0.25},
    }
    for c, r := range want { if got := DefaultRules(c); got != r { t.Errorf("%s: %+v", c, got) } }
    if Category("cactus").Valid() { t.Error("unknown category valid") }
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./core/...`
Expected: FAIL (undefined identifiers).

- [ ] **Step 4: Implement the types, `MoisturePct`, `DefaultRules` and the errors**

`MoisturePct = (dry - raw) / (dry - wet) * 100`, clamped to [0, 100]. Callers guarantee `dry != wet` (pot validation in Task 10).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./core/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod Makefile .gitignore core/
git commit -m "feat(core): domain types, calibration, category rule defaults"
```

---

### Task 2: Pre-checks, answer validation, disagreement

**Files:**
- Create: `core/precheck.go`, `core/validate.go`
- Test: `core/precheck_test.go`, `core/validate_test.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces:
  ```go
  func Precheck(p Pot, lastReadingAt *time.Time, now time.Time, offlineAfter time.Duration) Status
  func ValidateRecommendation(r Recommendation, potML int) error
  func Disagree(a, b Recommendation) bool
  ```

- [ ] **Step 1: Write the failing tests**

```go
func TestPrecheck(t *testing.T) {
    now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
    cal := Pot{DryRaw: ptr(540), WetRaw: ptr(270)}
    recent, old := now.Add(-10*time.Minute), now.Add(-31*time.Minute)
    if s := Precheck(cal, nil, now, 30*time.Minute); s != StatusSensorOffline { t.Error(s) }
    if s := Precheck(cal, &old, now, 30*time.Minute); s != StatusSensorOffline { t.Error(s) }
    if s := Precheck(Pot{}, &recent, now, 30*time.Minute); s != StatusNeedsCalibration { t.Error(s) }
    if s := Precheck(Pot{}, &old, now, 30*time.Minute); s != StatusSensorOffline { t.Error("offline wins over calibration") }
    if s := Precheck(cal, &recent, now, 30*time.Minute); s != StatusOK { t.Error(s) }
}
func TestValidateRecommendation(t *testing.T) {
    ok := Recommendation{true, 250, ConfidenceMedium, "dry"}
    bad := []Recommendation{
        {true, 250, "sure", "x"},                    // confidence
        {false, 10, ConfidenceLow, "x"},             // !needs_water ⇒ 0
        {true, -1, ConfidenceLow, "x"},
        {true, 1001, ConfidenceLow, "x"},            // > half of 2000
        {true, 100, ConfidenceLow, strings.Repeat("a", 201)},
        {true, 100, ConfidenceLow, ""},
    }
    if err := ValidateRecommendation(ok, 2000); err != nil { t.Fatal(err) }
    for _, r := range bad { if ValidateRecommendation(r, 2000) == nil { t.Errorf("accepted %+v", r) } }
}
func TestDisagree(t *testing.T) {
    w := func(n bool, ml int) Recommendation { return Recommendation{NeedsWater: n, AmountML: ml} }
    if !Disagree(w(true, 200), w(false, 0)) { t.Error("needs_water differs") }
    if !Disagree(w(true, 100), w(true, 250)) { t.Error(">2x amount") }
    if Disagree(w(true, 200), w(true, 300)) || Disagree(w(false, 0), w(false, 0)) { t.Error("agree") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./core/...`
Expected: FAIL

- [ ] **Step 3: Implement the three functions**

- Validation rules: `confidence ∈ {low, medium, high}`, `!needs_water ⇒ amount_ml == 0`, `0 ≤ amount_ml ≤ potML/2`, and `1 ≤ len(reason) ≤ 200`. Return one error naming the first failed rule.
- Disagreement: `needs_water` differs, or both need water and `max/min amount > 2`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./core/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/
git commit -m "feat(core): prechecks, answer validation, disagreement"
```

---

### Task 3: Summarizer and rules advisor

**Files:**
- Create: `core/summary.go`, `core/advisor.go`
- Test: `core/summary_test.go`, `core/advisor_test.go`

**Interfaces:**
- Consumes: `Pot`, `RawPoint`, `Watering` (Task 1).
- Produces:
  ```go
  type SummaryInput struct{ Pot Pot; Now time.Time; Points []RawPoint; LastWatering *Watering }  // Points: last 48h, ascending
  type Summary struct {
      Pot          SummaryPot        `json:"pot"`
      Now          SummaryNow        `json:"now"`
      Trend        SummaryTrend      `json:"trend"`
      LastWatering *SummaryWatering  `json:"last_watering"`
      DataQuality  []string          `json:"data_quality"`   // never nil
  }
  type SummaryPot struct{ Species string `json:"species"`; Category Category `json:"category"`; PotML int `json:"pot_ml"`; SoilType string `json:"soil_type"`; Placement string `json:"placement"` }
  type SummaryNow struct{ MoisturePct float64; TempC, HumidityPct, LightLux *float64 }               // tags: moisture_pct, temp_c, humidity_pct, light_lux
  type SummaryTrend struct{ HourlyMoisturePct []*float64; DryingRatePctPerDay *float64; LightLuxHours24h float64 } // tags: hourly_moisture_pct, drying_rate_pct_per_day, light_lux_hours_24h
  type SummaryWatering struct{ HoursAgo float64; AmountML int; MoistureBeforePct, MoistureAfterPct *float64 }     // tags: hours_ago, amount_ml, moisture_before_pct, moisture_after_pct
  func Summarize(in SummaryInput) Summary   // requires in.Pot.Calibrated() and len(in.Points) > 0

  type AdviceRequest struct{ Summary Summary; Rules Rules }
  type Advisor interface {
      Info() ProviderInfo
      Advise(ctx context.Context, req AdviceRequest) (Recommendation, error)
  }
  type RulesAdvisor struct{}   // Info(): {Provider:"rules", Model:"rules-v1", PromptVersion:""}
  ```
  All percentages are rounded to 1 decimal place.

Rules advisor: water when `moisture < threshold`. The amount is `fraction × potML`, rounded to the nearest 10 ml. Confidence is `high` when `moisture < threshold − 5`, otherwise `medium`. When no water is needed, confidence is `high` and the amount is 0. Reasons: `"Moisture 18% is below the 30% threshold."` and `"Moisture 45% is at or above the 30% threshold."`

Algorithm decisions (not determined by the signature):
- **Now:** values from the last point. Ambient pointers pass through (nil stays nil).
- **Hourly:** 48 buckets `[now−48h+i·1h, now−48h+(i+1)·1h)`, oldest first, holding the mean moisture %. Empty buckets are `nil`.
- **Drying rate:** least-squares slope of moisture % over the last 24 h, using only points after `LastWatering.TS + 1h` when a watering exists. Report it as % lost per day (positive = drying). It's `nil` with fewer than 2 points or a span under 2 h.
- **Light lux-hours:** the sum over the last 24 hourly buckets of the mean lux × 1 h. Buckets with no lux are skipped.
- **Last watering:** `nil` when there's no watering. `before` is the last point before `TS`. `after` is the point closest to `TS + 1h` within `[TS, TS+2h]`. Either can be nil.
- **Data quality:** `"gap of N h in readings"` for each gap between consecutive points longer than 1 h (N rounded to whole hours, minimum 1), and `"less than 48 h of history"` when the first point is newer than `now − 47h`.

- [ ] **Step 1: Write the failing tests**

Build points with a helper `series(start time.Time, step time.Duration, pcts ...float64)`. It turns percentages into raw values using a pot calibrated at dry 540 / wet 270.

```go
func TestSummarizeNowAndProfile(t *testing.T)        // last point → Now.MoisturePct; Pot fields copied; JSON has no "name", "node_id", "id" keys
func TestSummarizeMissingAmbient(t *testing.T)       // points with zero Ambient → Now.TempC==nil; json contains `"temp_c":null`; LightLuxHours24h==0
func TestSummarizeHourlyBuckets(t *testing.T)        // 48 buckets; readings only in last 10h → first 38 nil, len==48
func TestSummarizeDryingRate(t *testing.T)           // linear 60→54 over 24h → rate 6.0 ±0.1
func TestSummarizeDryingRateIgnoresWateringJump(t *testing.T) // 30→20 over 12h, watering, jump to 50, then 50→47 over 11h → rate ≈ 6.5 ±0.5 (post-watering slope only)
func TestSummarizeLastWatering(t *testing.T)         // before == pct just before TS, after == pct near TS+1h, HoursAgo correct
func TestSummarizeDataQualityGap(t *testing.T)       // 3h hole → contains "gap of 3 h in readings"
func TestSummarizeShortHistory(t *testing.T)         // 5h of data → contains "less than 48 h of history"

func TestRulesAdvisor(t *testing.T) {
    req := func(pct float64) AdviceRequest {
        return AdviceRequest{Summary: Summary{Pot: SummaryPot{PotML: 2000}, Now: SummaryNow{MoisturePct: pct}}, Rules: Rules{30, 0.2}}
    }
    r, _ := RulesAdvisor{}.Advise(context.Background(), req(18))
    if !r.NeedsWater || r.AmountML != 400 || r.Confidence != ConfidenceHigh { t.Errorf("%+v", r) }
    r, _ = RulesAdvisor{}.Advise(context.Background(), req(27))
    if !r.NeedsWater || r.Confidence != ConfidenceMedium { t.Errorf("%+v", r) }
    r, _ = RulesAdvisor{}.Advise(context.Background(), req(45))
    if r.NeedsWater || r.AmountML != 0 { t.Errorf("%+v", r) }
    if err := ValidateRecommendation(r, 2000); err != nil { t.Error(err) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./core/ -run "Summarize|RulesAdvisor"`
Expected: FAIL

- [ ] **Step 3: Implement `Summarize` and `RulesAdvisor`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./core/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/
git commit -m "feat(core): reading summarizer and rules advisor"
```

---

### Task 4: SQLite store: open, migrations, readings and nodes

**Files:**
- Create: `core/store.go`, `store/sqlite/store.go`, `store/sqlite/readings.go`, `store/sqlite/migrations/00001_init.sql`, `store/sqlite/sqlitetest/sqlitetest.go`
- Test: `store/sqlite/readings_test.go`

**Interfaces:**
- Consumes: Task 1 types and errors.
- Produces (`core/store.go`):
  ```go
  type ReadingStore interface {
      InsertReading(ctx context.Context, r Reading) (stored bool, err error)   // false on duplicate (node_id, ts); also upserts node + channels, node.last_seen=max(last_seen, ts)
      History(ctx context.Context, nodeID string, ch int, from, to time.Time) ([]RawPoint, error) // ascending, inclusive from, exclusive to
      LastRaw(ctx context.Context, nodeID string, ch int) (*RawPoint, error)    // nil,nil if none
  }
  type NodeStore interface {
      ListNodes(ctx context.Context) ([]Node, error)         // channels sorted by ch; PotID from pots link
      RenameNode(ctx context.Context, id, name string) (Node, error)  // ErrNotFound
  }
  ```
- Produces: `func Open(ctx context.Context, path string) (*sqlite.Store, error)` (runs migrations), `func (s *Store) Close() error`, and `func sqlitetest.New(t *testing.T) *sqlite.Store` (a temp-file DB, closed via `t.Cleanup`).

Schema (`00001_init.sql`, goose `-- +goose Up/Down`). Timestamps are INTEGER unix milliseconds UTC:
- `nodes(id TEXT PK, name TEXT NOT NULL DEFAULT '', last_seen INTEGER NOT NULL)`
- `node_channels(node_id TEXT REFERENCES nodes(id), ch INTEGER, PRIMARY KEY(node_id, ch))`
- `readings(node_id TEXT, ts INTEGER, temp_c REAL, humidity_pct REAL, light_lux REAL, PRIMARY KEY(node_id, ts))`
- `channel_readings(node_id TEXT, ts INTEGER, ch INTEGER, moisture_raw INTEGER NOT NULL, PRIMARY KEY(node_id, ch, ts))`
- `pots(id INTEGER PK AUTOINCREMENT, name, species, category, pot_ml, soil_type, placement, node_id TEXT NULL, channel INTEGER NULL, dry_raw INTEGER NULL, wet_raw INTEGER NULL, threshold_pct REAL, amount_fraction REAL, check_interval_s INTEGER, created_at INTEGER)` plus `UNIQUE INDEX pots_link ON pots(node_id, channel) WHERE node_id IS NOT NULL`
- `waterings(id INTEGER PK AUTOINCREMENT, pot_id INTEGER REFERENCES pots(id) ON DELETE CASCADE, ts INTEGER, created_at INTEGER, amount_ml INTEGER, source TEXT)` plus an index on `created_at`
- `advice(id INTEGER PK AUTOINCREMENT, pot_id INTEGER REFERENCES pots(id) ON DELETE CASCADE, created_at INTEGER, trigger TEXT, status TEXT, recommendation TEXT NULL, rules TEXT NULL, provider TEXT, model TEXT, prompt_version TEXT, fallback INTEGER, fallback_reason TEXT, disagree INTEGER, summary TEXT NULL)` plus an index on `(pot_id, created_at)`. Recommendation columns hold JSON.

Open details: driver `"sqlite"`, DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)`, `db.SetMaxOpenConns(1)`. Migrations run through `goose.NewProvider(goose.DialectSQLite3, db, migrationsFS)` and `provider.Up(ctx)`, with no goose globals. Duplicate readings: `INSERT OR IGNORE` into `readings`. If no row was affected, skip the channel rows and return `false`. Do everything in one transaction.

- [ ] **Step 1: Write the failing tests**

```go
func TestInsertReadingAndHistory(t *testing.T)   // insert 3 readings (2 channels, ambient) → History(ch 1) returns 3 ascending points with ambient joined
func TestInsertReadingDuplicateIgnored(t *testing.T) // same (node, ts) twice, second has different raw → stored==false, history keeps first value
func TestInsertReadingOutOfOrder(t *testing.T)   // insert ts 3,1,2 → History ascending 1,2,3
func TestInsertReadingNilAmbient(t *testing.T)   // Ambient{} → RawPoint.Ambient.TempC == nil
func TestAutoRegisterNode(t *testing.T)          // first reading creates node with channels [0,1], Name "", LastSeen==ts; older backfill reading doesn't move LastSeen back
func TestLastRaw(t *testing.T)                   // nil for unknown channel; newest point otherwise
func TestRenameNode(t *testing.T)                // rename → ListNodes shows name; unknown id → errors.Is(err, core.ErrNotFound)
func TestOpenTwiceMigratesOnce(t *testing.T)     // Open same file twice → no error
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./store/...`
Expected: FAIL

- [ ] **Step 3: Implement `Open`, the migration and the reading/node methods**

`go get modernc.org/sqlite github.com/pressly/goose/v3`. `ListNodes` also fills `NodeChannel.LastRaw/LastAt` (newest channel reading) and `PotID` (from `pots`).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./store/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/store.go store/ go.mod go.sum
git commit -m "feat(store): sqlite store with migrations, readings and nodes"
```

---

### Task 5: SQLite store: pots, waterings, advice

**Files:**
- Modify: `core/store.go`
- Create: `store/sqlite/pots.go`, `store/sqlite/waterings.go`, `store/sqlite/advice.go`
- Test: `store/sqlite/pots_test.go`, `store/sqlite/waterings_test.go`, `store/sqlite/advice_test.go`

**Interfaces:**
- Produces (`core/store.go`):
  ```go
  type PotStore interface {
      CreatePot(ctx context.Context, p Pot) (Pot, error)   // ErrConflict if (node_id, channel) already linked
      GetPot(ctx context.Context, id int64) (Pot, error)   // ErrNotFound
      ListPots(ctx context.Context) ([]Pot, error)         // by id
      UpdatePot(ctx context.Context, p Pot) (Pot, error)   // full replace by p.ID; ErrNotFound / ErrConflict
      DeletePot(ctx context.Context, id int64) error       // ErrNotFound; cascades waterings + advice
  }
  type WateringStore interface {
      CreateWatering(ctx context.Context, w Watering) (Watering, error)
      ListWaterings(ctx context.Context, potID int64, limit int) ([]Watering, error) // newest first
      LastWatering(ctx context.Context, potID int64) (*Watering, error)              // by TS; nil if none
      WateringsSince(ctx context.Context, since time.Time) ([]WateringLink, error)   // created_at > since, ascending created_at
  }
  type AdviceStore interface {
      InsertAdvice(ctx context.Context, a Advice) (Advice, error)
      ListAdvice(ctx context.Context, potID int64, limit int) ([]Advice, error)     // newest first
      LatestAdvice(ctx context.Context, potID int64) (*Advice, error)               // nil if none
      LastAdviceTimes(ctx context.Context) (map[int64]time.Time, error)             // pot id → newest created_at
  }
  type Store interface{ ReadingStore; NodeStore; PotStore; WateringStore; AdviceStore }
  ```
  Map the SQLite unique-constraint error to `core.ErrConflict` (check the error text for `UNIQUE constraint failed`).

- [ ] **Step 1: Write the failing tests**

```go
func TestPotCRUD(t *testing.T)                 // create → get equal (incl. nil link/calibration, CheckInterval round-trip); list; update fields; delete → get ErrNotFound
func TestPotLinkConflict(t *testing.T)         // two pots on (n1, 0) → second ErrConflict; same on UpdatePot; two unlinked pots OK
func TestDeletePotCascades(t *testing.T)       // pot with watering + advice → delete → WateringsSince(zero) empty
func TestWateringsSince(t *testing.T)          // 2 waterings at created_at t1<t2; since t1 → only second, includes pot's NodeID/Channel; unlinked pot → nil link
func TestLastWateringByTS(t *testing.T)        // backdated watering created later but older TS → LastWatering returns the newer TS
func TestAdviceRoundTrip(t *testing.T)         // ok advice with both recs + summary JSON → LatestAdvice deep-equal; offline advice with nil recs round-trips nils
func TestLastAdviceTimes(t *testing.T)         // 2 pots, 3 advice → map has newest per pot
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./store/...`
Expected: FAIL

- [ ] **Step 3: Implement the methods**

Make `*sqlite.Store` satisfy `core.Store`, checked at compile time with `var _ core.Store = (*Store)(nil)`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./store/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/store.go store/
git commit -m "feat(store): pots, waterings and advice"
```

---

### Task 6: Ingest service

**Files:**
- Create: `ingest/ingest.go`
- Test: `ingest/ingest_test.go`

**Interfaces:**
- Consumes: `core.ReadingStore`, `core.ValidationError`.
- Produces:
  ```go
  type Service struct{ /* store core.ReadingStore; now func() time.Time; log *slog.Logger */ }
  func New(store core.ReadingStore, now func() time.Time, log *slog.Logger) *Service
  func (s *Service) Ingest(ctx context.Context, r core.Reading) (stored bool, err error)  // *core.ValidationError on bad input
  ```

Validation (each failure is a `FieldError` with a JSON path as the field):
- `node_id`: 1–64 chars matching `^[A-Za-z0-9_-]+$`.
- `ts`: zero means `now()`. More than 7 days in the past, or more than 5 min in the future, is an error.
- `channels`: at least 1. Each `ch` must be 0–15 and unique. Each `moisture_raw` must be 0–4095 (field `channels[1].moisture_raw`).
- `ambient` (when set): `temp_c` −40..85, `humidity_pct` 0..100, `light_lux` 0..200000.
- Log rejections at Warn with `node_id` and the reason.

- [ ] **Step 1: Write the failing tests** (use `sqlitetest.New`)

```go
func TestIngestStoresWithReceiveTimeWhenTSMissing(t *testing.T)
func TestIngestRejects(t *testing.T) // table: empty node_id, "a b", ts 8 days old, ts +6 min, no channels, ch 16, duplicate ch, raw 4096, raw -1, humidity 101 → *core.ValidationError with matching Field
func TestIngestAcceptsBoundaries(t *testing.T) // ts exactly now-7d+1s and now+5m; raw 0 and 4095
func TestIngestDuplicateReturnsNotStored(t *testing.T)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./ingest/...`
Expected: FAIL

- [ ] **Step 3: Implement `New` and `Ingest`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ingest/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ingest/
git commit -m "feat(ingest): validated reading ingestion"
```

---

### Task 7: OpenAI-compatible advisor

**Files:**
- Create: `advisor/openaicompat/client.go`, `advisor/openaicompat/prompt_v1.txt`, `advisor/openaicompat/live_test.go` (`//go:build live`)
- Test: `advisor/openaicompat/client_test.go`

**Interfaces:**
- Consumes: `core.Advisor`, `core.AdviceRequest`, `core.Recommendation`, `core.ProviderInfo`.
- Produces:
  ```go
  type Config struct{ BaseURL, Model, APIKey string; HTTPClient *http.Client }
  func New(cfg Config) *Client                 // nil HTTPClient → &http.Client{} (timeouts come from ctx)
  func (c *Client) Info() core.ProviderInfo    // {"openaicompat", cfg.Model, "v1"}
  func (c *Client) Advise(ctx context.Context, req core.AdviceRequest) (core.Recommendation, error)
  ```

Behavior:
- `POST {BaseURL}/chat/completions` (trim trailing `/` from BaseURL) with `Authorization: Bearer <key>` and body `{model, temperature: 0.2, messages: [{role: system, content: prompt_v1.txt}, {role: user, content: json(req.Summary)}]}`. `req.Rules` is not sent.
- Parse `choices[0].message.content`. Extract the JSON object: strip a ```` ```json ```` / ```` ``` ```` fence if present. Otherwise take the substring from the first `{` to the last `}`. Unmarshal into `core.Recommendation` with `DisallowUnknownFields` off. Return an error for a non-2xx status (include the status code, never the key), empty choices, or no JSON object. Range validation is the caller's job (Task 8).
- `prompt_v1.txt` is embedded with `//go:embed`. Content: the advisor role for houseplant watering. The user message is a JSON summary. Answer with only one JSON object `{"needs_water": bool, "amount_ml": int, "confidence": "low"|"medium"|"high", "reason": string ≤ 200 chars}`, with `amount_ml` 0 when no water is needed and at most half of `pot_ml`. Use the category, the trend and the last watering. Prefer not watering when unsure.

- [ ] **Step 1: Write the failing tests** (`httptest.NewServer` returning canned chat-completion JSON)

```go
func TestAdviseSendsSummaryOnly(t *testing.T)   // capture request: path /v1/chat/completions when BaseURL ".../v1/"; bearer header; user content == json(summary); system content starts with prompt text; body has no "threshold"
func TestAdviseParsesPlainJSON(t *testing.T)
func TestAdviseParsesFencedJSON(t *testing.T)   // "```json\n{...}\n```"
func TestAdviseParsesJSONWithProse(t *testing.T) // "Here is my answer: {...} Hope it helps."
func TestAdviseBadJSON(t *testing.T)            // "no idea" → error
func TestAdviseHTTP500(t *testing.T)            // error mentions 500, doesn't contain API key
func TestAdviseContextTimeout(t *testing.T)     // handler sleeps 200ms, ctx 50ms → error
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./advisor/...`
Expected: FAIL

- [ ] **Step 3: Implement the client and the prompt**

`live_test.go` reads `ADVISOR_BASE_URL/MODEL/API_KEY`, skips when the key is empty, sends a fixed summary and asserts `core.ValidateRecommendation(r, 2000) == nil`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./advisor/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add advisor/
git commit -m "feat(advisor): OpenAI-compatible provider with versioned prompt"
```

---

### Task 8: Checker (the shared check path)

**Files:**
- Create: `scheduler/checker.go`
- Test: `scheduler/checker_test.go`

**Interfaces:**
- Consumes: `core.Store` (Task 5), `core.Advisor`, `core.RulesAdvisor`, `core.Precheck`, `core.Summarize`, `core.ValidateRecommendation`, `core.Disagree`.
- Produces:
  ```go
  type CheckerConfig struct {
      Store core.Store; Advisor core.Advisor   // Advisor may itself be core.RulesAdvisor{}
      Now func() time.Time; OfflineAfter, AttemptTimeout time.Duration; Log *slog.Logger
  }
  func NewChecker(cfg CheckerConfig) *Checker
  func (c *Checker) Check(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, error)          // waits for the pot's lock
  func (c *Checker) TryCheck(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, bool, error)  // false if a check for this pot is running
  ```

Flow for one pot (under a per-pot `*sync.Mutex` held in a `map[int64]*sync.Mutex` guarded by a mutex):
1. `GetPot`. With no link, `last = nil`. Otherwise `LastRaw(node, ch)`.
2. `status := core.Precheck(pot, lastTS, now, OfflineAfter)`. If it isn't `ok`, store and return `Advice{Status: status, ProviderInfo: c.advisor.Info()}` with nil recommendations.
3. `History(node, ch, now−48h, now+1s)`, `LastWatering`, `Summarize`, and `req := AdviceRequest{summary, pot.Rules}`.
4. `rules := RulesAdvisor{}.Advise(req)`.
5. If the advisor is the rules provider (`Info().Provider == "rules"`), primary is `rules`. Otherwise make up to 2 attempts, each with `context.WithTimeout(ctx, AttemptTimeout)`. An attempt fails on an error or when `ValidateRecommendation(rec, pot.PotML)` fails. When both attempts fail, primary is `rules` with `Fallback = true` and `FallbackReason` set to the last error, truncated to 200 chars and logged at Warn. Log the advisor latency per attempt.
6. Store `Advice{Recommendation: primary, Rules: rules, Disagree: Disagree(primary, rules), Summary: json(summary), ...}`. `Disagree` is false when there's a fallback.

- [ ] **Step 1: Write the failing tests** (real `sqlitetest` store and a `fakeAdvisor{responses []func() (core.Recommendation, error), calls int}`)

```go
func TestCheckUnlinkedPotIsOffline(t *testing.T)     // Review Focus 5: no error, Status sensor_offline, advice persisted
func TestCheckNeverReportedIsOffline(t *testing.T)   // linked but no readings
func TestCheckNeedsCalibration(t *testing.T)
func TestCheckOKUsesLLM(t *testing.T)                // fake returns valid → Recommendation from fake, Rules set, Fallback false, Summary JSON has "pot", ProviderInfo from fake
func TestCheckRetriesOnceThenSucceeds(t *testing.T)  // first error, second valid → calls==2, Fallback false
func TestCheckFallsBackAfterTwoFailures(t *testing.T) // invalid amount twice → Fallback true, Recommendation==Rules, FallbackReason non-empty, calls==2
func TestCheckTimeoutFallsBack(t *testing.T)         // fake blocks until ctx done; AttemptTimeout 20ms
func TestCheckRulesProviderNoRetry(t *testing.T)     // Advisor=RulesAdvisor{} → Provider "rules", Fallback false
func TestCheckDisagree(t *testing.T)                 // moisture 18% (rules water), fake says no water → Disagree true
func TestTryCheckSkipsWhenRunning(t *testing.T)      // fake blocks on channel; TryCheck during Check → ok==false
func TestWateringChangesRecommendation(t *testing.T) // rules provider: dry readings → needs water; add watering + wet readings after → needs_water false (DoD 4 at unit level)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./scheduler/ -run Check`
Expected: FAIL

- [ ] **Step 3: Implement the `Checker`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./scheduler/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add scheduler/
git commit -m "feat(scheduler): checker with retry, rules fallback and per-pot locking"
```

---

### Task 9: Scheduler loop

**Files:**
- Create: `scheduler/scheduler.go`
- Test: `scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `core.PotStore`, `core.AdviceStore`, the `Checker` (through an interface).
- Produces:
  ```go
  type checkRunner interface{ TryCheck(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, bool, error) }
  type SchedulerConfig struct{ Pots core.PotStore; Advice core.AdviceStore; Checker checkRunner; Now func() time.Time; Tick time.Duration; Parallel int; Log *slog.Logger }
  func NewScheduler(cfg SchedulerConfig) *Scheduler   // Tick default 1m, Parallel default 4
  func (s *Scheduler) RunOnce(ctx context.Context)     // checks every due pot, returns when all finish
  func (s *Scheduler) Run(ctx context.Context)         // RunOnce immediately, then every Tick until ctx done
  ```
  A pot is due when it has no advice or `lastAdvice + pot.CheckInterval <= now`. Due pots run concurrently, bounded by `Parallel`. Each error is logged with `pot_id` and doesn't stop the others. Each pot runs at most one check per `RunOnce`, so overdue pots after downtime get exactly one.

- [ ] **Step 1: Write the failing tests** (sqlite store with a recording fake runner)

```go
func TestRunOnceChecksDuePots(t *testing.T)        // pot A no advice, pot B advice 7h ago (interval 6h), pot C advice 1h ago → runner called for A and B only, TriggerScheduled
func TestRunOnceErrorDoesNotBlockOthers(t *testing.T) // runner errors for A → B still checked
func TestRunOnceOverdueOnlyOnce(t *testing.T)       // pot overdue by 3 days → exactly one call
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./scheduler/ -run RunOnce`
Expected: FAIL

- [ ] **Step 3: Implement the `Scheduler`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./scheduler/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add scheduler/
git commit -m "feat(scheduler): periodic per-pot checks"
```

---

### Task 10: OpenAPI contract and API server

**Files:**
- Create: `api/openapi.yaml`, `api/oapi.cfg.yaml`, `api/gen.go` (`//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -config oapi.cfg.yaml openapi.yaml`), `api/api.gen.go` (generated, committed), `api/server.go`, `api/convert.go`, `api/errors.go`, `core/downsample.go`
- Test: `api/server_test.go`, `api/testhelpers_test.go`, `core/downsample_test.go`

**Interfaces:**
- Consumes: `core.Store`, `*ingest.Service`, `*scheduler.Checker` (`Check`), `core.ProviderInfo`.
- Produces:
  ```go
  type Deps struct{ Store core.Store; Ingest *ingest.Service; Checker interface{ Check(context.Context, int64, core.Trigger) (core.Advice, error) }; Advisor core.ProviderInfo; Now func() time.Time; Log *slog.Logger }
  func NewHandler(d Deps) http.Handler   // serves everything under /api/v1
  // core/downsample.go
  func Downsample(points []HistoryPoint, step time.Duration) []HistoryPoint
  ```
  `core.HistoryPoint{TS time.Time; MoistureRaw, MoisturePct, TempC, HumidityPct, LightLux *float64}`. `Downsample` groups into buckets aligned to `step` since the Unix epoch, uses the bucket start as `TS`, and takes per-field means over non-nil values. A step of 0 returns the input unchanged.

`oapi.cfg.yaml`: `package: api`, `generate: {std-http-server: true, strict-server: true, models: true, embedded-spec: true}`, `output-options: {nullable-type: true}`, `output: api.gen.go`. Mount it with `HandlerWithOptions(NewStrictHandlerWithOptions(srv, nil, StrictHTTPServerOptions{RequestErrorHandlerFunc, ResponseErrorHandlerFunc}), StdHTTPServerOptions{BaseURL: "/api/v1"})`. Both error handlers write the `Error` shape.

OpenAPI schemas (snake_case fields; `?` means optional; `null` means nullable):
- `Error{code, message, details?: [{field, message}]}`. Every operation declares `default: Error`, plus 400/404/409 where relevant.
- `ReadingIn{node_id, ts?, ambient?: {temp_c?, humidity_pct?, light_lux?}, channels: [{ch, moisture_raw}]}` → `202 {stored: bool}`.
- `Node{id, name, last_seen, channels: [{ch, last_raw: int|null, last_at: date-time|null, pot_id: int|null}]}`, `NodePatch{name}` (1–64 chars).
- `PotCreate{name (1–64), species?, category, pot_ml (50–50000), soil_type?, placement?, node_id?, channel?, dry_raw?, wet_raw?, threshold_pct?, amount_fraction?, check_interval_min?}`. A missing threshold or fraction comes from `core.DefaultRules`. `check_interval_min` defaults to 360.
- `PotPatch`: every PotCreate field optional. `node_id`, `channel`, `dry_raw` and `wet_raw` are nullable (null clears them).
- `Pot`: every PotCreate field required (nullable where it can be empty), plus `id`, `created_at`, `current{moisture_raw: int|null, moisture_pct: number|null, last_reading_at: date-time|null}` and `latest_advice: Advice|null`.
- `Recommendation{needs_water, amount_ml, confidence (enum), reason}`.
- `Advice{id, pot_id, created_at, trigger (enum), status (enum), recommendation: Recommendation|null, rules: Recommendation|null, provider, model, prompt_version, fallback, fallback_reason, disagree, summary: object|null}`.
- `AdviceList{latest: Advice|null, history: Advice[]}`. The history has at most 50 items, newest first.
- `HistoryPoint{ts, moisture_raw, moisture_pct, temp_c, humidity_pct, light_lux}` (each number|null).
- `Watering{id, pot_id, ts, created_at, amount_ml, source}`, `WateringIn{amount_ml (1–20000), ts?, source? = "manual"}`, `WateringFeedItem = Watering + node_id: string|null + channel: int|null`.
- `Health{status: "ok", advisor_provider, advisor_model}`.

Endpoints and handler rules:
- `POST /readings` → `Ingest`. A `ValidationError` becomes 400 `invalid_request` with details.
- `GET /nodes`, `PATCH /nodes/{id}`.
- `GET /pots`, `POST /pots` (201), `GET /pots/{id}`, `PATCH /pots/{id}`, `DELETE /pots/{id}` (204). Pot validation runs after merging the patch, returning 400 with details: category valid, `0 ≤ threshold_pct ≤ 100`, `0 < amount_fraction ≤ 0.5`, `15 ≤ check_interval_min ≤ 10080`, `channel` 0–15 and set only together with `node_id`, `dry_raw`/`wet_raw` both set or both null, `dry_raw != wet_raw` (message `"dry and wet calibration values must differ"`). `ErrConflict` on the link gives 409 `"node <id> channel <n> is already linked to another pot"`.
- `GET /pots/{id}/readings?from&to&step`: `from`/`to` default to now−24h and now. `step` is integer seconds, 0–86400, default 0. `to − from` may be at most 31 days. An unlinked pot returns `[]`. `moisture_pct` is null when the pot isn't calibrated.
- `GET/POST /pots/{id}/waterings`. POST validates `ts` like readings (≤ 7 d past, ≤ 5 min future) and sets `created_at = now`.
- `GET /waterings?since=`: `since` defaults to now−24h.
- `GET /pots/{id}/advice`, `POST /pots/{id}/advice` → `Checker.Check(…, TriggerManual)`, returning 200 with the Advice.
- `GET /health`.

- [ ] **Step 1: Write `openapi.yaml` and generate**

Run: `go generate ./api/...`
Expected: `api/api.gen.go` created, and `go build ./...` succeeds after adding a stub `server.go` that embeds `Unimplemented`-style methods.

- [ ] **Step 2: Write the failing tests**

`testhelpers_test.go`: `newTestAPI(t) (*httptest.Server, *sqlite.Store)` with a fixed clock, a sqlitetest store, a real ingest service and a real Checker using `core.RulesAdvisor{}`. `do(t, method, path string, body any) *http.Response` validates every response against `GetSwagger()` using `openapi3filter.ValidateResponse` and fails the test on a mismatch.

```go
func TestPostReadingsAndAutoRegister(t *testing.T)  // 202 stored:true; GET /nodes shows node with 2 channels, last_raw
func TestPostReadingsInvalid(t *testing.T)          // raw 5000 → 400, code invalid_request, details[0].field=="channels[0].moisture_raw"
func TestMalformedJSON(t *testing.T)                // "{" → 400 Error shape
func TestPotLifecycle(t *testing.T)                 // POST (defaults from category: foliage → 30, 0.2, 360) → GET → PATCH name → DELETE 204 → GET 404 not_found
func TestPotLinkConflict(t *testing.T)              // Review Focus 2: second pot on same node/channel → 409 conflict with message
func TestPotCalibrationValidation(t *testing.T)     // Review Focus 1: dry==wet → 400; only dry → 400; reversed (dry 270, wet 540) → 200
func TestPotPatchUnlink(t *testing.T)               // PATCH {"node_id":null,"channel":null} → link cleared
func TestPotCurrentAndRecalibration(t *testing.T)   // reading raw 405; calibrate 540/270 → current.moisture_pct 50; recalibrate 600/300 → 65 (history recomputed)
func TestPotReadingsHistoryStep(t *testing.T)       // 12 readings 5 min apart, step=3600 → 1 or 2 points averaged
func TestWateringsFeed(t *testing.T)                // POST watering → GET /waterings?since=… includes node_id/channel
func TestCheckNowAndAdviceList(t *testing.T)        // calibrated dry pot with fresh reading → POST advice status ok needs_water true; GET advice latest == that, history len 1
func TestHealth(t *testing.T)                       // advisor_provider "rules"
func TestDownsample(t *testing.T)                   // (core) means per bucket; nil fields ignored; step 0 → unchanged
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./api/... ./core/...`
Expected: FAIL

- [ ] **Step 4: Implement the server, converters, error mapping and `Downsample`**

`errors.go` maps `*core.ValidationError` → 400, `ErrNotFound` → 404, `ErrConflict` → 409, and anything else → 500 `internal` (logged; the response message is a generic "internal error").

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/ core/ go.mod go.sum
git commit -m "feat(api): OpenAPI contract and REST handlers"
```

---

### Task 11: `cmd/waterer` config and wiring

**Files:**
- Create: `cmd/waterer/config.go`, `cmd/waterer/main.go`
- Test: `cmd/waterer/config_test.go`

**Interfaces:**
- Consumes: `sqlite.Open`, `ingest.New`, `openaicompat.New`, `scheduler.NewChecker`, `scheduler.NewScheduler`, `api.NewHandler`.
- Produces:
  ```go
  type Config struct {
      Addr, DBPath string; OfflineAfter, AdvisorTimeout time.Duration
      Provider, BaseURL, Model, APIKey string
      Warnings []string   // e.g. no-key downgrade
  }
  func LoadConfig(getenv func(string) string) (Config, error)
  func (c Config) LogValue() slog.Value   // APIKey rendered as "set"/"unset"
  ```
  Env and defaults: `WATERER_ADDR=:8080`, `WATERER_DB=/data/waterer.db`, `OFFLINE_AFTER=30m`, `ADVISOR_TIMEOUT=30s`, `ADVISOR_PROVIDER=openaicompat`, `ADVISOR_BASE_URL=https://api.deepseek.com`, `ADVISOR_MODEL=deepseek-chat`, `ADVISOR_API_KEY` (no default).

`main` builds a JSON slog logger, loads the config (on error, print it to stderr and exit 1), logs the config and its warnings, and opens the store. It picks the advisor (`rules` → `core.RulesAdvisor{}`, otherwise `openaicompat.New`) and wires everything up. It mounts `/api/v1/` on an `http.ServeMux`, starts the scheduler goroutine and an `http.Server{ReadHeaderTimeout: 10s}`, and shuts down cleanly on SIGINT/SIGTERM via `signal.NotifyContext`. Close the store last.

- [ ] **Step 1: Write the failing tests**

```go
func TestLoadConfigDefaults(t *testing.T)       // empty env + key → defaults above
func TestLoadConfigNoKeyFallsBackToRules(t *testing.T) // openaicompat without key → Provider "rules", Warnings non-empty
func TestLoadConfigInvalid(t *testing.T)        // OFFLINE_AFTER="soon" → error naming OFFLINE_AFTER; ADVISOR_PROVIDER="gpt" → error
func TestConfigLogValueHidesKey(t *testing.T)   // slog JSON output doesn't contain the key string
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/waterer/...`
Expected: FAIL

- [ ] **Step 3: Implement `LoadConfig`, `LogValue` and `main`**

- [ ] **Step 4: Run the tests and a manual smoke check**

Run: `go test ./cmd/waterer/...`, then `WATERER_DB=./dev.db go run ./cmd/waterer` and in another shell `curl -s localhost:8080/api/v1/health`
Expected: PASS, `{"status":"ok","advisor_provider":"rules",...}`, and a warning log line about the missing key.

- [ ] **Step 5: Commit**

```bash
git add cmd/waterer/
git commit -m "feat(waterer): config loading and service wiring"
```

---

### Task 12: Simulator physics (env, soil, sensor)

**Files:**
- Create: `sim/env.go`, `sim/soil.go`, `sim/sensor.go`
- Test: `sim/env_test.go`, `sim/soil_test.go`, `sim/sensor_test.go`

**Interfaces:**
- Produces:
  ```go
  type Env struct{ /* rng *rand.Rand */ }
  func NewEnv(seed int64) *Env
  func (e *Env) At(t time.Time) core.Ambient          // all three fields set; t is simulated time, uses local-hour of t in UTC
  type Soil struct{ /* moisture, excess float64; category core.Category; potML int */ }
  func NewSoil(c core.Category, potML int, startPct float64) *Soil
  func (s *Soil) Step(dt time.Duration, a core.Ambient)  // dt in simulated time
  func (s *Soil) Water(ml int)
  func (s *Soil) Moisture() float64                     // 0..100
  type Sensor struct{ DryRaw, WetRaw int /* rng */ }
  func NewSensor(dryRaw, wetRaw int, seed int64) *Sensor
  func (s *Sensor) Raw(pct float64) int                 // inverted, ±1–2% noise, clamped 0..4095
  ```

Model constants:
- **Light:** `5000 × max(0, sin(π·(h−6)/12)) × cloud`, where h is the hour of day. `cloud` is 1 most of the time, with a 10% chance per hour-bucket of 0.3–0.7 (seeded per hour so it's stable within the hour).
- **Temperature:** `22 + 3·sin(π·(h−9)/12)` ±0.3 noise, which keeps it in 19–25 °C.
- **Humidity:** `50 − 4·(temp − 22)` ±1, clamped 0–100.
- **Drying (%/day at 22 °C, average daylight):** succulent 4, foliage 8, flowering 10, herb 12. Scale by `(1000/potML)^0.3`. Multiply by `1 + 0.05(temp−22) + light/10000 + (50−hum)/100`, and halve when light is 0.
- **Watering:** `rise = ml/potML × 150` percentage points. Moisture gains `rise × 1.3`, clamped to 100. Set `excess = rise × 0.3`. Each step drains `excess × (1 − exp(−dt/20min))`, which settles within about an hour.
- **Sensor:** `raw = dry − pct/100·(dry−wet)`, plus noise uniform in ±1.5% of `(dry−wet)`, rounded.

- [ ] **Step 1: Write the failing tests**

```go
func TestEnvDayNight(t *testing.T)            // 02:00 light 0; 12:00 light > 2000; temps over a day within [18.5, 25.5]; humidity higher at night than noon
func TestSoilDriesFasterInDayAndHeat(t *testing.T) // 1h at noon dries more than 1h at night
func TestSoilCategoryRates(t *testing.T)      // 24h steps of 5min: herb loses more than succulent; foliage 2000ml loses ~4–12 points
func TestSoilWateringOvershootThenSettles(t *testing.T) // 40%, water 300ml in 2000ml pot → immediately > 40+22.5; after 90 min ≈ 40+22.5 ±3 (minus drying)
func TestSoilStaysInRange(t *testing.T)       // water 5000ml → ≤100; dry 60 days → ≥0
func TestSensorInvertedAndNoisy(t *testing.T) // Raw(0)≈dry, Raw(100)≈wet within noise; Raw(20) > Raw(80); 100 samples not all equal
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./sim/...`
Expected: FAIL

- [ ] **Step 3: Implement the env, soil and sensor**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./sim/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add sim/
git commit -m "feat(sim): environment, soil and capacitive sensor models"
```

---

### Task 13: Simulator runner and `cmd/simulator`

**Files:**
- Create: `sim/config.go`, `sim/client.go`, `sim/runner.go`, `cmd/simulator/main.go`, `deploy/simulator.yaml`
- Test: `sim/config_test.go`, `sim/runner_test.go`

**Interfaces:**
- Consumes: Task 12 models and the API JSON shapes (the simulator posts its own small structs and doesn't import `api`, as an ESP32 wouldn't).
- Produces:
  ```go
  type Config struct {
      Backend string; Interval Duration; Speed float64; Backfill Duration; Seed int64
      Nodes []NodeConfig; Failures Failures
  }
  type NodeConfig struct{ ID string; Channels []ChannelConfig }
  type ChannelConfig struct{ Ch int; Category core.Category; PotML int; StartMoisture float64; DryRaw, WetRaw int }  // yaml: ch, category, pot_ml, start_moisture, dry_raw, wet_raw; 0 dry/wet → derived: dry 520+(ch*17)%40, wet 260+(ch*11)%30
  type Failures struct {
      Dropout    *struct{ Node string; After, For Duration } `yaml:"dropout"`       // real time since start
      Stuck      *struct{ Node string; Ch int } `yaml:"stuck"`
      OutOfRange *struct{ Node string; Ch, Every int } `yaml:"out_of_range"`      // every Nth reading sends raw 9999
  }
  type Duration struct{ time.Duration }   // YAML "30s"
  func LoadConfig(path string) (Config, error)   // defaults: interval 30s, speed 1, backfill 0; validates backend URL, ≥1 node, unique ch, known category, pot_ml>0
  type Backend interface {
      PostReading(ctx context.Context, r ReadingPayload) error
      WateringsSince(ctx context.Context, since time.Time) ([]WateringEvent, error)
  }
  type WateringEvent struct{ ID int64; NodeID *string `json:"node_id"`; Channel *int; AmountML int `json:"amount_ml"`; CreatedAt time.Time `json:"created_at"` }
  func NewHTTPBackend(baseURL string, hc *http.Client) Backend
  func NewRunner(cfg Config, b Backend, now func() time.Time, log *slog.Logger) *Runner
  func (r *Runner) Backfill(ctx context.Context) error   // speed 1 physics, step max(interval, 5m), real past timestamps
  func (r *Runner) Tick(ctx context.Context)             // one interval: poll waterings, step physics by interval×speed, flush buffer, post
  func (r *Runner) Run(ctx context.Context) error        // Backfill then Tick every Interval
  ```

Runner rules:
- **Backfill:** starts at `start_moisture` at `now − backfill`. Physics run at speed 1, so the history looks realistic. Readings are posted directly and failed posts go to the buffer.
- **Waterings:** keep a `since` cursor (start: runner start time minus 1 s) and a set of seen IDs. Apply only events whose `NodeID`/`Channel` match a configured channel, and ignore events with a nil link.
- **Buffer:** a FIFO of at most 10,000 payloads. Each tick resends buffered payloads oldest-first before posting the new one, and stops at the first failure. Drop payloads whose `ts` is older than `now − 7d + 1h`. A 4xx response other than 429 is dropped and logged (bad data never gets accepted), while network errors, 5xx and 429 are buffered.
- **Failures:** dropout skips posting (physics keep running) while inside `[start+After, start+After+For)`. Stuck repeats the first raw value produced after start. Out-of-range sends 9999 on every Nth reading of that channel.

`cmd/simulator/main.go`: a `-config` flag (default `/etc/simulator.yaml`), a JSON slog logger, `signal.NotifyContext`, and `Run`.

`deploy/simulator.yaml`: the spec's example config (backend `http://waterer:8080`, interval 30s, speed 60, backfill 48h, the 3 channels) with fixed `dry_raw`/`wet_raw` for e2e: ch0 540/270, ch1 530/265, ch2 550/280. No failures.

- [ ] **Step 1: Write the failing tests** (fake `Backend` that records posts and can fail on demand; fixed `now`)

```go
func TestLoadConfigSpecExample(t *testing.T)       // parses deploy/simulator.yaml; derived dry/wet when omitted
func TestLoadConfigInvalid(t *testing.T)           // unknown category, duplicate ch, empty nodes → error
func TestBackfillPostsHistory(t *testing.T)        // backfill 1h, interval 30s → step 5m → 12 or 13 posts with ascending ts in [now-1h, now]
func TestTickAppliesWatering(t *testing.T)         // event for (n1, ch0) 500ml → next payload raw for ch0 lower than before by a clear margin; ch1 unaffected
func TestTickIgnoresUnlinkedAndDuplicateWaterings(t *testing.T)
func TestBufferAndResend(t *testing.T)             // backend down for 3 ticks → 0 accepted; up → 4 posts in ts order
func TestBufferDrops4xx(t *testing.T)
func TestDropout(t *testing.T)                     // inside window → no posts; after → posts resume
func TestStuckChannel(t *testing.T)                // ch value identical across ticks, other channel varies
func TestOutOfRange(t *testing.T)                  // every 3rd reading for ch has raw 9999
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./sim/...`
Expected: FAIL

- [ ] **Step 3: Implement the config, HTTP backend, runner and main**

- [ ] **Step 4: Run the tests and a manual end-to-end check**

Run: `go test ./sim/...`. Then, with the waterer running from Task 11, make a copy of `deploy/simulator.yaml` with `backend: http://localhost:8080` and `backfill: 2h`, run `go run ./cmd/simulator -config <copy>`, and `curl -s localhost:8080/api/v1/nodes`.
Expected: PASS, and node `windowsill-1` with 3 channels and recent `last_raw`.

- [ ] **Step 5: Commit**

```bash
git add sim/ cmd/simulator/ deploy/
git commit -m "feat(sim): simulator runner with backfill, watering polling, buffering and failure modes"
```

---

### Task 14: Docker, Compose, e2e smoke test, docs

**Files:**
- Create: `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env.example`, `e2e/smoke_test.go` (`//go:build e2e`)
- Modify: `Makefile` (`image` target), `CLAUDE.md` (replace "Planned commands" with real ones and remove the Status section)

**Interfaces:**
- Consumes: both binaries, `deploy/simulator.yaml`, and the API from Task 10.

Details:
- **`Dockerfile`:** stage `build` uses `FROM --platform=$BUILDPLATFORM golang:<go directive>`, copies `go.mod`/`go.sum` first and runs `go mod download`, then `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w"` for both commands. Target `waterer` is `gcr.io/distroless/static-debian12:nonroot` with `/waterer`, `EXPOSE 8080`, and `ENV WATERER_DB=/data/waterer.db`. Target `simulator` is the same base with `/simulator` and `deploy/simulator.yaml` copied to `/etc/simulator.yaml`. (The dashboard plan adds a Node stage in front.)
- **`docker-compose.yml`:** service `waterer` (build target `waterer`, `env_file: .env` with `required: false`, port `8080:8080`, volume `waterer-data:/data`). Service `simulator` (build target `simulator`, `depends_on: waterer`, `restart: unless-stopped`). Named volume `waterer-data`. distroless `nonroot` runs as uid 65532, so the image must create `/data` owned by 65532 (copy an empty dir with `--chown=65532:65532` in the build stage) so the named volume inherits it.
- **`.env.example`:** the four `ADVISOR_*` variables with DeepSeek defaults and an empty key.
- **Makefile `image`:** `docker buildx build --platform linux/amd64,linux/arm64 --target waterer -t greenh/waterer .` (and the same for the simulator).
- **`e2e/smoke_test.go`:** `WATERER_URL` defaults to `http://localhost:8080`. It waits up to 60 s for `/nodes` to list `windowsill-1` with 3 channels, then creates a pot on ch0 (foliage, 2000 ml, calibrated 540/270) and asserts the readings history over the last 48 h has ≥ 100 points (backfill). It runs `POST /pots/{id}/advice` and expects status `ok`. It notes `current.moisture_pct`, posts a 400 ml watering, and polls `GET /pots/{id}` every 5 s for up to 120 s until moisture rose by ≥ 10 points. It then runs a check again and expects advice that differs from the first, or `needs_water: false`. Finally it deletes the pot.

- [ ] **Step 1: Write the Dockerfile, compose file, .env.example and .dockerignore**

- [ ] **Step 2: Bring the stack up**

Run: `docker compose up --build -d`, then `curl -s localhost:8080/api/v1/nodes`
Expected: both containers running, and `windowsill-1` listed within about 30 s.

- [ ] **Step 3: Write the e2e smoke test and run it against the stack**

Run: `go test -tags e2e ./e2e/... -v -count=1`
Expected: PASS

- [ ] **Step 4: Run the full test suite and tear down**

Run: `go test ./...`, then `docker compose down`
Expected: PASS

- [ ] **Step 5: Update CLAUDE.md commands**

List the real commands, with Windows equivalents where `make` is missing: `go test ./...`, a single test `go test ./core -run TestSummarize -v`, `go generate ./api/...`, `docker compose up --build`, `go test -tags e2e ./e2e/...`, and `go test -tags live ./advisor/openaicompat/...`.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile .dockerignore docker-compose.yml .env.example e2e/ Makefile CLAUDE.md
git commit -m "feat: docker packaging, compose stack and e2e smoke test"
```
