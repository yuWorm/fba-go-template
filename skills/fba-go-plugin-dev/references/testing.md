# Plugin Testing

## Test Levels

Use the smallest level that proves the behavior:

- `Module` tests: plugin metadata, registration, route count, dependency assumptions.
- Service tests: business behavior, validation, Python-compatible edge cases.
- Repository tests: memory and GORM behavior, filters, soft delete, pagination.
- Migration tests: tables, seed data, idempotent initial data.
- API tests: route handler request and response contract.
- Runtime tests: command aggregation, migration execution, default server behavior.

## Standard Commands

Template tests:

```bash
make -C templates/fba-go-template/admin L=1 test
```

Root tests:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./...
```

Targeted plugin example:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./templates/fba-go-template/admin/plugins/task/...
```

When running inside the admin template directory, use `make L=1 test` so `go.local.mod` points at the local FBA Go checkout.

## Python Alignment Tests

For migrated behavior, use Python tests and source as reference:

- copy endpoint paths and permission expectations
- align DTO field names and enum values
- align seed data values where the frontend depends on IDs or codes
- align response envelopes and status messages

The goal is not to copy Python implementation style. The goal is API and behavior compatibility.

## Database Testing

Memory repository tests are fast but do not prove GORM query behavior. For GORM repositories, use SQLite for generic behavior and real MySQL/PostgreSQL where SQL dialect matters.

MySQL-specific seed SQL, JSON fields, charset behavior, and reserved words need MySQL coverage when changed.

## Completion Checklist

- Tests cover the changed contract.
- `go test` or `make L=1 test` output was read and exit code was zero.
- Generated files, SQL seeds, and docs are included if behavior changed.
- No unrelated template submodule changes are staged accidentally.
