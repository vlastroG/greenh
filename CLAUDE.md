# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

A hobby automatic plant waterer: for each pot, say whether it needs water and how much. Software only for now. Hardware is simulated, and the UI is a web dashboard.

## Status

Backend and simulator are implemented. The React dashboard is a separate upcoming plan.

## Commands

`make` may be absent on Windows; each target is a plain command.

- `docker compose up --build` starts `waterer` (API on :8080) and `simulator`. Reads an optional `.env` (see `.env.example`).
- `go test ./...` (`make test`) runs all tests. Automated tests never call a real LLM.
- `go test ./core -run TestSummarize -v` runs a single test.
- `go generate ./api/...` regenerates Go types from `api/openapi.yaml`.
- `go test -tags e2e ./e2e/... -v -count=1` (`make e2e`) runs the smoke test against a running compose stack (`WATERER_URL`, default `http://localhost:8080`).
- `go test -tags live ./advisor/openaicompat/...` runs the optional real LLM call (needs `ADVISOR_API_KEY`).
- `make image` builds multi-arch images with buildx.

## Architecture

- **Two Go programs, one module:** `cmd/waterer` is the backend: API, embedded React dashboard, scheduler and SQLite. `cmd/simulator` acts as an ESP32 node with soil attached. The backend has no simulation code. The simulator learns about waterings only by polling `GET /api/v1/waterings?since=`.
- **One ingestion path:** `POST /api/v1/readings` → `ingest.Ingest`. The HTTP handler and the future serial reader (Arduino Uno, JSON lines) are thin callers of it.
- **`core/` is pure logic with no I/O.** It holds the domain types, summarizer, `Advisor` interface, rule-based advisor, calibration and offline detection. `store/sqlite` implements interfaces defined in `core`.
- **Contract-first API:** `api/openapi.yaml` generates Go types (`oapi-codegen`) and TS types (`openapi-typescript`). It's also the contract for a future Swift app, so change the spec first, then regenerate.
- **Raw readings are stored as received.** Moisture % is computed when read, using the pot's `dry_raw`/`wet_raw` calibration, so recalibrating fixes past history too. Sensors report inverted values (drier = higher raw).
- **Node / channel / pot:** a pot links to exactly one `(node_id, channel)`. An unknown `node_id` is auto-registered with unlinked channels.
- **Advisor pipeline:** pre-checks in `core` (`sensor_offline`, `needs_calibration`) run first. Then the LLM (`openaicompat`, DeepSeek first) gets an anonymized summary. Our code validates every answer. On a failure it retries once, then falls back to rules with the `fallback` flag set. The rules advisor runs on every check, and its result is stored next to the LLM's for the "disagree" marker. Each advice record stores provider, model, prompt version and the exact input summary. The system prompt lives in one versioned file.
- **Scheduler:** wakes every minute and runs per-pot checks. It never runs two checks for the same pot at once, one pot's failure doesn't block the others, and after downtime each overdue pot gets a single check.

## Constraints

- Go with `CGO_ENABLED=0`, so SQLite goes through `modernc.org/sqlite` (WAL mode). `goose` migrations are embedded and run on startup.
- Logging is `log/slog` JSON. Never log API keys.
- Advisor config comes only from env vars or a config file, never from the API: `ADVISOR_PROVIDER` (`openaicompat` | `rules`), `ADVISOR_BASE_URL`, `ADVISOR_MODEL`, `ADVISOR_API_KEY`. With no key, the backend starts in `rules` mode.
- Only the pot profile and a reading summary are sent to the LLM. No names, locations or IDs.
- Every API error has the shape `{ "code", "message" }`.
- Simulator `speed` compresses only the physics. Timestamps stay real, so don't add fake clocks.
- Packaging: multi-stage Docker build onto a distroless base, multi-arch (`amd64`, `arm64`).
