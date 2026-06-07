# Uploadfile Env Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `UPLOADFILE_*` environment configuration for uploadfile default seed storage and scene data.

**Architecture:** Keep runtime configuration in `upload_storage` and `upload_scene`. Add a focused uploadfile config package that reads `.env` plus process env, applies values to `repo.Seed`, and is called by `Module.Register` before constructing memory repositories or migrations.

**Tech Stack:** Go standard library dotenv parsing, uploadfile repo/model/migration packages, plugin runtime, GORM SQLite tests, `go.local.mod` verification.

---

## File Structure

- Create `plugins/uploadfile/config/config.go`: load `UPLOADFILE_*` values and apply them to `repo.Seed`.
- Create `plugins/uploadfile/config/config_test.go`: loader and seed override tests.
- Modify `plugins/uploadfile/module.go`: load configured seed and pass it to memory repository and migration.
- Modify `plugins/uploadfile/migration/migration.go`: allow initial-data migration to receive configured seed.
- Modify `plugins/uploadfile/repo/gorm_test.go`: update default initial-data call if needed.
- Modify `plugins/uploadfile/plugin_test.go`: verify configured seed reaches memory registration and migration seed closure.

## Task 1: Config Loader And Seed Application

**Files:**
- Create: `plugins/uploadfile/config/config.go`
- Test: `plugins/uploadfile/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Add tests that create a temporary dotenv file and assert:

```go
opts, err := config.Load(config.LoadOptions{EnvFile: envFile})
seed, err := config.ApplyToSeed(repo.SeedData(), opts)
```

The configured seed must set local root JSON, S3 bucket/region/base URL, default scene max size/TTL, and process env must override dotenv.

- [ ] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/config
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement loader**

Implement a small dotenv reader, real env override merge, boolean/int parsing, provider validation, JSON-array normalization, and seed application.

- [ ] **Step 4: Verify green**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/config
```

Expected: PASS.

## Task 2: Module And Migration Wiring

**Files:**
- Modify: `plugins/uploadfile/module.go`
- Modify: `plugins/uploadfile/migration/migration.go`
- Test: `plugins/uploadfile/plugin_test.go`
- Test: `plugins/uploadfile/repo/gorm_test.go`

- [ ] **Step 1: Write failing wiring tests**

Add plugin/migration tests that set `FBA_ENV_FILE` to a temp dotenv file and verify:

- `Module.Register` with memory repository exposes configured default storage through the service handler path or injected repo hook.
- `InitialData(provider, configuredSeed)` inserts configured default storage and default scene.

- [ ] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile ./plugins/uploadfile/repo
```

Expected: FAIL because module and migration still use `repo.SeedData()` directly.

- [ ] **Step 3: Wire configured seed**

In `Module.Register`, load uploadfile config, apply it to `repo.SeedData()`, and use the result for memory repository and `uploadmigration.InitialData(provider, seed)`.

In migration, change initial data to accept optional seed and default to `repo.SeedData()` when none is provided.

- [ ] **Step 4: Verify green**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile ./plugins/uploadfile/repo
```

Expected: PASS.

## Task 3: Final Verification

- [ ] **Step 1: Run uploadfile tests**

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...
```

- [ ] **Step 2: Run full local tests**

```bash
make L=1 test
```

- [ ] **Step 3: Run full build**

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go build -modfile=go.local.mod ./...
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-06-07-uploadfile-env-config-design.md docs/superpowers/plans/2026-06-07-uploadfile-env-config.md plugins/uploadfile
git commit -m "feat: configure uploadfile defaults from env"
```
