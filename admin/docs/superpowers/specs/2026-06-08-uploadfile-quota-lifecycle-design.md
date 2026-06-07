# Uploadfile Quota And Lifecycle Design

## Goal

Prevent uploadfile storage from growing without bounds and make lifecycle cleanup behavior explicit for task/command users.

## Scope

This iteration adds backend-only quota enforcement and keeps the existing cleanup task architecture. It does not add UI, tenant/dept quotas, scheduler seed rows, or a new quota table.

## Quota Model

Quota limits are runtime service options loaded from `UPLOADFILE_*` configuration:

- `UPLOADFILE_MAX_TOTAL_BYTES`: maximum bytes across all non-deleted file objects.
- `UPLOADFILE_MAX_OWNER_BYTES`: maximum bytes for one owner scope.
- `UPLOADFILE_MAX_TOTAL_FILES`: maximum non-deleted file object count.
- `UPLOADFILE_MAX_OWNER_FILES`: maximum non-deleted file object count for one owner scope.

A value of `0` disables that limit.

Owner quota uses the ref-level owner contract:

- count distinct objects that have at least one non-deleted ref with the owner pair;
- do not double count a file that has multiple live refs for the same owner;
- ignore deleted file objects and deleted refs.

Total quota counts all file objects whose status is not `deleted`, including `pending` direct uploads because they reserve storage capacity.

## Enforcement

The service resolves and validates the target owner before touching storage. Multipart upload and presigned direct-upload creation both enforce quota before writing an object or requesting a presigned URL.

For a normal user, the owner quota is the default `user/<id>` owner. For a super admin, owner quota is enforced when the upload declares an owner. If no owner is declared by a super admin, only total quota applies.

Quota checks are best-effort and non-transactional. They stop ordinary overuse but do not provide strict concurrent reservations. A future reservation table can harden this if the plugin needs high-concurrency quota guarantees.

## Cleanup Lifecycle

The existing `uploadfile.cleanup` task and `uploadfile cleanup [--dry-run]` command stay as the lifecycle entry points.

Cleanup deletes:

- expired temporary refs and their physical file when no other live refs remain;
- stale pending direct-upload objects older than `UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS`.

The command output remains compact counters so it is usable in logs and scheduled task output. Quota enforcement reduces the need for cleanup to act as the only capacity control.

## Error Semantics

Quota violations return a bad-request service error with a specific message:

- `upload total byte quota exceeded`
- `upload owner byte quota exceeded`
- `upload total file quota exceeded`
- `upload owner file quota exceeded`

These are validation failures for the attempted upload, not authorization failures.

## Tests

Coverage should include:

- env loading for all quota keys;
- memory and GORM repository usage stats, including deleted object/ref exclusion and distinct object counting;
- multipart upload rejects total byte/file quota overflow before storage write;
- direct-upload presign rejects owner byte/file quota overflow before presign;
- plugin env config wires quota settings into the provided admin upload backend.
