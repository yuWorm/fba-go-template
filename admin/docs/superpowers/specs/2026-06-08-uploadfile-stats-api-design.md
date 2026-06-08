# Uploadfile Stats API Design

## Goal

Expose backend upload usage statistics so administrators and future UI pages can inspect capacity by scene, storage, provider, status, and owner.

## API

Add:

```text
GET /api/v1/sys/upload/stats
```

Query parameters:

- `scene_code`
- `storage_code`
- `provider`
- `status`
- `owner_type`
- `owner_id`

Response data:

```json
{
  "files": 2,
  "bytes": 12
}
```

The endpoint uses `plugin.Auth()` and the `uploadfile` tag. It does not require a write permission.

## Scope Rules

Stats use the same owner contract as file list:

- super admins can query any owner filter or omit owner filters for global stats;
- normal users are scoped to their default `user/<id>` owner;
- normal users supplying a foreign owner filter receive zero usage;
- deleted refs do not make an object visible to owner or scene stats;
- deleted file objects are excluded from all capacity stats.

The service returns the same `files` and `bytes` shape for all filters. Grouped breakdowns are intentionally out of scope for this iteration; clients can request one filter at a time.

## Data Model

No new table is needed. The repository extends `UploadUsage` filters:

- object-level filters: `provider`, `storage_code`, `status`;
- ref-level filters: `scene_code`, `owner_type`, `owner_id`.

When ref filters are present, usage counts distinct file objects that have at least one matching non-deleted ref.

## Tests

Coverage should include:

- memory and GORM repository usage by scene/storage/status/owner;
- service stats scoping for normal users and super admins;
- API route registration and response contract.
