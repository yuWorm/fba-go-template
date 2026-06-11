# Admin Template

## Source Map

- Template root: `templates/fba-go-template/admin`
- Runtime: `templates/fba-go-template/admin/internal/runtime`
- Module registration: `templates/fba-go-template/admin/internal/app/register.go`
- Built-in app modules: `internal/app/admin`, `config`, `dict`, `notice`
- Reference plugins: `plugins/email`, `plugins/oauth2`, `plugins/task`
- Local module file: `admin/go.mod`
- Generated module template: `admin/go.mod.tmpl`
- Generated project manifest template: `admin/fbago.yaml.tmpl`
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

When template-owned app or plugin source changes shape, check whether
`fbago.yaml.tmpl` should also change. The generated manifest is what future
`fbago template diff/update` commands use to decide which `internal/app/*` and
`plugins/*` paths are template-managed.

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

## Managed Manifest

`admin/fbago.yaml.tmpl` must list every template-owned app and reference plugin
that generated projects should be able to diff or update later.

Rules:

- Keep app modules under `internal/app`; do not move business modules elsewhere
  to support template updates.
- Use `kind: app` for `internal/app/*` modules and `kind: plugin` for `plugins/*`.
- Use `mode: source` for paths that the template may manage in generated projects.
- Keep `path` as the generated project path and `source_path` as the template path.
- When adding a built-in app/plugin, add a managed entry in the same change.
- When removing a built-in app/plugin, remove the managed entry so generated
  projects can report `D` changes and require explicit `--force` before deletion.
- Document project-side customizations with `mode: manual` in generated projects
  rather than deleting the manifest entry; this preserves origin traceability.

## Data Modes

Modules should support memory mode when practical. When database config is present, modules should switch to GORM repositories and register migrations.

This lets the template run tests without external services and still behave like production when configured.

## Verification

Use the template Makefile:

```bash
make -C templates/fba-go-template/admin L=1 test
```

If scaffold behavior changes, generate a project and test it too.
