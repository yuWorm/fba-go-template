# Uploadfile 使用文档

本文说明 `plugins/uploadfile` 的两种使用方式：

- Go 代码内直接调用函数。
- 通过 HTTP API 调用。

当前文档只覆盖后端能力，不包含 UI。

## 核心概念

Uploadfile 把物理文件和业务归属拆开：

- `upload_file_object`：物理文件对象，记录存储后端、object key、大小、MIME、状态等。
- `upload_file_ref`：业务引用，记录场景、业务对象、字段、owner、临时状态等。
- `upload_scene`：上传场景，控制大小、扩展名、MIME、默认存储和临时文件 TTL。
- `upload_storage`：存储后端，支持 `local`、`s3`、`oss`。
- `upload_share`：分享记录，支持密码、过期时间、最大下载次数。

owner 默认规则：

- 普通用户默认 owner 为 `owner_type=user`、`owner_id=<当前用户ID>`。
- 普通用户只能上传、绑定、查询自己的 owner。
- 超管可以指定或查询任意 owner。
- deleted ref 不再授权访问，也不会让对象出现在 owner/scene 统计中。

## 环境配置

可在 `.env` 或进程环境变量中配置：

```dotenv
UPLOADFILE_STORAGE_PROVIDER=local
UPLOADFILE_STORAGE_PREFIX=uploads
UPLOADFILE_STORAGE_BASE_URL=
UPLOADFILE_LOCAL_ROOT=.cache/uploadfile
UPLOADFILE_STORAGE_BUCKET=
UPLOADFILE_STORAGE_REGION=
UPLOADFILE_STORAGE_ENDPOINT=
UPLOADFILE_S3_FORCE_PATH_STYLE=false
UPLOADFILE_OSS_USE_PATH_STYLE=false
UPLOADFILE_OSS_USE_CNAME=false

UPLOADFILE_DEFAULT_MAX_SIZE=20971520
UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS=86400
UPLOADFILE_DEFAULT_ALLOWED_EXTS=jpg,jpeg,png,gif,webp,pdf,txt,doc,docx,xls,xlsx,mp4,mov,avi,flv
UPLOADFILE_DEFAULT_ALLOWED_MIMES=

UPLOADFILE_DOWNLOAD_TOKEN_TTL_SECONDS=600
UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS=3600
UPLOADFILE_DIRECT_UPLOAD_PRESIGN_TTL_SECONDS=900
UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS=900

UPLOADFILE_MAX_TOTAL_BYTES=0
UPLOADFILE_MAX_OWNER_BYTES=0
UPLOADFILE_MAX_TOTAL_FILES=0
UPLOADFILE_MAX_OWNER_FILES=0
```

说明：

- 配额值为 `0` 表示不限制。
- `UPLOADFILE_DEFAULT_*` 只影响默认 scene 的 seed/default 配置。
- S3 凭证走 AWS SDK 默认凭证链。
- OSS 凭证使用 `OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`OSS_SESSION_TOKEN`。

## Go 内直接调用

### 方式一：使用插件提供的 Admin 上传后端

如果业务模块只需要“上传文件并拿到 URL”，优先使用插件注册的 `adminservice.FileUploadBackend`。uploadfile 插件注册时会提供这个后端。

```go
package yourservice

import (
	"context"
	"strings"

	adminservice "github.com/yuWorm/fba-go-template/admin/internal/app/admin/service"
)

type Service struct {
	upload adminservice.FileUploadBackend
}

func New(upload adminservice.FileUploadBackend) *Service {
	return &Service{upload: upload}
}

