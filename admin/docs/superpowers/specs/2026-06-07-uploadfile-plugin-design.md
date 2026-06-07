# Uploadfile Plugin Design

Date: 2026-06-07
Status: Draft

## Goal

Build a reusable `uploadfile` plugin for the FBA Go admin template. The plugin provides unified file upload, sharing, querying, and management across local and object-storage backends.

The first implementation should solve the core admin/backend workflow:

- Upload files through the application server.
- Store file metadata in the database.
- Support temporary files before a business object exists.
- Bind uploaded files to one or more business scenarios.
- Query files by scenario and business object.
- Share files with token, optional password, expiration, and download limits.
- Keep the storage backend replaceable.
- Leave a clear owner model for later personal-space, department, or tenant isolation.

The first implementation should not become a full file permission system or personal drive.

## Existing Context

The admin template currently has `POST /sys/files/upload` in the admin module. That endpoint only validates filename and size, then returns a Python-compatible `/static/upload/<name>_<timestamp>.<ext>` URL. It does not persist metadata, store file bytes, support storage backends, share files, or manage references.

The new feature belongs under `plugins/uploadfile` because it is a reusable integration, not project-owned business logic. It should follow the existing plugin pattern:

- `FBAPlugin() plugin.Module`
- declarative `Register(ctx plugin.Context) error`
- `api`, `dto`, `model`, `repo`, `service`, `storage`, and `migration` packages
- memory repository fallback
- GORM repository and migrations when `db.Provider` is available
- route declarations through `plugin.RegisterRoutes`
- success responses through `response.Success`

The plugin should initially expose its own `/upload/*` routes to avoid conflicting with the existing admin `/sys/files/upload` route. A later compatibility step can bridge the existing route to the plugin service.

## Design Principles

### Separate Physical Files From Business References

Use a Rails Active Storage-style split:

- `upload_file_object` stores the physical object and metadata.
- `upload_file_ref` stores how business records refer to files.

This avoids a single table with overloaded `scene`, `biz_id`, and `owner` columns. It also supports temporary uploads, many attachments per business record, reusing one file across multiple scenes, and reliable cleanup.

### Keep Storage Behind an Interface

Business code should not call OSS, S3, or local filesystem APIs directly. A `storage.Backend` interface should hide backend-specific behavior.

First implementation supports `local`. Later implementations add `s3` and `oss` without changing service or API contracts.

### Keep Owner Optional But Designed

The first version records `uploaded_by` and `created_by` for audit. It also reserves `owner_type` and `owner_id` on the reference model for future user, department, tenant, or system ownership.

The plugin should not enforce complex owner ACLs in the first version. Protected admin routes use existing RBAC. Business routes should query by scene and subject identity.

## Data Model

### `upload_file_object`

Physical object metadata.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | int | primary key |
| `uuid` | string | public stable id |
| `storage_code` | string | selected storage config |
| `provider` | string | `local`, `s3`, `oss` |
| `bucket` | string nullable | bucket/container for object storage |
| `object_key` | string | canonical object path/key |
| `original_name` | string | sanitized original filename |
| `ext` | string | lowercase extension without dot |
| `mime` | string | detected or submitted MIME type |
| `size` | int64 | bytes |
| `sha256` | string nullable | content digest when available |
| `etag` | string nullable | backend ETag when available |
| `visibility` | string | `private` or `public` |
| `status` | string | `pending`, `active`, `deleted` |
| `uploaded_by` | int nullable | authenticated user id if available |
| `created_time` | time | auto create |
| `updated_time` | time nullable | auto update |
| `deleted_time` | time nullable | soft delete marker |

Indexes:

- unique `uuid`
- unique `storage_code + object_key`
- `status + created_time`
- `uploaded_by + created_time`
- `sha256` for optional de-duplication/search

### `upload_file_ref`

Business scenario reference.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | int | primary key |
| `file_id` | int | references object id |
| `scene_code` | string | `avatar`, `notice_attachment`, etc. |
| `subject_type` | string nullable | business type, for example `user`, `notice` |
| `subject_id` | string nullable | string to support numeric ids and UUIDs |
| `field` | string nullable | `avatar`, `attachments`, `cover` |
| `display_name` | string nullable | user-facing name override |
| `sort` | int | ordering within field |
| `status` | string | `temp`, `active`, `deleted` |
| `expires_at` | time nullable | temporary cleanup deadline |
| `owner_type` | string nullable | future owner scope: `user`, `dept`, `tenant`, `system` |
| `owner_id` | string nullable | owner identifier |
| `created_by` | int nullable | authenticated user id if available |
| `metadata` | json/text nullable | scenario-specific metadata |
| `created_time` | time | auto create |
| `updated_time` | time nullable | auto update |
| `deleted_time` | time nullable | soft delete marker |

Indexes:

- `file_id + status`
- `scene_code + subject_type + subject_id + field + status`
- `status + expires_at`
- `owner_type + owner_id + status`
- `created_by + created_time`

