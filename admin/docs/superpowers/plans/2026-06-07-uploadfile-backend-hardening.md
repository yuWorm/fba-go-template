# Uploadfile Backend Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the non-UI backend hardening for `uploadfile`: owner-scoped access, safe deletion rules, real S3/OSS backends, and share/cleanup governance.

**Architecture:** Keep enforcement in the service layer so handler and repository code stay mechanical. Reuse the existing `storage.Backend` factory registry for S3/OSS, add repository count helpers for deletion protection, and keep each increment independently testable and committable.

**Tech Stack:** Go, Fiber v3, FBA Go plugin runtime, GORM, local filesystem storage, AWS SDK for Go v2 S3, Alibaba Cloud OSS Go SDK V2, existing response/error/pagination helpers. Local tests and builds must use `go.local.mod`.

---

## References

- AWS SDK for Go v2 S3 docs cover `PutObject`, `GetObject`, `DeleteObject`, and S3 presign clients: https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_s3_code_examples.html
- Alibaba Cloud OSS Go SDK V2 package and presign docs use `github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss`: https://www.alibabacloud.com/help/en/oss/developer-reference/v2-presign-upload
- Alibaba Cloud OSS Go SDK V2 simple upload/delete docs cover `PutObject` and `DeleteObject`: https://www.alibabacloud.com/help/en/oss/developer-reference/v2-simple-upload and https://www.alibabacloud.com/help/en/oss/developer-reference/v2-delete-objects

## Scope

This plan includes:

- owner defaults and owner-scoped access control for normal users;
- super admin bypass for owner checks;
- deletion protection for referenced scenes and storages;
- S3 and OSS backend implementations behind `storage.Backend`;
- share management permission checks and cleanup dry-run support;
- focused unit/API/GORM tests plus full template verification.

This plan excludes:

- UI screens;
- multipart/断点续传;
- direct-browser upload API endpoints;
- virus scanning and image processing;
- tenant/dept data-scope integration beyond the generic `owner_type/owner_id` fields.

## File Structure

- Modify `plugins/uploadfile/api/handler.go`: pass full actor context, including super-admin flag, into service calls.
- Modify `plugins/uploadfile/api/handler_test.go`: add non-super-user and super-admin API coverage.
- Modify `plugins/uploadfile/service/service.go`: apply owner scope to upload, list, delete, bind, and share operations.
- Create `plugins/uploadfile/service/owner.go`: actor owner helpers and access checks.
- Modify `plugins/uploadfile/service/service_test.go`: service-level owner and deletion-protection tests.
- Modify `plugins/uploadfile/repo/repository.go`: add count helpers used by safe deletion.
- Modify `plugins/uploadfile/repo/memory.go`: implement count helpers for memory fallback.
- Modify `plugins/uploadfile/repo/gorm.go`: implement count helpers with GORM.
- Modify `plugins/uploadfile/repo/memory_test.go`: verify count helpers in memory.
- Modify `plugins/uploadfile/repo/gorm_test.go`: verify count helpers in SQLite-backed GORM.
- Create `plugins/uploadfile/storage/s3.go`: AWS S3 backend factory.
- Create `plugins/uploadfile/storage/s3_test.go`: fake-client tests for S3 backend behavior.
- Create `plugins/uploadfile/storage/oss.go`: Alibaba Cloud OSS backend factory.
- Create `plugins/uploadfile/storage/oss_test.go`: fake-client tests for OSS backend behavior.
- Modify `plugins/uploadfile/storage/backend.go`: register `s3` and `oss` factories.
- Modify `plugins/uploadfile/module.go`: add cleanup dry-run command flag if supported by command runtime.
- Modify `plugins/uploadfile/plugin_test.go`: verify command output and route behavior after changed signatures.
- Modify `go.mod`, `go.sum`, `go.local.mod`, `go.local.sum`: add S3/OSS SDK dependencies.

---

## Task 1: Owner Scope And Actor Context

