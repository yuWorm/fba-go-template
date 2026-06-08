# Uploadfile Stats API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a backend stats endpoint for uploadfile capacity usage.

**Architecture:** Extend the existing repository `UploadUsage` filter, add a service method that applies owner scope, then expose it through a new authenticated route and handler. Keep the response compact as `files` and `bytes`.

**Tech Stack:** Go, FBA Go plugin routes, Fiber handlers, memory/GORM uploadfile repositories, `go.local.mod` tests.

---

### Task 1: Repository Stats Filters

**Files:**
- Modify: `plugins/uploadfile/repo/repository.go`
- Modify: `plugins/uploadfile/repo/memory.go`
- Modify: `plugins/uploadfile/repo/gorm.go`
- Modify: `plugins/uploadfile/repo/memory_test.go`
- Modify: `plugins/uploadfile/repo/gorm_test.go`

- [x] **Step 1: Write failing repository filter tests**

Add tests that call `UploadUsage` with `SceneCode`, `StorageCode`, `Provider`, `Status`, and owner filters. Assert deleted objects and deleted refs stay excluded and duplicate refs do not double count objects.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/repo -run 'Test(Memory|GORM)RepositoryUploadUsageFilters' -count=1
```

Expected: compile failure because the new filter fields do not exist yet.

- [x] **Step 3: Implement repository filters**

Add `SceneCode`, `Provider`, `StorageCode`, and `Status` to `repo.UsageFilter`; apply object filters directly and ref filters through a distinct file-id match.

### Task 2: Service Stats Scope

**Files:**
- Modify: `plugins/uploadfile/dto/dto.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/service/service_test.go`

- [x] **Step 1: Write failing service tests**

Add tests that verify:

- a normal user stats request with no owner filter returns only their owner usage;
- a normal user stats request with a foreign owner filter returns zero;
- a super admin can query global usage.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run 'TestServiceUploadStats' -count=1
```

Expected: compile failure because `UploadStats` does not exist yet.

- [x] **Step 3: Implement service method**

Add `dto.UploadStats` with `files` and `bytes`, then add `Service.UploadStats(ctx, filter, actor)`.

### Task 3: API Route And Contract

**Files:**
- Modify: `plugins/uploadfile/api/routes.go`
- Modify: `plugins/uploadfile/api/handler.go`
- Modify: `plugins/uploadfile/api/handler_test.go`
- Modify: `plugins/uploadfile/plugin_test.go`
- Modify: `docs/superpowers/specs/2026-06-08-uploadfile-api-contract.md`

- [x] **Step 1: Write failing API tests**

Add API tests for:

- `GET /api/v1/sys/upload/stats` response;
- normal user owner scoping;
- route registration metadata.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/api ./plugins/uploadfile -run 'TestUploadAPIStats|TestUploadfilePluginRegistersRoutes' -count=1
```

Expected: failures because the route and handler do not exist.

- [x] **Step 3: Implement route and handler**

Add `Handler.UploadStats`, register `GET /sys/upload/stats`, and document it in the API contract.

### Task 4: Verification And Commit

**Files:**
- All files above.

- [x] Run `gofmt -w plugins/uploadfile`.
- [x] Run `git diff --check`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./...`.
- [x] Commit only stats API files; leave unrelated `.gitignore` untouched.