### `upload_scene`

Scenario configuration.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | int | primary key |
| `code` | string | unique scene code |
| `name` | string | human name |
| `max_size` | int64 | max bytes |
| `allowed_exts` | json/text | extension allowlist |
| `allowed_mimes` | json/text | MIME allowlist |
| `default_storage_code` | string nullable | storage override |
| `default_visibility` | string | `private` or `public` |
| `temp_ttl_seconds` | int | default temp expiration |
| `path_template` | string | object key template |
| `enabled` | bool | availability |
| `created_time` | time | auto create |
| `updated_time` | time nullable | auto update |

Seed scenes:

- `default`: general files, conservative max size.
- `avatar`: images only, small max size, public or private depending current frontend needs.
- `attachment`: documents/images/video within default admin limits.

### `upload_storage`

Storage backend configuration.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | int | primary key |
| `code` | string | unique storage code |
| `provider` | string | `local`, `s3`, `oss` |
| `bucket` | string nullable | bucket/container |
| `region` | string nullable | object storage region |
| `endpoint` | string nullable | custom endpoint |
| `base_url` | string nullable | public/CDN base URL |
| `prefix` | string | object key prefix |
| `is_default` | bool | default backend |
| `enabled` | bool | availability |
| `config` | json/text nullable | non-secret options |
| `created_time` | time | auto create |
| `updated_time` | time nullable | auto update |

Secrets are not stored directly. S3 and OSS credentials should come from environment variables or secret references in later implementations.

Seed storage:

- `local`: provider `local`, prefix `uploads`, enabled, default.

### `upload_share`

Share records.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | int | primary key |
| `file_id` | int | required |
| `ref_id` | int nullable | optional scenario-specific share |
| `token` | string | random unique share token |
| `password_hash` | string nullable | optional password hash |
| `expires_at` | time nullable | share expiration |
| `max_downloads` | int nullable | optional max count |
| `download_count` | int | current count |
| `status` | string | `active`, `disabled`, `expired` |
| `created_by` | int nullable | authenticated user id |
| `created_time` | time | auto create |
| `updated_time` | time nullable | auto update |

Indexes:

- unique `token`
- `file_id + status`
- `created_by + created_time`
- `status + expires_at`

## Owner Model

Owner is intentionally not the primary relationship between files and business records.

Use these meanings:

- `uploaded_by` on `upload_file_object`: who uploaded the bytes.
- `created_by` on `upload_file_ref` and `upload_share`: who created the business reference or share.
- `subject_type + subject_id` on `upload_file_ref`: which business object owns the attachment in a business sense.
- `owner_type + owner_id` on `upload_file_ref`: optional future owner scope for personal files, departments, tenants, or system-owned files.

First-version rules:

- Store `uploaded_by` and `created_by` when the authenticated user is available.
- Set `owner_type = "user"` and `owner_id = current_user.id` only when an upload request explicitly asks for user-owned files or when the scene configuration requires it.
- Do not enforce owner ACL in the service layer yet.
- Use route RBAC for admin management.
- Use `scene_code + subject_type + subject_id + field` for business attachment queries.

Future owner-aware rules:

- Personal file center: query `owner_type=user AND owner_id=current_user.id`.
- Tenant isolation: require `owner_type=tenant` or add a dedicated `tenant_id` if tenancy becomes a core concept.
- Department files: use `owner_type=dept`.
- System files: use `owner_type=system`.

## Storage Backend Interface

Define a backend interface similar to:

```go
type Backend interface {
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (PresignedURL, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error)
	PublicURL(key string) string
}
```

First implementation:

- `local` writes under a configured root, default `.cache/uploadfile`.
- Path traversal is rejected by cleaning and validating object keys.
- `PublicURL` can return `/uploads/<key>` only if the route/static serving story is added later. For first version, downloads should go through plugin routes for private behavior.

Later implementations:

- `s3` uses AWS SDK for Go v2 and supports custom endpoint for S3-compatible services.
- `oss` uses Alibaba Cloud OSS Go SDK V2.
- Both support presigned upload/download.

## File Key Strategy

Default object key format:

```text
{prefix}/{scene_code}/{yyyy}/{mm}/{dd}/{uuid}.{ext}
```

Examples:

```text
uploads/avatar/2026/06/07/01HX...png
uploads/attachment/2026/06/07/01HY...pdf
```

Do not trust client-provided paths. Original filename is metadata only.

## API

Routes should require authentication except public share routes.

### Upload

`POST /upload/files`

Multipart form:

- `file`: required
- `scene_code`: optional, default `default`
- `field`: optional
- `subject_type`: optional
- `subject_id`: optional
- `owner_type`: optional
- `owner_id`: optional
- `temp`: optional bool, default true when no subject is provided

Behavior:

