# AGENTS.md

## Local Go Commands

- Local testing and running must use `go.local.mod`.
- Prefer Makefile targets with `L=1`:
  - `make L=1 test`
  - `make L=1 run`
  - `make L=1 dev`
- When invoking Go directly, pass `-modfile=go.local.mod` and keep `GOWORK=off`:
  - `GOWORK=off go test -modfile=go.local.mod ./...`
  - `GOWORK=off go run -modfile=go.local.mod ./cmd/api`