**Files:**
- Create: `plugins/uploadfile/service/owner.go`
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/api/handler.go`
- Test: `plugins/uploadfile/service/service_test.go`
- Test: `plugins/uploadfile/api/handler_test.go`

- [ ] **Step 1: Write failing service owner tests**

Add tests to `plugins/uploadfile/service/service_test.go`:

```go
func TestServiceDefaultsUploadOwnerToCurrentUser(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "owned.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("owned"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	ref, err := repository.GetRef(ctx, uploaded.Ref.ID)
	if err != nil {
		t.Fatalf("GetRef() error = %v", err)
	}
	if ptrValue(ref.OwnerType) != "user" || ptrValue(ref.OwnerID) != "7" {
		t.Fatalf("ref owner = %v/%v, want user/7", ref.OwnerType, ref.OwnerID)
	}
}

func TestServiceRejectsNormalUserAccessToForeignOwnedFile(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename: "private.txt", ContentType: "text/plain", Size: 7,
		Reader: strings.NewReader("private"), SceneCode: model.DefaultSceneCode, Actor: owner,
	})
	if err != nil {
		t.Fatalf("Upload(owner) error = %v", err)
	}
	if err := svc.DeleteFiles(ctx, []int{uploaded.File.ID}, other); err == nil {
		t.Fatal("DeleteFiles() by foreign owner succeeded")
	}
	if _, err := svc.CreateShare(ctx, service.ShareInput{FileID: uploaded.File.ID, Actor: other}); err == nil {
		t.Fatal("CreateShare() by foreign owner succeeded")
	}
}
```

Add this helper to `plugins/uploadfile/service/service_test.go` if it is not already present:

```go
func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
```

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/service
```

Expected: FAIL because `GetRef` and owner-aware service signatures are missing.

- [ ] **Step 2: Add repository `GetRef` for access checks**

Modify `plugins/uploadfile/repo/repository.go`:

```go
GetRef(ctx context.Context, id int) (model.FileRef, error)
```

Implement in memory and GORM repositories. Add focused tests in `repo/memory_test.go` and `repo/gorm_test.go` that create a ref and load it by ID.

- [ ] **Step 3: Add owner helper**

Create `plugins/uploadfile/service/owner.go`:

```go
package service

import (
	"strconv"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

const ownerTypeUser = "user"

type Actor struct {
	UserID       *int
	IsSuperAdmin bool
}

func (a Actor) defaultOwner() (*string, *string) {
	if a.UserID == nil || *a.UserID <= 0 {
		return nil, nil
	}
	ownerType := ownerTypeUser
	ownerID := strconv.Itoa(*a.UserID)
	return &ownerType, &ownerID
}

func (a Actor) allowsOwner(ownerType *string, ownerID *string) bool {
	if a.IsSuperAdmin {
		return true
	}
	defaultType, defaultID := a.defaultOwner()
	if defaultType == nil || defaultID == nil {
		return ownerType == nil && ownerID == nil
	}
	return ownerValue(ownerType) == *defaultType && ownerValue(ownerID) == *defaultID
}

func (a Actor) ownsObject(object model.FileObject, refs []model.FileRef) bool {
	if a.IsSuperAdmin {
		return true
	}
	if a.UserID != nil && object.UploadedBy != nil && *object.UploadedBy == *a.UserID {
		return true
	}
	for _, ref := range refs {
		if a.allowsOwner(ref.OwnerType, ref.OwnerID) {
			return true
		}
	}
	return false
}

func ownerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
```

Move the existing `Actor` declaration out of `service.go` so there is one definition.

- [ ] **Step 4: Update upload owner defaults**

In `Service.Upload`, after resolving `temp`, default missing `OwnerType/OwnerID` from `input.Actor.defaultOwner()` and reject mismatched explicit owners for normal users:

```go
ownerType := cleanOptional(input.OwnerType)
ownerID := cleanOptional(input.OwnerID)
if ownerType == nil && ownerID == nil {
	ownerType, ownerID = input.Actor.defaultOwner()
}
if !input.Actor.allowsOwner(ownerType, ownerID) {
	return dto.UploadResult{}, forbidden("file owner is not allowed")
}
```