func (s *Service) UploadAvatar(ctx context.Context, userID int) (string, error) {
	result, err := s.upload.Upload(ctx, adminservice.FileUploadInput{
		Filename:    "avatar.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("hello"),
		UserID:      &userID,
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
```

这个接口会使用 uploadfile 的默认 scene，返回公开访问路径，例如：

```text
/api/v1/public/upload/files/<uuid>
```

私有文件直接访问该 URL 会被拒绝，需要临时访问 token 或分享下载 token。

### 方式二：直接构造完整 Service

需要绑定、分享、统计、清理等完整能力时，可以直接使用 `plugins/uploadfile/service.Service`。

```go
package main

import (
	"context"
	"strings"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
)

func example() error {
	ctx := context.Background()
	userID := 7

	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{
		Root: ".cache/uploadfile",
	}))

	svc := service.New(repository, registry, service.Options{
		TokenSecret:            []byte("replace-with-auth-secret"),
		DownloadTokenTTL:       10 * time.Minute,
		FileAccessTokenMaxTTL:  time.Hour,
		DirectUploadPresignTTL: 15 * time.Minute,
		MaxOwnerBytes:          100 * 1024 * 1024,
	})

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "contract.txt",
		ContentType: "text/plain",
		Size:        8,
		Reader:      strings.NewReader("contract"),
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "order",
		SubjectID:   "SO-1001",
		Field:       "contract",
		Actor:       service.Actor{UserID: &userID},
	})
	if err != nil {
		return err
	}

	_ = uploaded.File.ID
	_ = uploaded.File.URL
	return nil
}
```

生产环境通常使用插件注册流程自动创建 GORM repository。手动构造时，如果已有 `db.Provider`，使用：

```go
repository := repo.NewGORMRepository(provider)
```

### 普通上传

```go
result, err := svc.Upload(ctx, service.UploadInput{
	Filename:    "invoice.pdf",
	ContentType: "application/pdf",
	Size:        fileSize,
	Reader:      reader,
	SceneCode:   "default",
	SubjectType: "order",
	SubjectID:   "SO-1001",
	Field:       "invoice",
	Actor:       service.Actor{UserID: &userID},
})
```

行为：

- 校验 scene 是否存在且启用。
- 校验大小、扩展名、MIME。
- 校验 owner 和配额。
- 写入存储后端。
- 创建 file object 和 file ref。
- 如果没有 `SubjectType` 或 `SubjectID`，默认创建临时 ref。

### Presigned 直传

第一步：创建 presigned PUT URL。

```go
presigned, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
	Filename:    "video.mp4",
	ContentType: "video/mp4",
	Size:        1024,
	SceneCode:   "default",
	SubjectType: "course",
	SubjectID:   "1001",
	Field:       "video",
	TTL:         15 * time.Minute,
	Actor:       service.Actor{UserID: &userID},
})
```

返回内容包含：

- `presigned.File`：`status=pending` 的文件对象。
- `presigned.Ref`：文件引用。
- `presigned.Presigned`：客户端上传所需的 method、URL、expires_at、headers。

第二步：客户端向 `presigned.Presigned.URL` 执行 PUT。

第三步：客户端上传完成后，业务侧确认：

```go
file, err := svc.CompletePresignedUpload(ctx, presigned.File.ID, service.Actor{
	UserID: &userID,
})
```

确认时会调用存储后端 `Head` 校验对象存在、大小一致、MIME 一致，然后把对象标记为 `active`。

### 绑定临时文件

临时文件可以后续绑定到业务对象：

```go
err := svc.Bind(ctx, service.BindInput{
	FileIDs:     []int{fileID},
	SceneCode:   "default",
	SubjectType: "order",
	SubjectID:   "SO-1001",
	Field:       "attachment",
	Actor:       service.Actor{UserID: &userID},
})
```

绑定后 ref 会变为 `active`，并写入 subject 和 field。

### 查询文件、引用和统计

```go
files, err := svc.ListFiles(ctx, repo.ObjectFilter{
	SceneCode: "default",
	Status:    model.StatusActive,
}, 1, 20, service.Actor{UserID: &userID})

refs, err := svc.ListRefs(ctx, repo.RefFilter{
	SceneCode:   "default",
	SubjectType: "order",
	SubjectID:   "SO-1001",
}, 1, 20, service.Actor{UserID: &userID})

stats, err := svc.UploadStats(ctx, repo.UsageFilter{
	SceneCode: "default",
}, service.Actor{UserID: &userID})
```

普通用户会自动按自己的 owner scope 过滤。超管可以查全局：

```go
stats, err := svc.UploadStats(ctx, repo.UsageFilter{}, service.Actor{
	UserID:       &adminID,
	IsSuperAdmin: true,
})
```

### 打开和下载文件

```go
reader, file, err := svc.OpenFile(ctx, fileID, service.Actor{UserID: &userID})
if err != nil {
	return err
}
defer reader.Close()

_ = file.OriginalName
```

私有文件需要 owner 权限。公开 URL 访问私有文件时需要临时 token：

```go
token, err := svc.CreateFileAccessToken(ctx, fileID, service.FileAccessTokenInput{
	TTL:   5 * time.Minute,
	Actor: service.Actor{UserID: &userID},
})
```

然后访问：

```text
token.DownloadURL
```

### 创建分享

```go
password := "secret"
maxDownloads := 3
expiresAt := time.Now().Add(24 * time.Hour)

