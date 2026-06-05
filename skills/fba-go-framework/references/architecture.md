# FBA Go Architecture

## Source Map

- Public facade: `fba.go`
- Application runtime: `core/app/application.go`
- HTTP and middleware setup: `core/fiberx/app.go`, `core/middleware/*`
- Dependency injection: `core/di/container.go`
- Plugin contracts: `core/plugin/module.go`, `core/plugin/context.go`, `core/plugin/registry.go`
- Route mounting and RBAC bridge: `core/plugin/route.go`, `core/plugin/mount.go`, `core/rbac/*`
- Database provider: `core/db/*`
- Migrations: `core/migration/*`
- Realtime: `core/realtime/*`
- Official admin runtime composition: `templates/fba-go-template/admin/internal/runtime/runtime.go`

## Runtime Shape

`fba.go` intentionally exposes a small facade:

- `LoadOptionsFromEnv`
- `LoadOptionsFromEnvFile`
- `NewApplication`
- aliases for `Application`, `Options`, and hooks

Core application creation lives in `core/app.New`. It builds:

- a Fiber app via `fiberx.New`
- core observability routes
- a DI container
- realtime hub and online store
- optional Redis-backed realtime broadcaster
- startup and shutdown hooks

The core application does not know which business modules exist. It owns infrastructure and exposes `Application.HTTP()` and `Application.Container()`.

## Template Runtime Composition

The admin template runtime composes the framework:

1. Load options from `.env` and environment variables.
2. Create `fba.Application`.
3. Open the database if configured and provide `db.Provider` into DI.
4. Build a `plugin.Registry`.
5. Register built-in app modules and plugins.
6. Create `plugin.RuntimeContext` with container, root router, API group, config, and logger.
7. Call `registry.RegisterAll`.
8. Mount collected routes onto `cfg.App.APIBasePath`.
9. Execute CLI commands with default command `server`.

This boundary matters: plugins declare capabilities, but runtime decides when to mount routes, run migrations, and execute commands.

## Plugin Context Contract

Plugins interact through `plugin.Context`:

- `Container()` resolves or provides services.
- `Router()` exposes the root Fiber router.
- `APIGroup()` exposes the versioned API group.
- `Config()` exposes resolved config.
- `Provide()` registers constructors into DI.
- `Route()` collects a route declaration.
- `Task()` collects a task declaration.
- `Migration()` collects a migration.
- `Command()` collects a CLI command.
- `Swagger()` collects an OpenAPI fragment.

`RuntimeContext` stores these declarations and returns defensive copies through `Routes`, `Tasks`, `Migrations`, `Commands`, and `SwaggerFragments`.

## Plugin Registry

`plugin.Registry` stores `plugin.Module` entries by ID and mode.

Modes:

- `ModeAuto`: register automatically.
- `ModeDisabled`: skip registration and fail required dependents.
- `ModePureDependency`: available as dependency metadata but not auto-registered by `RegisterAll`.

`Resolve` topologically sorts dependencies and reports missing dependencies, disabled required dependencies, and cycles.

## Routing and RBAC

Routes are declared with helpers:

- `plugin.GET`
- `plugin.POST`
- `plugin.PUT`
- `plugin.DELETE`

Auth metadata is attached with:

- `plugin.Auth()`
- `plugin.Perm("permission:code")`
- `plugin.Superuser()`

`MountRoutes` wraps protected routes with an authenticator resolved from DI or provided explicitly. Auth-only routes require a current user but do not apply permission checks unless the route declares RBAC metadata.

## Migrations

Core migration contracts live under `core/migration`. The admin runtime uses a `GORMStore` and `Runner` over migrations collected from plugins. A migration is registered by calling `ctx.Migration(...)` during plugin registration.

Database-aware plugins should register migrations only when a usable `db.Provider` is available. Memory-only fallback should not register GORM migrations.

## Extension Boundary

Use core packages for stable, cross-project contracts. Use template runtime for composition. Use plugins and app modules for business features.

Do not add business-specific behavior to core unless multiple generated projects need the same stable contract.