Use `ownerType` and `ownerID` in `repo.CreateRefParam`.

- [ ] **Step 5: Update service signatures**

Change these service methods:

```go
ListRefs(ctx context.Context, filter repo.RefFilter, page int, size int, actor Actor) (pagination.PageData[dto.RefDetail], error)
ListFiles(ctx context.Context, filter repo.ObjectFilter, page int, size int, actor Actor) (pagination.PageData[dto.FileDetail], error)
DeleteFiles(ctx context.Context, ids []int, actor Actor) error
CreateShare(ctx context.Context, input ShareInput) (dto.ShareDetail, error)
ListShares(ctx context.Context, filter repo.ShareFilter, page int, size int, actor Actor) (pagination.PageData[dto.ShareDetail], error)
DisableShare(ctx context.Context, id int, actor Actor) error
```

Normal users must be scoped:

```go
if !actor.IsSuperAdmin {
	ownerType, ownerID := actor.defaultOwner()
	filter.OwnerType = ownerValue(ownerType)
	filter.OwnerID = ownerValue(ownerID)
}
```

For delete/share operations, load the object and matching refs before mutating; return `forbidden("file owner is not allowed")` when `actor.ownsObject(...)` is false.

- [ ] **Step 6: Update API actor wiring**

Modify `plugins/uploadfile/api/handler.go`:

```go
func actor(c fiber.Ctx) service.Actor {
	user, ok := c.Locals(plugin.CurrentUserLocalKey).(*rbac.CurrentUser)
	if !ok || user == nil || user.ID <= 0 {
		id := defaultCurrentUserID
		return service.Actor{UserID: &id}
	}
	id := int(user.ID)
	return service.Actor{UserID: &id, IsSuperAdmin: user.IsSuperAdmin}
}
```

Pass `actor(c)` into list/delete/share service calls.

- [ ] **Step 7: Run owner tests**

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/repo ./plugins/uploadfile/service ./plugins/uploadfile/api
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add plugins/uploadfile
git commit -m "feat: enforce uploadfile owner scope"
```

Do not stage `.gitignore`.

---

## Task 2: Scene And Storage Deletion Protection

**Files:**
- Modify: `plugins/uploadfile/repo/repository.go`
- Modify: `plugins/uploadfile/repo/memory.go`
- Modify: `plugins/uploadfile/repo/gorm.go`
- Modify: `plugins/uploadfile/service/service.go`
- Test: `plugins/uploadfile/repo/memory_test.go`
- Test: `plugins/uploadfile/repo/gorm_test.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [ ] **Step 1: Write failing deletion-protection tests**

Add service tests:

```go
func TestServicePreventsDeletingSceneWithRefs(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename: "scene.txt", ContentType: "text/plain", Size: 5,
		Reader: strings.NewReader("scene"), SceneCode: model.DefaultSceneCode,
		Actor: service.Actor{UserID: intPtr(1), IsSuperAdmin: true},
	})
	if err != nil || uploaded.Ref.ID == 0 {
		t.Fatalf("Upload() = %+v, %v", uploaded, err)
	}
	if err := svc.DeleteScene(ctx, model.DefaultSceneCode); err == nil {
		t.Fatal("DeleteScene() deleted referenced scene")
	}
}

func TestServicePreventsDeletingStorageInUse(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	_, err := svc.Upload(ctx, service.UploadInput{
		Filename: "storage.txt", ContentType: "text/plain", Size: 7,
		Reader: strings.NewReader("storage"), SceneCode: model.DefaultSceneCode,
		Actor: service.Actor{UserID: intPtr(1), IsSuperAdmin: true},
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if err := svc.DeleteStorage(ctx, model.DefaultStorageCode); err == nil {
		t.Fatal("DeleteStorage() deleted storage with objects")
	}
}
```

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/service
```

Expected: FAIL because deletion protection is not enforced.

- [ ] **Step 2: Add repository count helpers**

Modify `Repository`:

```go
CountRefsByScene(ctx context.Context, sceneCode string) (int64, error)
CountObjectsByStorage(ctx context.Context, storageCode string) (int64, error)
CountScenesByStorage(ctx context.Context, storageCode string) (int64, error)
```

Memory implementation counts non-deleted refs/objects/scenes by code. GORM implementation uses `Model(...).Where(...).Count(&total)`.

- [ ] **Step 3: Enforce safe deletes in service**

Update `DeleteScene`:

```go
refCount, err := s.repo.CountRefsByScene(ctx, code)
if err != nil {
	return err
}
if refCount > 0 {
	return badRequest("upload scene is in use", nil)
}
```

Update `DeleteStorage`:

```go
objectCount, err := s.repo.CountObjectsByStorage(ctx, code)
if err != nil {
	return err
}
if objectCount > 0 {
	return badRequest("storage is in use", nil)
}
sceneCount, err := s.repo.CountScenesByStorage(ctx, code)
if err != nil {
	return err
}
if sceneCount > 0 {
	return badRequest("storage is used by upload scenes", nil)
}
```

Keep updates that set `enabled=false` allowed.

- [ ] **Step 4: Run repo and service tests**

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/repo ./plugins/uploadfile/service
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add plugins/uploadfile
git commit -m "feat: protect uploadfile configuration deletion"
```

