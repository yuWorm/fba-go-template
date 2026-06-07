# Uploadfile Plugin Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first version of the `uploadfile` plugin with local storage, metadata persistence, temporary refs, scenario binding, file management, and password-capable shares.

**Architecture:** Implement a reusable `plugins/uploadfile` module with `model`, `repo`, `storage`, `service`, `api`, and `migration` packages. Keep physical file objects separate from business refs, use memory fallback for no-DB runs, switch to GORM when `db.Provider` exists, and expose routes declaratively through the plugin context.

**Tech Stack:** Go, Fiber v3, FBA Go plugin runtime, GORM, local filesystem storage, existing response/error/pagination helpers.

---

## Chunk 1: Core Domain, Storage, And Repository

### Task 1: Add Models And DTOs

**Files:**
- Create: `plugins/uploadfile/model/model.go`
- Create: `plugins/uploadfile/dto/dto.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [ ] **Step 1: Write tests for seed defaults and DTO mapping**

Create tests that assert:
- `model.SeedStorages()` contains enabled `local`.
- `model.SeedScenes()` contains `default`, `avatar`, and `attachment`.
- DTO mapping hides internal physical file paths but exposes `id`, `uuid`, `url`, `original_name`, `mime`, `size`, `status`, and ref fields.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/service`
Expected: fail because package/files do not exist.

- [ ] **Step 2: Implement models**

Add structs:
- `FileObject`
- `FileRef`
- `Scene`
- `Storage`
- `Share`

Use table names:
- `upload_file_object`
- `upload_file_ref`
- `upload_scene`
- `upload_storage`
- `upload_share`

Add constants for providers, statuses, visibility, default scene/storage codes, and seed functions.

- [ ] **Step 3: Implement DTOs**

Add DTOs for:
- `UploadResult`
- `FileDetail`
- `RefDetail`
- `SceneDetail`
- `StorageDetail`
- `ShareDetail`
- `DeleteParam`
- `BindParam`
- `ShareCreateParam`
- `ShareVerifyParam`

- [ ] **Step 4: Run model/DTO tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/service`
Expected: pass for model/DTO tests.

### Task 2: Add Local Storage Backend

**Files:**
- Create: `plugins/uploadfile/storage/backend.go`
- Create: `plugins/uploadfile/storage/local.go`
- Test: `plugins/uploadfile/storage/local_test.go`

- [ ] **Step 1: Write storage tests**

Assert local backend:
- writes a file under a temp root,
- reads it back,
- deletes it,
- rejects `../` traversal keys,
- returns deterministic public URLs when `BaseURL` is configured.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/storage`
Expected: fail because storage package is missing.

- [ ] **Step 2: Implement storage interface**

Define:
- `Backend`
- `PutOptions`
- `ObjectInfo`
- `PresignedURL`
- `Registry`

`PresignPut` and `PresignGet` can return unsupported errors for local in first version unless local signed URLs are implemented later.

- [ ] **Step 3: Implement local backend**

Use `os.MkdirAll`, `os.Create`, `io.Copy`, `os.Open`, and `os.Remove`.
Clean object keys with path validation. Reject absolute paths, empty keys, and keys that escape root.

- [ ] **Step 4: Run storage tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/storage`
Expected: pass.

### Task 3: Add Repository Interface And Memory Repository

**Files:**
- Create: `plugins/uploadfile/repo/repository.go`
- Create: `plugins/uploadfile/repo/memory.go`
- Test: `plugins/uploadfile/repo/memory_test.go`

- [ ] **Step 1: Write memory repository tests**

Assert repository can:
- fetch default scene and storage,
- create object and ref,
- list refs by `scene_code + subject_type + subject_id + field`,
- bind temp refs to active,
- create/disable shares,
- increment share download count.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/repo`
Expected: fail because repo package is missing.

- [ ] **Step 2: Define repository contract**

Include methods for:
- scene/storage lookup and list,
- object create/get/list/update status,
- ref create/list/bind/update status,
- share create/get/list/disable/increment.

Add filters:
- `ObjectFilter`
- `RefFilter`
- `ShareFilter`

- [ ] **Step 3: Implement memory repository**

Use mutex-protected slices and copy values on return.
Keep IDs auto-incrementing.
Seed scenes/storages from model seed data.

- [ ] **Step 4: Run repo tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/repo`
Expected: pass.

## Chunk 2: Service And API

### Task 4: Add Service Layer

**Files:**
- Create: `plugins/uploadfile/service/service.go`
- Create: `plugins/uploadfile/service/errors.go`
- Create: `plugins/uploadfile/service/token.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [ ] **Step 1: Write service tests**

Assert service:
- uploads bytes to local storage and creates object + temp ref,
- creates active ref when subject is provided,
- validates scene size/ext/mime,
- binds temp files to subject,
- lists refs with file details,
- soft deletes files and refs,
- creates password-protected shares,
- rejects invalid share password,
- returns a signed download token for valid password,
- streams public share downloads and increments count.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/service`
Expected: fail because service behavior is missing.

- [ ] **Step 2: Implement options and constructor**

`service.New(repository, storageRegistry, options)` should default to:
- memory repository with seed data,
- local storage under `.cache/uploadfile`,
- token secret derived from config/env fallback for local tests.

- [ ] **Step 3: Implement upload and bind**

Use request structs:
- `UploadInput`
- `BindInput`
- `Actor`

Compute SHA-256 while writing when practical. Generate UUID/key, validate file extension/MIME/size, store object, and write metadata.

- [ ] **Step 4: Implement management and ref queries**

Return `pagination.PageData` for list endpoints. Keep service methods independent from Fiber.

- [ ] **Step 5: Implement share token and password handling**

Use standard library `crypto/rand`, `crypto/sha256`, `crypto/hmac`, and `encoding/base64`.
Hash passwords with salt using SHA-256 for first version; do not store plaintext.
Signed download token payload should include share token and expiration.

- [ ] **Step 6: Run service tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/service`
Expected: pass.

