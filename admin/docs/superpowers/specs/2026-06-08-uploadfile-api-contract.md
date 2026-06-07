# Uploadfile API Contract

## Scope

This document records the current uploadfile HTTP and command contract. It is not a UI spec. Routes are mounted under `/api/v1` in the admin template and return the core response envelope for JSON APIs:

```json
{
  "code": 200,
  "data": {}
}
```

Download routes stream file bytes and set `Content-Type` and `Content-Disposition` when file metadata is available.

## Authenticated File APIs

### Multipart Upload

`POST /api/v1/sys/upload/files`

Auth: required. Permission: `sys:upload:file:add`.

Request: multipart form with `file` and optional fields:

- `scene_code`: defaults to `default`.
- `field`
- `subject_type`
- `subject_id`
- `owner_type`
- `owner_id`
- `temp`: boolean string.

Response: `UploadResult` with `file` and `ref`.

### Presigned Direct Upload

`POST /api/v1/sys/upload/files/presign`

Auth: required. Permission: `sys:upload:file:add`.

Body:

```json
{
  "filename": "direct.txt",
  "content_type": "text/plain",
  "size": 6,
  "scene_code": "default",
  "field": "attachment",
  "subject_type": "notice",
  "subject_id": "1001",
  "owner_type": "user",
  "owner_id": "42",
  "temp": true,
  "ttl_seconds": 900
}
```

Behavior:

- Validates scene, file extension, MIME, and size before issuing the presigned URL.
- Creates a file object with `status=pending`.
- Creates a file ref using the same temp/owner rules as multipart upload.
- Returns the target file/ref and a presigned PUT URL with required headers.

Response fields:

- `file`: pending file detail.
- `ref`: created ref detail.
- `presigned.method`: expected `PUT`.
- `presigned.url`: URL clients upload bytes to.
- `presigned.expires_at`: expiration formatted as `2006-01-02 15:04:05`.
- `presigned.headers`: headers the client should send, including content type when required by backend signing.

### Complete Direct Upload

`POST /api/v1/sys/upload/files/:pk/complete`

Auth: required. Permission: `sys:upload:file:add`.

Behavior:

- Verifies the caller can access the file owner scope.
- Rejects deleted files.
- For non-active objects, calls the storage backend `Head` operation before activation.
- Rejects completion when the uploaded object is unavailable.
- Rejects completion when storage size differs from the recorded requested size.
- Rejects completion when both MIME values are available and differ.
- Marks the object `active` only after metadata validation passes.

Error status:

- `400`: deleted file, missing uploaded object, size mismatch, or MIME mismatch.
- `403`: caller is outside the file owner scope.
- `404`: file or storage cannot be found.

### Authenticated Download

`GET /api/v1/sys/upload/files/:pk/download`

Auth: required. Owner scoped.

Streams bytes for an accessible non-deleted file.

### Temporary Private File Access Token

`POST /api/v1/sys/upload/files/:pk/access-token`

Auth: required. Owner scoped.

Body:

```json
{
  "ttl_seconds": 300
}
```

Behavior:

- `ttl_seconds` is optional. When omitted, the service default is used and capped by `UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS`.
- Explicit `ttl_seconds` above the configured max is rejected with `400`.
- The returned `download_token` grants temporary access to the public file URL for private files.

Response:

```json
{
  "download_url": "/api/v1/public/upload/files/<uuid>?download_token=<token>",
  "download_token": "<token>",
  "expires_at": "2026-06-08 10:05:00"
}
```

## Public File APIs

### Public File Download

`GET /api/v1/public/upload/files/:uuid`

Behavior:

- Public visibility files can be streamed without a token.
- Private visibility files require a valid `download_token` query parameter.
- Expired or invalid temporary access tokens return `403`.
- Deleted files return `400`.

## Share APIs

### Create Share

`POST /api/v1/sys/upload/shares`

Auth: required. Permission: `sys:upload:share:add`.

Optional passwords are hashed before persistence. Optional expiration and max download limits are enforced on public download.

### Verify Share Password

`POST /api/v1/public/upload/shares/:token/verify`

Returns a short-lived `download_token` when the password is valid.

### Download Share

`GET /api/v1/public/upload/shares/:token/download`

Password-protected shares require the verified `download_token` query parameter. Public share download increments `download_count`.

## Cleanup Command And Task

Task type: `uploadfile.cleanup`

Command:

```bash
admin uploadfile cleanup [--dry-run]
```

Output:

```text
expired_refs=0 pending_files=0 deleted_files=0 dry_run=false
```

Counters:

- `expired_refs`: temporary refs whose `expires_at` was reached.
- `pending_files`: stale direct-upload file objects still in `status=pending`.
- `deleted_files`: physical/object records deleted or projected for deletion.
- `dry_run`: `true` when no mutation was applied.

## Runtime Configuration

Storage and scene defaults:

- `UPLOADFILE_STORAGE_PROVIDER`
- `UPLOADFILE_STORAGE_PREFIX`
- `UPLOADFILE_STORAGE_BASE_URL`
- `UPLOADFILE_LOCAL_ROOT`
- `UPLOADFILE_STORAGE_BUCKET`
- `UPLOADFILE_STORAGE_REGION`
- `UPLOADFILE_STORAGE_ENDPOINT`
- `UPLOADFILE_S3_FORCE_PATH_STYLE`
- `UPLOADFILE_OSS_USE_PATH_STYLE`
- `UPLOADFILE_OSS_USE_CNAME`
- `UPLOADFILE_DEFAULT_MAX_SIZE`
- `UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS`
- `UPLOADFILE_DEFAULT_ALLOWED_EXTS`
- `UPLOADFILE_DEFAULT_ALLOWED_MIMES`

Lifecycle options:

- `UPLOADFILE_DOWNLOAD_TOKEN_TTL_SECONDS`: default private/share download token TTL in seconds.
- `UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS`: maximum explicit private file access token TTL.
- `UPLOADFILE_DIRECT_UPLOAD_PRESIGN_TTL_SECONDS`: default presigned PUT TTL for direct upload.
- `UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS`: cleanup grace period for stale pending direct-upload objects.

Quota options:

- `UPLOADFILE_MAX_TOTAL_BYTES`: maximum bytes across all non-deleted upload objects; `0` disables the limit.
- `UPLOADFILE_MAX_OWNER_BYTES`: maximum bytes for one ref owner scope; `0` disables the limit.
- `UPLOADFILE_MAX_TOTAL_FILES`: maximum non-deleted upload object count; `0` disables the limit.
- `UPLOADFILE_MAX_OWNER_FILES`: maximum non-deleted upload object count for one ref owner scope; `0` disables the limit.