---

## Task 3: AWS S3 Backend

**Files:**
- Create: `plugins/uploadfile/storage/s3.go`
- Create: `plugins/uploadfile/storage/s3_test.go`
- Modify: `plugins/uploadfile/storage/backend.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `go.local.mod`
- Modify: `go.local.sum`

- [ ] **Step 1: Add dependencies**

Run with network approval if needed:

```bash
go get github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/s3
go get -modfile=go.local.mod github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/s3
```

Expected: `go.mod`, `go.sum`, `go.local.mod`, and `go.local.sum` include AWS SDK v2 modules.

- [ ] **Step 2: Write failing fake-client tests**

Create `plugins/uploadfile/storage/s3_test.go` with fake client interfaces:

```go
func TestS3BackendPutOpenDeleteAndPublicURL(t *testing.T) {
	client := newFakeS3Client()
	backend := NewS3WithClient(S3Options{
		Bucket:  "bucket-a",
		BaseURL: "https://cdn.example.test",
	}, client, fakeS3Presigner{})

	info, err := backend.Put(context.Background(), "uploads/a.txt", strings.NewReader("hello"), PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != "uploads/a.txt" || info.Size != 5 {
		t.Fatalf("Put() info = %+v", info)
	}
	reader, opened, err := backend.Open(context.Background(), "uploads/a.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()
	body, _ := io.ReadAll(reader)
	if string(body) != "hello" || opened.Size != 5 {
		t.Fatalf("Open() body=%q info=%+v", body, opened)
	}
	if url := backend.PublicURL("uploads/a.txt"); url != "https://cdn.example.test/uploads/a.txt" {
		t.Fatalf("PublicURL() = %q", url)
	}
	if err := backend.Delete(context.Background(), "uploads/a.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
```

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/storage
```

Expected: FAIL because `NewS3WithClient` and `S3Options` do not exist.

- [ ] **Step 3: Implement S3 backend with injectable interfaces**

Create `plugins/uploadfile/storage/s3.go`:

```go
type S3Options struct {
	Bucket         string
	Region         string
	Endpoint       string
	BaseURL        string
	ForcePathStyle bool
}

type s3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3PresignAPI interface {
	PresignPutObject(context.Context, *s3.PutObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}
```

`NewS3FromConfig` must:

- require `Bucket`;
- use `config.LoadDefaultConfig(ctx)` indirectly through a constructor helper;
- apply `Region`;
- set custom endpoint when `BackendConfig.Endpoint` is present;
- use `BackendConfig.BaseURL` for public URLs;
- parse `ForcePathStyle` from JSON `config`.

- [ ] **Step 4: Register S3 factory**

Modify `storage.NewRegistry()`:

```go
registry.AddFactory("s3", NewS3FromConfig)
```

- [ ] **Step 5: Run storage tests and build**

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/storage
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go build -modfile=go.local.mod ./plugins/uploadfile/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum go.local.mod go.local.sum plugins/uploadfile
git commit -m "feat: add uploadfile s3 backend"
```

---

## Task 4: Alibaba Cloud OSS Backend

**Files:**
- Create: `plugins/uploadfile/storage/oss.go`
- Create: `plugins/uploadfile/storage/oss_test.go`
- Modify: `plugins/uploadfile/storage/backend.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `go.local.mod`
- Modify: `go.local.sum`

- [ ] **Step 1: Add dependencies**

Run with network approval if needed:

```bash
go get github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss
go get -modfile=go.local.mod github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss
```

Expected: both module files and sum files include the OSS SDK V2 package.

- [ ] **Step 2: Write failing fake-client tests**

Create `plugins/uploadfile/storage/oss_test.go`:

```go
func TestOSSBackendPutOpenDeleteAndPresign(t *testing.T) {
	client := newFakeOSSClient()
	backend := NewOSSWithClient(OSSOptions{
		Bucket:  "oss-bucket",
		Region:  "cn-hangzhou",
		BaseURL: "https://cdn.example.test",
	}, client)

	info, err := backend.Put(context.Background(), "uploads/a.txt", strings.NewReader("hello"), PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != "uploads/a.txt" || info.Size != 5 {
		t.Fatalf("Put() info = %+v", info)
	}
	presigned, err := backend.PresignPut(context.Background(), "uploads/a.txt", 10*time.Minute, PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PresignPut() error = %v", err)
	}
	if presigned.Method != "PUT" || presigned.URL == "" {
		t.Fatalf("PresignPut() = %+v", presigned)
	}
	if err := backend.Delete(context.Background(), "uploads/a.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
```

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/storage
```

Expected: FAIL because OSS backend does not exist.

- [ ] **Step 3: Implement OSS backend with injectable interface**

Create `plugins/uploadfile/storage/oss.go`:

```go
type OSSOptions struct {
	Bucket   string
	Region   string
	Endpoint string
	BaseURL  string
}

type ossAPI interface {
	PutObject(context.Context, *oss.PutObjectRequest, ...func(*oss.Options)) (*oss.PutObjectResult, error)
	GetObject(context.Context, *oss.GetObjectRequest, ...func(*oss.Options)) (*oss.GetObjectResult, error)
	DeleteObject(context.Context, *oss.DeleteObjectRequest, ...func(*oss.Options)) (*oss.DeleteObjectResult, error)
	Presign(context.Context, any, ...func(*oss.PresignOptions)) (*oss.PresignResult, error)
}
```

`NewOSSFromConfig` must:

- require `Bucket` and `Region`;
- use `oss.LoadDefaultConfig().WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).WithRegion(region)`;
- apply endpoint if configured;
- use `BaseURL` for public URLs;
- map `PresignResult.SignedHeaders` to `storage.PresignedURL.Headers`.

- [ ] **Step 4: Register OSS factory**

Modify `storage.NewRegistry()`:

```go
registry.AddFactory("oss", NewOSSFromConfig)
```

- [ ] **Step 5: Run storage tests and build**

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/storage
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go build -modfile=go.local.mod ./plugins/uploadfile/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum go.local.mod go.local.sum plugins/uploadfile
git commit -m "feat: add uploadfile oss backend"
```

---

## Task 5: Share Governance And Cleanup Dry Run

**Files:**
- Modify: `plugins/uploadfile/service/service.go`
- Modify: `plugins/uploadfile/module.go`
- Modify: `plugins/uploadfile/plugin_test.go`
- Test: `plugins/uploadfile/service/service_test.go`

- [ ] **Step 1: Write failing share governance tests**

Add service tests:

```go
func TestServiceScopesShareListAndDisableToActor(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename: "share.txt", ContentType: "text/plain", Size: 5,
		Reader: strings.NewReader("share"), SceneCode: model.DefaultSceneCode, Actor: owner,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	share, err := svc.CreateShare(ctx, service.ShareInput{FileID: uploaded.File.ID, Actor: owner})
	if err != nil {
		t.Fatalf("CreateShare() error = %v", err)
	}
	foreign, err := svc.ListShares(ctx, repo.ShareFilter{}, 1, 20, other)
	if err != nil {
		t.Fatalf("ListShares(other) error = %v", err)
	}
	if len(foreign.Items) != 0 {
		t.Fatalf("foreign shares = %+v, want empty", foreign)
	}
	if err := svc.DisableShare(ctx, share.ID, other); err == nil {
		t.Fatal("DisableShare() by foreign user succeeded")
	}
}

func TestServiceCleanupExpiredTempsDryRunDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now:         func() time.Time { return now },
	})
	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename: "temp.txt", ContentType: "text/plain", Size: 4,
		Reader: strings.NewReader("temp"), SceneCode: model.DefaultSceneCode,
		Actor: service.Actor{UserID: intPtr(7)},
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	svc = service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now:         func() time.Time { return now.Add(48 * time.Hour) },
	})
	result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CleanupExpiredTemps(dry-run) error = %v", err)
	}
	if result.ExpiredRefs != 1 || result.DeletedFiles != 1 {
		t.Fatalf("dry-run result = %+v, want projected cleanup", result)
	}
	ref, err := repository.GetRef(ctx, uploaded.Ref.ID)
	if err != nil || ref.Status != model.RefStatusTemp {
		t.Fatalf("ref after dry-run = %+v, %v; want temp", ref, err)
	}
}
```

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/service
```

Expected: FAIL because cleanup options and share scoping are not complete.

- [ ] **Step 2: Add cleanup options**

Change:

```go
func (s *Service) CleanupExpiredTemps(ctx context.Context) (dto.CleanupResult, error)
```

to:

```go
type CleanupOptions struct {
	DryRun bool
}

func (s *Service) CleanupExpiredTemps(ctx context.Context, opts CleanupOptions) (dto.CleanupResult, error)
```

When `DryRun` is true, compute `ExpiredRefs` and projected `DeletedFiles`, but do not update refs, delete backend objects, or update object status.

- [ ] **Step 3: Add command dry-run flag**

If `core/command.Command` supports flags, add `--dry-run` to `uploadfile cleanup`. If it does not, accept positional `--dry-run` in args:

```go
dryRun := false
for _, arg := range args {
	if arg == "--dry-run" {
		dryRun = true
	}
}
result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{DryRun: dryRun})
```

Output:

```text
expired_refs=1 deleted_files=1 dry_run=true
```

- [ ] **Step 4: Run service and plugin command tests**

Run:

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/service ./plugins/uploadfile
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add plugins/uploadfile
git commit -m "feat: govern uploadfile shares and cleanup"
```

---

## Task 6: Final Verification

**Files:**
- Verify all changed files.
- Do not stage `.gitignore` unless the user explicitly asks for it.

- [ ] **Step 1: Run targeted plugin tests**

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go test -modfile=go.local.mod ./plugins/uploadfile/...
```

Expected: all uploadfile packages pass.

- [ ] **Step 2: Run full template tests**

```bash
make L=1 test
```

Expected: command internally uses `go test -modfile=go.local.mod ./...` and exits 0.

- [ ] **Step 3: Run full build**

```bash
GOWORK=off GOCACHE=/Volumes/WorkSpace/Projects/Idea/fba-go/templates/fba-go-template/admin/.cache/go-build go build -modfile=go.local.mod ./...
```

Expected: exit 0.

- [ ] **Step 4: Review staged files**

```bash
git status --short
git diff --check
git diff --stat HEAD~4..HEAD -- plugins/uploadfile go.mod go.sum go.local.mod go.local.sum
```

Expected:

- no whitespace errors;
- only uploadfile and module dependency files staged/committed for these tasks;
- `.gitignore` remains unstaged unless explicitly requested.
