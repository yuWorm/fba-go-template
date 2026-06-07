# Uploadfile Admin Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route the existing admin `POST /sys/files/upload` endpoint through uploadfile when the plugin is registered, while keeping the legacy response shape and fallback behavior.

**Architecture:** Add a small optional upload backend interface to the admin `FileService`. Admin resolves the backend lazily from the plugin container at request time, so admin can still register before uploadfile. Uploadfile registers an adapter that writes through `plugins/uploadfile/service.Service` and maps the result back to the existing `{url}` DTO.

**Tech Stack:** Go, Fiber v3, FBA Go DI/plugin context, admin service layer, uploadfile service layer, `go.local.mod` test/build commands.

---

## Task 1: Add Optional Admin Upload Backend

**Files:**
- Modify: `internal/app/admin/service/log_service.go`
- Modify: `internal/app/admin/api/sys_handler.go`
- Modify: `internal/app/admin/api/auth_handler.go`
- Modify: `internal/app/admin/module.go`
- Test: `internal/app/admin/plugin_test.go`

- [x] Write tests proving fallback legacy upload still returns `/static/upload/<name>_<ts>.<ext>` and an injected backend is used when available.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./internal/app/admin` and confirm the injected-backend test fails.
- [x] Add `FileUploadInput`, `FileUploadBackend`, and lazy resolver support to `FileService`.
- [x] Update the admin handler to open the multipart file and pass content type, reader, actor metadata, filename, and size into `FileService`.
- [x] Wire `NewFileServiceWithResolver(ctx.Container())` in the admin module and preserve `NewFileService()` fallback in direct handler construction.
- [x] Run the admin package tests until green.

## Task 2: Register Uploadfile Adapter

**Files:**
- Modify: `plugins/uploadfile/module.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [x] Write a plugin test proving uploadfile registers an admin `FileUploadBackend`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile` and confirm the new test fails.
- [x] Implement an uploadfile adapter that calls `Service.Upload` with default scene, temp ref, and current admin actor metadata.
- [x] Provide the adapter through `ctx.Provide`.
- [x] Run uploadfile package tests until green.

## Task 3: Verify And Commit

**Files:**
- Modify: `docs/superpowers/plans/2026-06-07-uploadfile-admin-compat.md`

- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./internal/app/admin ./plugins/uploadfile`.
- [x] Run `make L=1 test`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go build -modfile=go.local.mod ./...`.
- [x] Run `git diff --check`.
- [ ] Commit only admin/uploadfile/plan files and leave unrelated `.gitignore` untouched.