### Task 5: Add API Handlers And Routes

**Files:**
- Create: `plugins/uploadfile/api/handler.go`
- Create: `plugins/uploadfile/api/routes.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [ ] **Step 1: Write route tests**

Mount the plugin in a Fiber app with a fake authenticator. Assert:
- authenticated multipart upload returns `response.Success`,
- missing file returns validation error,
- bind endpoint updates refs,
- ref query returns refs,
- share create returns token,
- public share metadata works without auth,
- password-protected download requires verify token.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile`
Expected: fail because API/module is missing.

- [ ] **Step 2: Implement handlers**

Follow existing handler style:
- bind params with Fiber binding,
- read multipart `file`,
- use `c.RequestCtx()`,
- return `c.JSON(response.Success(data))`,
- return service errors directly.

Extract current user from `c.Locals(plugin.CurrentUserLocalKey).(*rbac.CurrentUser)`.

- [ ] **Step 3: Implement routes**

Use:
- `POST /upload/files` with auth and `sys:uploadfile:upload`
- `POST /upload/files/bind` with auth and `sys:uploadfile:bind`
- `GET /upload/refs` with auth
- `GET /upload/files` with auth and `sys:uploadfile:list`
- `GET /upload/files/:pk` with auth and `sys:uploadfile:detail`
- `DELETE /upload/files` with auth and `sys:uploadfile:delete`
- `POST /upload/shares` with auth and `sys:uploadfile:share`
- `GET /upload/shares` with auth and `sys:uploadfile:share`
- `DELETE /upload/shares/:pk` with auth and `sys:uploadfile:share`
- public share routes without auth.

- [ ] **Step 4: Run plugin route tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile`
Expected: pass.

## Chunk 3: Database, Migration, And Registration

### Task 6: Add GORM Repository And Migrations

**Files:**
- Create: `plugins/uploadfile/repo/gorm.go`
- Create: `plugins/uploadfile/migration/migration.go`
- Test: `plugins/uploadfile/repo/gorm_test.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [ ] **Step 1: Write GORM repository tests**

Use in-memory SQLite. Assert:
- AutoMigrate creates tables,
- seed storage/scene rows can be inserted idempotently,
- list filters match memory behavior,
- bind/share behavior matches memory behavior.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/repo`
Expected: fail because GORM repository is missing.

- [ ] **Step 2: Implement GORM repository**

Use existing pagination style from task repo.
Map `gorm.ErrRecordNotFound` to `repo.ErrNotFound`.
Use transactions for upload object+ref creation and temp bind when needed.

- [ ] **Step 3: Implement migrations**

Add:
- `AutoMigrate(provider)` version `0001`
- `InitialData(provider)` version `0002`

Initial data should seed only upload tables (`upload_storage`, `upload_scene`) in first version.

- [ ] **Step 4: Run repo and plugin migration tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/repo ./plugins/uploadfile`
Expected: pass.

### Task 7: Add Plugin Module And App Registration

**Files:**
- Create: `plugins/uploadfile/module.go`
- Modify: `internal/app/register.go`
- Test: `internal/app/register_test.go`
- Test: `plugins/uploadfile/plugin_test.go`

- [ ] **Step 1: Write registration tests**

Assert:
- `uploadfile.FBAPlugin().Meta().ID == "uploadfile"`,
- plugin registers routes,
- plugin registers migrations when DB provider exists,
- `internal/app.Register` includes uploadfile after implementation tests pass.

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile ./internal/app`
Expected: fail until module/register updates are implemented.

- [ ] **Step 2: Implement module**

In `Register`:
- start with memory repo,
- resolve injected repo if present,
- resolve `db.Provider` and switch to GORM repo + migrations when available,
- build local storage registry from seeded/default storage,
- provide service/repository if useful for future bridge work,
- register routes.

Meta:
- ID `uploadfile`
- name `Uploadfile Plugin`
- version `0.1.0`
- description
- tags `upload`, `file`, `storage`
- optional dependency on `admin`
- `AutoInjectDefault: true`

- [ ] **Step 3: Add app registration**

Modify `internal/app/register.go` to import and add `uploadfile.FBAPlugin()` after existing reusable plugins.

- [ ] **Step 4: Run registration tests**

Run: `GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile ./internal/app`
Expected: pass.

## Chunk 4: Full Verification And Cleanup

### Task 8: Full Test Run And Final Review

**Files:**
- Review all files changed under `plugins/uploadfile`
- Review `internal/app/register.go`
- Review docs if implementation changes the plan

- [ ] **Step 1: Run targeted tests**

Run:

```bash
GOWORK=off go test -modfile=go.local.mod ./plugins/uploadfile/... ./internal/app
```

Expected: pass.

- [ ] **Step 2: Run full admin test suite**

Run:

```bash
make L=1 test
```

Expected: pass.

- [ ] **Step 3: Inspect git diff**

Run:

```bash
git diff --stat
git diff -- plugins/uploadfile internal/app/register.go
```

Expected: only uploadfile plugin and registration changes, plus already committed docs.

- [ ] **Step 4: Commit implementation**

Commit only implementation files, not unrelated existing changes such as `.gitignore`.

```bash
git add plugins/uploadfile internal/app/register.go
git commit -m "feat: add uploadfile plugin"
```

- [ ] **Step 5: Final verification after commit**

Run:

```bash
make L=1 test
```

Expected: pass.
