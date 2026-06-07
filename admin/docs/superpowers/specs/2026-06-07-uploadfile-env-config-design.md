# Uploadfile Env Config Design

## Goal

Allow the uploadfile plugin to read `UPLOADFILE_*` startup configuration from the process environment and `.env`, then apply it to the default upload storage, default scene seed data, and uploadfile lifecycle service options.

## Boundaries

Uploadfile keeps storage and scene configuration in `upload_storage` and `upload_scene`. Environment configuration affects seed/default data at plugin startup and initial migration time. Existing database rows are inserted with `ON CONFLICT DO NOTHING`, so environment values do not overwrite rows already managed through the API. Lifecycle TTL values are runtime service options and take effect when the plugin registers.

The implementation stays inside `plugins/uploadfile` and does not add uploadfile fields to core `config.Options`.

## Configuration Source

The loader reads dotenv values from `FBA_ENV_FILE` when set, otherwise `.env`. Real process environment variables override dotenv values, matching the core config loader precedence.

Supported variables:

- `UPLOADFILE_STORAGE_PROVIDER`: `local`, `s3`, or `oss`; default `local`.
- `UPLOADFILE_STORAGE_PREFIX`: object key prefix; default `uploads`.
- `UPLOADFILE_STORAGE_BASE_URL`: public base URL for generated object URLs.
- `UPLOADFILE_LOCAL_ROOT`: local storage root, serialized into local storage `config`.
- `UPLOADFILE_STORAGE_BUCKET`: S3/OSS bucket.
- `UPLOADFILE_STORAGE_REGION`: S3/OSS region.
- `UPLOADFILE_STORAGE_ENDPOINT`: S3/OSS compatible endpoint.
- `UPLOADFILE_S3_FORCE_PATH_STYLE`: S3 `force_path_style` config boolean.
- `UPLOADFILE_OSS_USE_PATH_STYLE`: OSS `use_path_style` config boolean.
- `UPLOADFILE_OSS_USE_CNAME`: OSS `use_cname` config boolean.
- `UPLOADFILE_DEFAULT_MAX_SIZE`: default scene max size in bytes.
- `UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS`: default scene temporary reference TTL in seconds.
- `UPLOADFILE_DEFAULT_ALLOWED_EXTS`: default scene allowed extensions as JSON array or comma-separated values.
- `UPLOADFILE_DEFAULT_ALLOWED_MIMES`: default scene allowed MIME types as JSON array or comma-separated values.
- `UPLOADFILE_DOWNLOAD_TOKEN_TTL_SECONDS`: default private/share download token TTL in seconds.
- `UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS`: maximum private file access token TTL in seconds.
- `UPLOADFILE_DIRECT_UPLOAD_PRESIGN_TTL_SECONDS`: default direct-upload presigned PUT TTL in seconds.
- `UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS`: cleanup grace period for stale pending direct-upload objects in seconds.

Cloud credentials are intentionally not written into uploadfile storage config. S3 continues to use the AWS SDK default credential chain. OSS continues to use `OSS_ACCESS_KEY_ID`, `OSS_ACCESS_KEY_SECRET`, and `OSS_SESSION_TOKEN`.

## Data Flow

`uploadfile.Module.Register` loads options, applies them to `repo.SeedData()`, and uses that configured seed for the memory repository fallback. When a database provider exists, the same seed is passed to the uploadfile initial-data migration. Runtime lifecycle values are mapped into `service.Options` and `service.CleanupOptions`.

The storage factory layer does not change. The configured seed produces the same fields that already feed `storage.BackendConfig`.

## Error Handling

Invalid numeric or boolean values fail plugin registration. Invalid `UPLOADFILE_STORAGE_PROVIDER` fails plugin registration. Invalid JSON arrays for allowed extensions/MIMEs fail plugin registration. Empty values are ignored and defaults are preserved.

## Testing

Add unit tests for loading dotenv values, environment override precedence, seed application for local and S3/OSS config JSON, and invalid config errors. Add plugin/migration tests that prove configured seed values are used by memory registration and initial-data migration.
