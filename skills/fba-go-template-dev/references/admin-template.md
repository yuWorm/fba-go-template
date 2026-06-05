# Admin Template

## Source Map

- Template root: `templates/fba-go-template/admin`
- Runtime: `templates/fba-go-template/admin/internal/runtime`
- Module registration: `templates/fba-go-template/admin/internal/app/register.go`
- Built-in app modules: `internal/app/admin`, `config`, `dict`, `notice`
- Reference plugins: `plugins/email`, `plugins/oauth2`, `plugins/task`
- Local module file: `admin/go.mod`
- Generated module template: `admin/go.mod.tmpl`
- Local modfile for root checkout: `admin/go.local.mod`

## Runnable vs Generated Files

The admin template is both:

- a runnable Go project for tests and local development
- a source template used by `fbago init`

Keep this distinction explicit:

- `go.mod` belongs to the runnable template module `github.com/yuWorm/fba-go-template/admin`.
- `go.mod.tmpl` belongs to generated projects and must render `[[ .Module ]]`, `[[ .CoreVersion ]]`, and optional `[[ .CoreReplace ]]`.
- `go.local.mod` supports `make L=1` against the local fba-go checkout.

When dependencies change in `go.mod`, check whether `go.mod.tmpl` should also change.

## Runtime Behavior

`internal/runtime.NewWithOptions` composes:

1. configuration defaults
2. application creation
3. optional database provider
4. plugin registry
5. built-in module registration
6. plugin runtime context
7. plugin route mounting
8. CLI command execution

The default CLI command is `server`. Running `go run ./cmd/api` starts the server.

## Module Layout

Use the existing package shape:

```text
module/
  api/
  dto/
  migration/
  model/
  repo/
  service/
  module.go
  plugin_test.go
```

Not every module needs every package, but avoid mixing handler, repository, and migration logic in one file when the feature grows.

## Built-In Modules

Project-owned app modules:

- `admin`: auth, users, roles, menus, departments, data rules, logs, monitor, files, plugin management.
- `config`: system config APIs and admin config provider.
- `dict`: dictionary types and values.
- `notice`: notices and initial notice data.

Reference plugins:

- `email`: email integration pattern.
- `oauth2`: OAuth2 integration pattern.
- `task`: scheduler and task management pattern.

## Data Modes

Modules should support memory mode when practical. When database config is present, modules should switch to GORM repositories and register migrations.

This lets the template run tests without external services and still behave like production when configured.

## Verification

Use the template Makefile:

```bash
make -C templates/fba-go-template/admin L=1 test
```

If scaffold behavior changes, generate a project and test it too.
