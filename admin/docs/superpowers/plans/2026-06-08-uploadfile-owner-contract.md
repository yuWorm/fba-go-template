# Uploadfile Owner Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lock uploadfile owner semantics so foreign owner filters return empty results and deleted refs no longer authorize access or scene/owner object queries.

**Architecture:** Keep owner data on `upload_file_ref` and uploaded-user fallback on `upload_file_object.uploaded_by`. Implement scope decisions in `plugins/uploadfile/service/owner.go` and `service.go`, and make repository object filters ignore deleted refs in both memory and GORM implementations.

**Tech Stack:** Go, FBA Go plugin service/repo tests, memory repository, GORM repository, `go.local.mod` verification.

---

### Task 1: Service Owner Scope Tests

**Files:**
- Modify: `plugins/uploadfile/service/service_test.go`

- [x] **Step 1: Write failing tests**

Add tests for:

- normal users passing `OwnerID` for another user get an empty file/ref list instead of their own files;
- a deleted ref no longer authorizes file access when the actor is not `uploaded_by`.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run 'TestService(ListFilesWithForeignOwnerFilterReturnsEmpty|DeletedRefDoesNotAuthorizeOwnerAccess)' -count=1
```

Expected: failures showing current owner scoping rewrites foreign owner filters and deleted refs still authorize access.

### Task 2: Repository Deleted Ref Filter Tests

**Files:**
- Modify: `plugins/uploadfile/repo/memory_test.go`
- Modify: `plugins/uploadfile/repo/gorm_test.go`

- [x] **Step 1: Write failing tests**

Add tests that `ListObjects` with scene/owner filters ignores refs with `status=deleted`.

- [x] **Step 2: Verify red**

Run:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/repo -run 'Test(Memory|GORM).*DeletedRef' -count=1
```

Expected: failures showing deleted refs currently match object filters.

### Task 3: Implement Owner Scope Rules

**Files:**
- Modify: `plugins/uploadfile/service/owner.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/repo/memory.go`
- Modify: `plugins/uploadfile/repo/gorm.go`

- [x] **Step 1: Implement service owner helpers**

Add owner filter scoping helpers so non-super-admin actors:

- get their default owner when no owner filter is supplied;
- get an impossible owner filter when they supply a foreign owner;
- do not receive access from deleted refs.

- [x] **Step 2: Implement repository filtering**

Make object-list ref matching ignore `model.RefStatusDeleted` in memory and GORM repositories.

- [x] **Step 3: Verify green**

Run service and repo targeted tests until they pass.

### Task 4: Verification And Commit

**Files:**
- Modify: `docs/superpowers/specs/2026-06-08-uploadfile-owner-contract.md`
- Modify: `docs/superpowers/plans/2026-06-08-uploadfile-owner-contract.md`

- [x] Run `gofmt -w plugins/uploadfile/service plugins/uploadfile/repo`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./...`.
- [x] Run `git diff --check`.
- [x] Commit only owner contract files and uploadfile changes; leave unrelated `.gitignore` untouched.
