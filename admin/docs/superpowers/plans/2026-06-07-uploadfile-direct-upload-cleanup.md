# Uploadfile Direct Upload And Cleanup Task Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add presigned direct-upload APIs, register an executable uploadfile cleanup task, and document uploadfile environment variables.

**Architecture:** Presigned uploads reuse the existing storage backend `PresignPut` contract and create a pending file object plus ref before the browser uploads to S3/OSS. A completion endpoint marks the pending object active after ownership checks. Cleanup remains service-owned and is exposed through both the existing CLI command and a task definition named `uploadfile.cleanup`.

**Tech Stack:** Go, Fiber v3 handlers/routes, uploadfile service/repo/storage abstractions, core plugin task metadata, core task definition registry, Asynq handler interface, `.env` template docs, `go.local.mod` verification.

---

## Task 1: Presigned Direct Upload

**Files:**
- Modify: `plugins/uploadfile/dto/dto.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/api/handler.go`
- Modify: `plugins/uploadfile/api/routes.go`
- Test: `plugins/uploadfile/service/service_test.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [x] Write failing service tests for `CreatePresignedUpload` creating a pending object/ref and `CompletePresignedUpload` marking it active.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service` and confirm it fails.
- [x] Implement service inputs/results, presign validation, pending object/ref creation, owner checks, and completion.
- [x] Add API DTOs and handlers for `POST /sys/upload/files/presign` and `POST /sys/upload/files/:pk/complete`.
- [x] Update route registration test.
- [x] Run service and plugin tests until green.

## Task 2: Cleanup Task Registration

**Files:**
- Modify: `plugins/uploadfile/module.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [x] Write failing plugin tests that `ctx.Tasks()` contains `uploadfile.cleanup` and a provided `core/task.Registry` receives an executable definition.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile` and confirm it fails.
- [x] Register plugin task metadata and add a core task definition when a registry is available.
- [x] Run plugin tests until green.

## Task 3: Env Examples And Verification

**Files:**
- Modify: `.env`
- Modify: `env.tmpl`
- Modify: `docs/superpowers/plans/2026-06-07-uploadfile-direct-upload-cleanup.md`

- [x] Add commented `UPLOADFILE_*` examples to `.env` and `env.tmpl`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
- [x] Run `make L=1 test`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go build -modfile=go.local.mod ./...`.
- [ ] Commit only uploadfile/docs/env files; keep unrelated `.gitignore` untouched.