share, err := svc.CreateShare(ctx, service.ShareInput{
	FileID:       fileID,
	Password:     &password,
	ExpiresAt:    &expiresAt,
	MaxDownloads: &maxDownloads,
	Actor:        service.Actor{UserID: &userID},
})
```

密码分享先验证密码：

```go
downloadToken, err := svc.VerifySharePassword(ctx, share.Token, "secret")
```

再下载：

```go
reader, file, err := svc.OpenShare(ctx, share.Token, downloadToken)
```

无密码分享直接传空 token：

```go
reader, file, err := svc.OpenShare(ctx, share.Token, "")
```

### 清理临时文件和 pending 直传

```go
result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{
	DryRun:     true,
	PendingTTL: 15 * time.Minute,
})
```

返回：

```go
result.ExpiredRefs
result.PendingFiles
result.DeletedFiles
```

插件注册后也会提供：

```bash
admin uploadfile cleanup
admin uploadfile cleanup --dry-run
```

任务类型：

```text
uploadfile.cleanup
```

## HTTP API 调用

所有管理 API 挂载在 `/api/v1` 下。JSON 成功响应格式：

```json
{
  "code": 200,
  "msg": "请求成功",
  "data": {}
}
```

示例中使用：

```bash
BASE_URL=http://127.0.0.1:8000/api/v1
TOKEN=<your-jwt-token>
AUTH="-H Authorization: Bearer $TOKEN"
```

实际 shell 中建议直接写 `-H "Authorization: Bearer $TOKEN"`。

### 权限

需要登录：

- 所有 `/sys/upload/*` API。

需要权限码：

- 上传和直传：`sys:upload:file:add`
- 删除文件：`sys:upload:file:del`
- 绑定 ref：`sys:upload:ref:bind`
- 创建 scene：`sys:upload:scene:add`
- 修改 scene：`sys:upload:scene:edit`
- 删除 scene：`sys:upload:scene:del`
- 创建 storage：`sys:upload:storage:add`
- 修改 storage：`sys:upload:storage:edit`
- 删除 storage：`sys:upload:storage:del`
- 创建 share：`sys:upload:share:add`
- 删除 share：`sys:upload:share:del`

公共下载和公共分享 API 不需要登录。

## 文件 API

### 普通上传

```bash
curl -X POST "$BASE_URL/sys/upload/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./invoice.txt;type=text/plain" \
  -F "scene_code=default" \
  -F "subject_type=order" \
  -F "subject_id=SO-1001" \
  -F "field=invoice" \
  -F "owner_type=user" \
  -F "owner_id=7"
```

响应 `data`：

```json
{
  "file": {
    "id": 1,
    "uuid": "....",
    "storage_code": "local",
    "provider": "local",
    "original_name": "invoice.txt",
    "ext": "txt",
    "mime": "text/plain",
    "size": 12,
    "visibility": "private",
    "status": "active",
    "url": "/api/v1/public/upload/files/<uuid>",
    "uploaded_by": 7
  },
  "ref": {
    "id": 1,
    "file_id": 1,
    "scene_code": "default",
    "subject_type": "order",
    "subject_id": "SO-1001",
    "field": "invoice",
    "status": "active",
    "owner_type": "user",
    "owner_id": "7"
  }
}
```

### 创建 presigned 直传

```bash
curl -X POST "$BASE_URL/sys/upload/files/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "video.mp4",
    "content_type": "video/mp4",
    "size": 1024,
    "scene_code": "default",
    "subject_type": "course",
    "subject_id": "1001",
    "field": "video",
    "owner_type": "user",
    "owner_id": "7",
    "ttl_seconds": 900
  }'
```

响应 `data.presigned`：

```json
{
  "method": "PUT",
  "url": "https://...",
  "expires_at": "2026-06-08 10:15:00",
  "headers": {
    "Content-Type": "video/mp4"
  }
}
```

客户端按返回的 method、url、headers 上传文件字节。

### 完成 presigned 直传

```bash
curl -X POST "$BASE_URL/sys/upload/files/1/complete" \
  -H "Authorization: Bearer $TOKEN"
```

完成时会校验存储对象的 size 和 MIME。校验失败时对象保持 `pending`。

### 查询文件列表

```bash
curl "$BASE_URL/sys/upload/files?page=1&size=20&scene_code=default&status=active" \
  -H "Authorization: Bearer $TOKEN"
```

支持 query：

- `keyword`
- `scene_code`
- `provider`
- `storage_code`
- `status`
- `uploaded_by`
- `owner_type`
- `owner_id`
- `page`
- `size`

### 查询文件详情

```bash
curl "$BASE_URL/sys/upload/files/1" \
  -H "Authorization: Bearer $TOKEN"
```

### 认证下载

```bash
curl -L "$BASE_URL/sys/upload/files/1/download" \
  -H "Authorization: Bearer $TOKEN" \
  -o invoice.txt
```

### 创建私有文件临时访问 token

```bash
curl -X POST "$BASE_URL/sys/upload/files/1/access-token" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ttl_seconds":300}'
```

响应：

```json
{
  "download_url": "/api/v1/public/upload/files/<uuid>?download_token=<token>",
  "download_token": "<token>",
  "expires_at": "2026-06-08 10:05:00"
}
```

访问：

```bash
curl -L "http://127.0.0.1:8000/api/v1/public/upload/files/<uuid>?download_token=<token>" \
  -o file.bin
```

### 删除文件

```bash
curl -X DELETE "$BASE_URL/sys/upload/files" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pks":[1,2,3]}'
```

删除是状态删除，状态变为 `deleted`。

### 上传统计

```bash
curl "$BASE_URL/sys/upload/stats?scene_code=default" \
  -H "Authorization: Bearer $TOKEN"
```

支持 query：

- `scene_code`
- `provider`
- `storage_code`
- `status`
- `owner_type`
- `owner_id`

响应：

```json
{
  "files": 2,
  "bytes": 12
}
```

普通用户会自动限制到自己的 owner。超管可以查全局或指定 owner。

## Ref API

### 绑定文件到业务对象

```bash
curl -X POST "$BASE_URL/sys/upload/refs/bind" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_ids": [1],
    "scene_code": "default",
    "subject_type": "order",
    "subject_id": "SO-1001",
    "field": "invoice",
    "owner_type": "user",
    "owner_id": "7"
  }'
```

### 查询 ref

```bash
curl "$BASE_URL/sys/upload/refs?scene_code=default&subject_type=order&subject_id=SO-1001&field=invoice" \
  -H "Authorization: Bearer $TOKEN"
```

支持 query：

- `file_id`
- `scene_code`
- `subject_type`
- `subject_id`
- `field`
- `status`
- `owner_type`
- `owner_id`
- `page`
- `size`

## Scene API

### 查询 scene

```bash
curl "$BASE_URL/sys/upload/scenes" \
  -H "Authorization: Bearer $TOKEN"
```

### 创建 scene

```bash
curl -X POST "$BASE_URL/sys/upload/scenes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "contract",
    "name": "Contract",
    "max_size": 10485760,
    "allowed_exts": "[\"pdf\",\"txt\"]",
    "allowed_mimes": "[\"application/pdf\",\"text/plain\"]",
    "default_storage_code": "local",
    "default_visibility": "private",
    "temp_ttl_seconds": 86400,
    "enabled": true
  }'
```

### 修改 scene

```bash
curl -X PUT "$BASE_URL/sys/upload/scenes/contract" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Contract Files",
    "max_size": 20971520,
    "allowed_exts": "[\"pdf\",\"txt\",\"docx\"]",
    "enabled": true
  }'
```

### 删除 scene

```bash
curl -X DELETE "$BASE_URL/sys/upload/scenes/contract" \
  -H "Authorization: Bearer $TOKEN"
```

默认 seed scene 不能删除；已被 ref 使用的 scene 不能删除。

## Storage API

### 查询 storage

```bash
curl "$BASE_URL/sys/upload/storages" \
  -H "Authorization: Bearer $TOKEN"
```

### 创建 local storage

```bash
curl -X POST "$BASE_URL/sys/upload/storages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "local2",
    "provider": "local",
    "base_url": "https://cdn.example.com/files",
    "prefix": "uploads",
    "is_default": false,
    "enabled": true,
    "config": "{\"root\":\"/var/lib/fba/uploads\",\"base_url\":\"https://cdn.example.com/files\"}"
  }'
```

### 创建 S3 storage

```bash
curl -X POST "$BASE_URL/sys/upload/storages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "s3-main",
    "provider": "s3",
    "bucket": "my-bucket",
    "region": "ap-southeast-1",
    "endpoint": "https://s3.example.com",
    "base_url": "https://cdn.example.com",
    "prefix": "uploads",
    "is_default": false,
    "enabled": true,
    "config": "{\"force_path_style\":true}"
  }'
```

### 创建 OSS storage

```bash
curl -X POST "$BASE_URL/sys/upload/storages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "oss-main",
    "provider": "oss",
    "bucket": "my-bucket",
    "region": "cn-hangzhou",
    "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
    "base_url": "https://cdn.example.com",
    "prefix": "uploads",
    "is_default": false,
    "enabled": true,
    "config": "{\"use_path_style\":true,\"use_cname\":false}"
  }'
```

### 修改 storage

```bash
curl -X PUT "$BASE_URL/sys/upload/storages/local" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "local",
    "prefix": "managed",
    "is_default": true,
    "enabled": true,
    "config": "{\"root\":\".cache/uploadfile\"}"
  }'
```

### 删除 storage

```bash
curl -X DELETE "$BASE_URL/sys/upload/storages/local2" \
  -H "Authorization: Bearer $TOKEN"
```

默认 storage 不能删除；被文件或 scene 使用的 storage 不能删除。

## Share API

### 创建分享

```bash
curl -X POST "$BASE_URL/sys/upload/shares" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_id": 1,
    "password": "secret",
    "expires_at": "2026-06-09 10:00:00",
    "max_downloads": 3
  }'
```

响应 `data.token` 是分享 token。

### 查询分享

```bash
curl "$BASE_URL/sys/upload/shares?file_id=1&status=active" \
  -H "Authorization: Bearer $TOKEN"
```

### 禁用分享

```bash
curl -X DELETE "$BASE_URL/sys/upload/shares/1" \
  -H "Authorization: Bearer $TOKEN"
```

### 查询分享元数据

```bash
curl "$BASE_URL/public/upload/shares/<token>"
```

### 验证分享密码

```bash
curl -X POST "$BASE_URL/public/upload/shares/<token>/verify" \
  -H "Content-Type: application/json" \
  -d '{"password":"secret"}'
```

响应：

```json
{
  "download_token": "<token>"
}
```

### 下载分享文件

密码分享：

```bash
curl -L "$BASE_URL/public/upload/shares/<token>/download?download_token=<download_token>" \
  -o file.bin
```

无密码分享：

```bash
curl -L "$BASE_URL/public/upload/shares/<token>/download" \
  -o file.bin
```

下载成功会增加 `download_count`。超过 `max_downloads` 会拒绝下载。

## 清理任务

命令：

```bash
admin uploadfile cleanup
admin uploadfile cleanup --dry-run
```

输出：

```text
expired_refs=1 pending_files=1 deleted_files=2 dry_run=false
```

字段含义：

- `expired_refs`：到期的临时 ref 数量。
- `pending_files`：超过 pending TTL 的直传对象数量。
- `deleted_files`：实际删除或 dry-run 预计删除的文件对象数量。
- `dry_run`：是否只预览不修改。

任务类型：

```text
uploadfile.cleanup
```

## 常见错误

- `400 file size exceeds scene limit`：超过 scene 的 `max_size`。
- `400 file extension is not allowed`：扩展名不在 scene allow list 内。
- `400 file MIME type is not allowed`：MIME 不在 scene allow list 内。
- `400 upload total byte quota exceeded`：超过全局字节配额。
- `400 upload owner byte quota exceeded`：超过 owner 字节配额。
- `400 upload total file quota exceeded`：超过全局文件数配额。
- `400 upload owner file quota exceeded`：超过 owner 文件数配额。
- `403 file owner is not allowed`：普通用户操作了不属于自己的 owner 或文件。
- `403 file is not public`：访问私有文件时没有有效 download token。
- `404 file not found`：文件不存在。
- `404 storage backend not found`：storage 配置存在，但没有可用后端。

## 接入建议

- 业务对象关联文件时，优先使用 `subject_type`、`subject_id`、`field` 建立 ref。
- 用户隔离使用默认 `owner_type=user`、`owner_id=<用户ID>`；不要把 owner 只存在业务表里。
- 临时上传用于“先传文件、后保存业务表单”的场景，保存业务成功后调用 bind。
- 大文件或浏览器直传对象存储时使用 presigned direct upload。
- 管理端容量展示使用 `GET /sys/upload/stats`。
- 定期运行 `uploadfile.cleanup`，清理过期临时文件和 stale pending 直传对象。
