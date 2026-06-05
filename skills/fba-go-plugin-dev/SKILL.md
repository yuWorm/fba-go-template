---
name: fba-go-plugin-dev
description: Create, modify, review, or document FBA Go plugins and app modules. Use when working with plugin.Module, FBAPlugin factories, plugin.Context registration, routes, RBAC permissions, migrations, repositories, services, plugin commands, task integration, plugin manifests, generated registration, or admin template plugins under templates/fba-go-template/admin/plugins and internal/app modules.
---

# FBA Go Plugin Development

## Workflow

Use this skill to build plugins that fit the FBA Go capability model.

1. Decide whether the feature is a project-owned app module or a reusable plugin. Business domains owned by the generated project belong under `internal/app`; reusable integrations belong under `plugins`.
2. Implement `FBAPlugin() plugin.Module`, `Meta() plugin.Meta`, and `Register(ctx plugin.Context) error`.
3. Keep registration declarative. Use `ctx.Provide`, `ctx.Route`, `ctx.Migration`, `ctx.Command`, `ctx.Task`, and `ctx.Swagger`; avoid reaching around the context.
4. Prefer repo/service/api/migration package boundaries from the admin template.
5. Align route paths, response envelopes, model fields, and seed data with `sources/fastapi-best-architecture/` when migrating Python behavior.
6. Add tests at module, service, repo, route, and migration levels according to the risk of the change.

## Load References

- Read `references/plugin-contract.md` before changing `Module`, `Meta`, dependencies, modes, registry, or generated registration.
- Read `references/routes-rbac-response.md` before adding handlers, routes, permissions, auth, or response behavior.
- Read `references/migrations.md` before adding tables, seed data, SQL migrations, or Python-aligned initialization.
- Read `references/commands.md` before adding plugin CLI commands.
- Read `references/task-integration.md` before adding scheduler or async task behavior.
- Read `references/testing.md` before finishing plugin work.

## Guardrails

- Plugin IDs must be stable and unique.
- Register dependencies in `Meta().DependsOn`; do not rely on import order.
- Use memory repository fallback when the plugin can run without a database, and switch to GORM plus migrations when `db.Provider` is available.
- Use `response.Success` for success responses and return errors for middleware mapping.
- Use `plugin.Auth`, `plugin.Perm`, and `plugin.Superuser` route options instead of inline auth logic.
- Keep plugin commands side-effect scoped and testable through `core/command`.

## Verification

Use targeted package tests during development, then run:

```bash
make -C templates/fba-go-template/admin L=1 test
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./...
```