1. Resolve and validate scene.
2. Validate size, extension, and MIME.
3. Generate object key.
4. Store bytes through storage backend.
5. Create `upload_file_object`.
6. Create `upload_file_ref`.
7. Return file detail and ref detail.

### Bind Temporary Files

`POST /upload/files/bind`

Body:

```json
{
  "file_ids": [1, 2],
  "scene_code": "notice_attachment",
  "subject_type": "notice",
  "subject_id": "1001",
  "field": "attachments"
}
```

Behavior:

- Convert matching temp refs to active refs.
- Create refs if object exists without a matching temp ref.
- Preserve ordering if provided.

### Query References

`GET /upload/refs`

Query:

- `scene_code`
- `subject_type`
- `subject_id`
- `field`
- `status`, default `active`

Returns refs joined with file metadata.

### Manage Files

`GET /upload/files`

Filters:

- filename keyword
- scene_code
- provider
- storage_code
- status
- uploaded_by
- owner_type
- owner_id
- created time range

`GET /upload/files/:pk`

Returns object, refs, and share summary.

`DELETE /upload/files`

Soft deletes objects and refs. Physical delete should happen only when no active refs remain and the caller requested physical deletion or a cleanup task handles it.

### Share

`POST /upload/shares`

Creates share token. Optional password is hashed before storage.

`GET /upload/shares`

Lists shares for management.

`DELETE /upload/shares/:pk`

Disables share.

Public:

- `GET /public/upload/shares/:token`: returns share metadata without file bytes.
- `POST /public/upload/shares/:token/verify`: validates password and returns a short-lived access marker or just success for first version.
- `GET /public/upload/shares/:token/download`: validates status, expiration, password requirement, and download count, then streams file or redirects to presigned URL later.

## Cleanup

Temporary cleanup should be implemented as a service method and later exposed as a command/task:

1. Find `upload_file_ref.status=temp AND expires_at < now`.
2. Mark refs deleted.
3. For each affected file, check active refs.
4. If no active refs remain, mark object deleted and delete physical object if configured.

First version can expose the cleanup service and unit tests without scheduler integration. Task integration can follow after core upload/share behavior is stable.

## Permissions

Initial permission codes:

- `sys:uploadfile:upload`
- `sys:uploadfile:list`
- `sys:uploadfile:detail`
- `sys:uploadfile:delete`
- `sys:uploadfile:bind`
- `sys:uploadfile:share`

Route policy:

- Upload and ref queries require `plugin.Auth()`.
- Admin list/detail/delete/share management require `plugin.Auth()` and matching permission.
- Public share routes are not authenticated but must validate token, password, expiration, status, and max downloads.

## Error Handling

Handlers return errors directly and rely on middleware mapping. Service errors should distinguish:

- validation failure
- scene not found or disabled
- storage not found or disabled
- file not found
- ref not found
- share not found
- share expired
- share password required
- share password invalid
- download limit reached

Avoid leaking physical object keys in public share errors.

## First Implementation Scope

Included:

- Plugin skeleton and registration.
- Models and migrations for all five tables.
- Memory repository.
- GORM repository.
- Local storage backend.
- Multipart upload through server.
- Temporary refs and bind.
- Ref query by scenario and subject.
- File management list/detail/delete.
- Share create/list/disable/public metadata/password/download.
- Tests for service, repo, routes, and migrations.

Deferred:

- S3 backend.
- OSS backend.
- Presigned direct upload.
- Chunked/resumable upload.
- Image variants/thumbnails.
- CDN integration.
- Quota enforcement.
- Full owner ACL.
- Admin frontend pages.
- Bridging existing `/sys/files/upload`.

## Implementation Plan

1. Create `plugins/uploadfile` package layout.
2. Add models and constants.
3. Add repository interface, memory implementation, and seed data.
4. Add GORM repository and pagination helpers.
5. Add migrations and initial SQL seed for menus/permissions/storage/scenes where appropriate.
6. Add local storage backend and storage registry.
7. Add service methods for upload, bind, list refs, list files, delete files, share, public share access, and cleanup.
8. Add DTOs and route handlers.
9. Register module routes and migrations.
10. Add plugin to `internal/app/register.go` if auto-injection is desired in the template.
11. Add targeted tests.
12. Run `make L=1 test`.

## Open Decisions

- Whether `uploadfile` should be auto-injected by default in `internal/app/register.go` immediately.
- Whether the existing admin `/sys/files/upload` route should remain unchanged in the first implementation.
- Whether scene and storage seed data should include menu rows in addition to database rows.
- Whether `owner_type/owner_id` should be accepted on first-version upload requests or only set internally.

Recommended defaults:

- Auto-inject the plugin after implementation tests pass.
- Leave `/sys/files/upload` unchanged for first implementation.
- Seed storage and scenes; add permission/menu seed only if current admin menu conventions can be followed safely.
- Accept owner fields only for authenticated admin callers and validate basic shape, but do not enforce ACL from them yet.
