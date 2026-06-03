# Repository Guidelines

## Project Structure & Module Organization
`cmd/server` contains the main HTTP service. `cmd/promptdump` is a small utility for exporting analyzer prompts from an existing demo. Core application code lives under `internal/`: `api` for routes, `orchestrator` for async job flow, `parser` for `.dem` parsing, `analyzer` for LLM-driven reporting, `storage` for SQLite and file persistence, `prokb` for benchmark data, `config` for env-based settings, and `domain` for shared types. Static frontend assets are served from `web/`. Runtime data defaults to `data/`.

## Build, Test, and Development Commands
Use Go 1.24 as declared in `go.mod`.

- `go run ./cmd/server` starts the API and serves `web/` on `HTTP_ADDR` (default `:8080`).
- `go test ./...` runs all unit tests across `internal/...`.
- `go build ./cmd/server` builds the server binary for validation or release packaging.
- `go run ./cmd/promptdump -api http://localhost:8080 -id <demo-id>` dumps the generated analyzer prompt for a processed demo.
- `go mod tidy` syncs module dependencies after import changes.

## Coding Style & Naming Conventions
Follow standard Go formatting: tabs for indentation, `gofmt` formatting, and idiomatic package organization. Keep package names short and lowercase (`parser`, `storage`). Exported identifiers use `PascalCase`; unexported helpers use `camelCase`. Prefer small focused files inside `internal/` and keep HTTP, parsing, analysis, and persistence concerns separated by package.

## Testing Guidelines
Tests use Go's built-in `testing` package. Place tests next to implementation files with `_test.go` suffix, as in `internal/parser/zones_test.go`. Name tests `TestXxx` with behavior-oriented cases such as `TestBuildOfflineRoundAnalysisEarlyDeath`. Add targeted unit coverage for parser edge cases, analyzer normalization/repair logic, and offline round analysis behavior before merging changes.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit style with scopes, for example `feat(analyzer): ...` and `fix(llm): ...`. Keep commits focused and use a scope that matches the package or surface changed. Pull requests should include a short summary, affected packages, test results from `go test ./...`, and screenshots when `web/` output changes. Link the relevant issue or task when one exists.

## Security & Configuration Tips
Configure the service through environment variables, not hardcoded secrets. Common settings include `LLM_PROVIDER`, `LLM_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `LLM_MODEL`, `DATA_DIR`, and `SQLITE_PATH`. Do not commit API keys, demo data, or generated SQLite files unless they are intentional fixtures.
