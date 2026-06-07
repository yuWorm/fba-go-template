# Uploadfile Lifecycle Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden uploadfile private access and direct-upload lifecycle by capping private file access tokens, validating direct-upload completion against storage metadata, and cleaning stale pending objects.

**Architecture:** Keep lifecycle policy in `plugins/uploadfile/service`, storage metadata probing in `plugins/uploadfile/storage`, and pending-object lookup in `plugins/uploadfile/repo`. Reuse the existing cleanup command and scheduled task so expired temp refs and stale direct uploads are handled by one maintenance path.

**Tech Stack:** Go, FBA Go plugin APIs, memory/GORM repositories, local/S3/OSS storage backends, targeted `go test` with `go.local.mod`.

---

### Task 1: Access Token TTL Limit

**Files:**
- Modify: `plugins/uploadfile/service/service.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [x] **Step 1: Write the failing test**

Add a service test that configures `FileAccessTokenMaxTTL`, rejects a token request above that max, and still accepts a request at the max.

- [x] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run TestServiceRejectsFileAccessTokenTTLAboveMax -count=1`

Expected: compile failure because `service.Options.FileAccessTokenMaxTTL` is not defined.

- [x] **Step 3: Implement the TTL cap**

Add `FileAccessTokenMaxTTL` to service options, default it to one hour, store it on `Service`, and return a bad request when a requested TTL exceeds the max.

- [x] **Step 4: Verify**

Run the same targeted test and expect PASS.

### Task 2: Direct Upload Completion Head Validation

**Files:**
- Modify: `plugins/uploadfile/storage/backend.go`
- Modify: `plugins/uploadfile/storage/local.go`
- Modify: `plugins/uploadfile/storage/s3.go`
- Modify: `plugins/uploadfile/storage/oss.go`
- Modify: `plugins/uploadfile/service/service.go`
- Test: `plugins/uploadfile/service/service_test.go`
- Test: `plugins/uploadfile/storage/s3_test.go`
- Test: `plugins/uploadfile/storage/oss_test.go`

- [x] **Step 1: Write the failing test**

Add service tests proving `CompletePresignedUpload` calls backend `Head` and rejects missing objects or size mismatches before activating pending records.

- [x] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run 'TestService(CompletesPresignedUploadAfterHeadValidation|RejectsPresignedUploadCompletionWhenStorageMetadataDiffers)' -count=1`

Expected: compile failure because `storage.Backend.Head` is not defined yet.

- [x] **Step 3: Implement metadata probing**

Add `Head(ctx, key)` to the backend interface and all backends. Validate pending uploads by comparing recorded size with `Head` size before marking objects active.

- [x] **Step 4: Verify**

Run service and storage backend package tests and expect PASS.

### Task 3: Stale Pending Upload Cleanup

**Files:**
- Modify: `plugins/uploadfile/repo/repository.go`
- Modify: `plugins/uploadfile/repo/memory.go`
- Modify: `plugins/uploadfile/repo/gorm.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/dto/dto.go`
- Modify: `plugins/uploadfile/module.go`
- Test: `plugins/uploadfile/service/service_test.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [x] **Step 1: Write the failing test**

Add a service test that creates a direct-upload pending object, advances time past the pending TTL, runs cleanup, and asserts the object/ref are deleted and storage delete is called.

- [x] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service -run TestServiceCleanupDeletesExpiredPendingPresignedUploads -count=1`

Expected: compile or assertion failure because pending cleanup is not implemented.

- [x] **Step 3: Implement pending cleanup**

Add repository lookup for pending objects created before a cutoff, extend cleanup options/result with pending TTL/count, and reuse existing cleanup command/task.

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w plugins/uploadfile
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...
git diff --check
git status --short
```

Commit only uploadfile and plan changes; leave unrelated worktree changes untouched.

### Task 4: Lifecycle Configuration Wiring

**Files:**
- Modify: `plugins/uploadfile/config/config.go`
- Modify: `plugins/uploadfile/config/config_test.go`
- Modify: `plugins/uploadfile/module.go`

- [x] **Step 1: Write the failing test**

Extend config tests to read `UPLOADFILE_DOWNLOAD_TOKEN_TTL_SECONDS`, `UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS`, `UPLOADFILE_DIRECT_UPLOAD_PRESIGN_TTL_SECONDS`, and `UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS`.

- [x] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/config -run TestLoadReadsDotenvAndAppliesLocalSeed -count=1`

Expected: compile failure because the new config fields are missing.

- [x] **Step 3: Implement configuration wiring**

Parse the new env keys into `uploadfile/config.Options`, pass service TTLs to `service.New`, and pass pending cleanup TTL to scheduled/manual cleanup.

- [x] **Step 4: Verify**

Run the config test and `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
