# Uploadfile Access Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add private authenticated downloads and temporary access URLs for uploadfile without introducing a full ACL subsystem.

**Architecture:** Keep permanent private access behind authenticated `/sys` routes and existing owner checks. Temporary access is an HMAC-signed file token embedded in the existing public file URL; public files still open without a token, private files require either authenticated access or a valid temporary token.

**Tech Stack:** Go, Fiber v3, uploadfile service/repo/storage abstractions, existing token helper style, `go.local.mod` test/build commands.

---

## Task 1: Service Access Methods

**Files:**
- Modify: `plugins/uploadfile/service/token.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/dto/dto.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [x] Write failing service tests for owner-only private download and temporary public download token.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/service` and confirm the new tests fail.
- [x] Add `FileAccessToken` DTO and token signing helpers.
- [x] Add `OpenFile` and `CreateFileAccessToken` service methods.
- [x] Update `OpenPublicFile` to allow private files only when a valid temporary token is present.
- [x] Run service tests until green.

## Task 2: API Routes

**Files:**
- Modify: `plugins/uploadfile/api/handler.go`
- Modify: `plugins/uploadfile/api/routes.go`
- Test: `plugins/uploadfile/api/handler_test.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [x] Write failing route/API tests for `GET /sys/upload/files/:pk/download` and `POST /sys/upload/files/:pk/access-token`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile ./plugins/uploadfile/api` and confirm the new tests fail.
- [x] Add handlers for authenticated private download and access-token creation.
- [x] Register routes with `plugin.Auth()`.
- [x] Run plugin/API tests until green.

## Task 3: Verify And Commit

**Files:**
- Modify: `docs/superpowers/plans/2026-06-07-uploadfile-access-control.md`

- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test -modfile=go.local.mod ./plugins/uploadfile/...`.
- [x] Run `make L=1 test`.
- [x] Run `GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go build -modfile=go.local.mod ./...`.
- [x] Run `git diff --check`.
- [ ] Commit only uploadfile/plan files and leave unrelated `.gitignore` untouched.
