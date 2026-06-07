# Uploadfile Quota And Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend upload quota enforcement and document the lifecycle cleanup contract.

**Architecture:** Add repository usage stats for global and owner scopes, then enforce service-level byte/file limits before multipart writes and presigned upload creation. Load quota limits from uploadfile env config and pass them through module registration into `service.Options`.

**Tech Stack:** Go, FBA Go uploadfile plugin, memory repository, GORM repository, `go.local.mod` tests.

---

### Task 1: Config Quota Tests

**Files:**
- Modify: `plugins/uploadfile/config/config_test.go`
- Modify: `plugins/uploadfile/config/config.go`

- [x] **Step 1: Write failing config test**

Add quota keys to `TestLoadReadsDotenvAndAppliesLocalSeed` and assert:

```go
if opts.MaxTotalBytes != 5555 || opts.MaxOwnerBytes != 4444 || opts.MaxTotalFiles != 33 || opts.MaxOwnerFiles != 22 {
	t.Fatalf("quota options = %+v, want configured quota limits", opts)
}
```

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/config -run TestLoadReadsDotenvAndAppliesLocalSeed -count=1
```

Expected: compile failure because the quota fields do not exist yet.

- [x] **Step 3: Implement config parsing**

Add the four fields to `config.Options`, parse the four env keys with `int64Value`, and add them to `uploadfileEnvKeys`.

### Task 2: Repository Usage Stats

**Files:**
- Modify: `plugins/uploadfile/repo/repository.go`
- Modify: `plugins/uploadfile/repo/memory.go`
- Modify: `plugins/uploadfile/repo/gorm.go`
- Modify: `plugins/uploadfile/repo/memory_test.go`
- Modify: `plugins/uploadfile/repo/gorm_test.go`

- [x] **Step 1: Write failing memory/GORM tests**

Add tests named:

```go
TestMemoryRepositoryUploadUsageCountsLiveObjectsAndDistinctOwnerRefs
TestGORMRepositoryUploadUsageCountsLiveObjectsAndDistinctOwnerRefs
```

The tests create live, pending, and deleted objects. They assert global usage includes live and pending objects, owner usage ignores deleted refs, and duplicate live refs for the same file are counted once.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/repo -run 'Test(Memory|GORM)RepositoryUploadUsage' -count=1
```

Expected: compile failure because `UploadUsage` does not exist yet.

- [x] **Step 3: Implement repository stats**

Add:

```go
type UsageFilter struct {
	OwnerType string
	OwnerID   string
}

type UsageStats struct {
	Files int64
	Bytes int64
}
```

Add `UploadUsage(ctx context.Context, filter UsageFilter) (UsageStats, error)` to the repository interface and implement it in memory and GORM repositories.

### Task 3: Service Quota Enforcement

**Files:**
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/service/service_test.go`

- [x] **Step 1: Write failing service tests**

Add tests for:

- total byte/file quota rejecting multipart upload before backend `Put`;
- owner byte/file quota rejecting presigned direct upload before backend `PresignPut`.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run 'TestServiceRejects.*Quota' -count=1
```

Expected: compile failure because service quota options do not exist yet.

- [x] **Step 3: Implement service quota checks**

Add quota fields to `service.Options` and `Service`, resolve owner before storage operations, and call a helper that checks total and owner usage using repository stats.

### Task 4: Module Wiring And Runtime Docs

**Files:**
- Modify: `plugins/uploadfile/module.go`
- Modify: `plugins/uploadfile/plugin_test.go`
- Modify: `docs/superpowers/specs/2026-06-08-uploadfile-api-contract.md`
- Modify: `env.tmpl`

- [x] **Step 1: Write failing module wiring test**

Add a plugin test that sets `UPLOADFILE_MAX_TOTAL_BYTES=4`, resolves the admin upload backend, and verifies a 5-byte upload is rejected.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile -run TestUploadfilePluginAppliesQuotaEnvToAdminUploadBackend -count=1
```

Expected: test fails because module options are not wired yet.

- [x] **Step 3: Wire module options and docs**

Map config quota fields into `service.Options`. Add env keys to `env.tmpl` and runtime API contract docs.

### Task 5: Verification And Commit

**Files:**
- All files above.

- [x] Run `gofmt -w plugins/uploadfile`.
- [x] Run `git diff --check`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./...`.
- [x] Commit only uploadfile quota/lifecycle files; leave unrelated `.gitignore` untouched.
