# Uploadfile Owner Contract

## Goal

Define how uploadfile decides who owns, can query, and can mutate files when a single physical object may be attached to multiple business scenes.

## Model

Uploadfile keeps physical storage metadata and business ownership separate:

- `upload_file_object` is the physical file object. `uploaded_by` records who uploaded it and remains a fallback access grant.
- `upload_file_ref` is the business attachment record. `owner_type` and `owner_id` live here because the same file can be bound to multiple scenes, subjects, fields, and owners.

This keeps multi-scene queries correct. Ownership is tied to the attachment/ref that makes the file relevant to a business object, not only to the raw stored object.

## Actor Scope

The current service actor has:

- `UserID`
- `IsSuperAdmin`

For a non-super-admin user with `UserID=7`, the default owner scope is:

```text
owner_type=user
owner_id=7
```

Super admins can declare and query any owner. Non-super-admin users can only declare or query their own default owner scope.

The admin runtime also exposes `DeptID` on `rbac.CurrentUser`. Uploadfile does not yet auto-grant dept ownership; future work can add explicit actor owner scopes such as `dept/<dept_id>` without changing the ref-level storage model.

## Create And Bind Rules

Multipart upload, direct-upload presign, and bind all use the same owner rules:

- If no owner is supplied, non-super-admin users default to `user/<current_user_id>`.
- If an owner is supplied, non-super-admin users may only supply their own `user/<current_user_id>` owner.
- Super admins may supply any owner pair.
- Owner values are stored on refs.

## Query Rules

File and ref list operations for non-super-admin users are scoped to the actor owner.

If a non-super-admin user supplies a foreign owner filter, the result is empty instead of silently returning the caller's own files. This makes query behavior explicit and avoids a caller thinking they queried another owner when the service rewrote the filter.

Super admins are not owner-scoped and can use explicit owner filters.

Object list filters that join through refs must ignore deleted refs. Deleted refs must not make a file visible in scene/owner queries.

## Access Rules

Object access is allowed when any of these is true:

- The actor is a super admin.
- The actor is `uploaded_by` for the object.
- The object has at least one non-deleted ref whose owner matches the actor owner scope.

Deleted refs do not authorize access. This matters for files that were once attached to a user but were later unbound or cleaned up.

## Error Semantics

Mutation APIs return `403` when the actor tries to mutate a file outside its owner scope.

List APIs return an empty page when the actor supplies a foreign owner filter.

Public token/share access remains token-based and does not use actor owner scope.

## Test Coverage

Owner contract coverage should include:

- default owner assignment on upload;
- explicit foreign owner rejection on upload/presign/bind for normal users;
- normal user list with no owner filter returns own owner scope;
- normal user list with foreign owner filter returns empty;
- deleted refs do not authorize access;
- deleted refs do not make objects match scene/owner list filters;
- super admin can query arbitrary owner filters.
