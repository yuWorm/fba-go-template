# Scaffold Generation

## Source Map

- CLI entry: `cmd/fbago/main.go`
- Scaffold init: `cmd/fbago/internal/scaffold/init.go`
- Embedded basic template: `cmd/fbago/internal/scaffold/templates/basic`
- Scaffold tests: `cmd/fbago/internal/scaffold/*_test.go`
- CLI tests: `cmd/fbago/main_test.go`
- Official admin template: `templates/fba-go-template/admin`

## Init Flow

`fbago init` parses:

- module argument
- `--dir`
- `--template`
- `--force`
- `--core-replace`
- `--core-version`

`scaffold.Init` then:

1. validates module name
2. resolves template name or path
3. loads embedded, local, or remote Git template files
4. rejects overwriting existing files unless forced
5. computes template data
6. rewrites template module imports when `.fbago-template.yaml` declares a module
7. renders `.tmpl` files, including `fbago.yaml.tmpl` to project-root `.fbago.yaml`
8. formats generated Go files
9. writes files

## Template Data

Template files can use:

- `[[ .Module ]]`: target module name.
- `[[ .TemplateModule ]]`: source runnable template module.
- `[[ .TemplateName ]]`: resolved template name.
- `[[ .TemplateSource ]]`: `embedded`, `local`, or `remote`.
- `[[ .TemplateRepo ]]`: best-effort Git origin URL for local/remote templates.
- `[[ .TemplateRef ]]`: remote template ref when provided.
- `[[ .TemplateCommit ]]`: best-effort Git commit for local/remote templates.
- `[[ .TemplatePath ]]`: template root path relative to the template repository.
- `[[ .CoreReplace ]]`: optional replace path for local core module.
- `[[ .CoreVersion ]]`: resolved FBA Go core module version.

Keep delimiters as `[[` and `]]` because Go module files and other template content may contain normal braces.

## Project Manifest Rules

Generated projects must include a root `.fbago.yaml` when a template expects
future updates. The file is rendered from `fbago.yaml.tmpl`; the template file is
stored without a leading dot because embedded template directories do not carry
dotfiles reliably.

Manifest v1 records template origin and managed source paths without changing the
current business layout under `internal/app`:

```yaml
version: 1

template:
  name: admin
  module: github.com/your-org/my-admin
  source_module: github.com/yuWorm/fba-go-template/admin
  source: local
  repo: https://github.com/yuWorm/fba-go-template.git
  ref: v0.0.2
  commit: <best-effort-commit>
  template_path: admin
  core_version: v0.0.2

managed:
  - name: admin
    kind: app
    mode: source
    path: internal/app/admin
    source_path: internal/app/admin
```

Template origin fields:

- `template.name`: stable template name, such as `basic` or `admin`.
- `template.module`: generated project module.
- `template.source_module`: runnable template module from `.fbago-template.yaml`.
- `template.source`: `embedded`, `local`, or `remote`.
- `template.repo`: best-effort Git origin URL for local templates, or clone URL for remote templates.
- `template.ref`: remote template ref when the caller specified one.
- `template.commit`: best-effort source commit. This is metadata, not the update selector.
- `template.template_path`: template root path inside the source repository, such as `admin`.
- `template.core_version`: generated core module version.

Managed entry fields:

- `name`: stable logical app/plugin name.
- `kind`: `app` for `internal/app/*`, `plugin` for `plugins/*`.
- `mode`: `source` for template-managed source.
- `path`: target path in the generated project. Preserve project-side edits to this value.
- `source_path`: source path in the template. Defaults to `path` when omitted.

Project-side escape hatches:

- `mode: manual`
- `mode: ignore`
- `mode: detached`

These modes keep the origin record but stop `fbago template diff` and
`fbago template update` from touching that app/plugin path. Use them when a
generated module has accumulated project-specific business changes.

## Template Diff / Update Rules

`fbago template diff`:

1. reads project `.fbago.yaml`
2. resolves the source template from the manifest unless `--template` overrides it
3. renders the template with the project module and recorded core version
4. compares only managed paths
5. prints `A`, `M`, or `D` file-level changes

`fbago template update` follows the same plan and then writes changes:

- `--dry-run` never writes files.
- New managed files (`A`) may be written without `--force`.
- Modified managed files (`M`) must require `--force`.
- Removed managed files (`D`) must require `--force`.
- The manifest must be written after managed files, so a partial failure does not
  record a new template state before source files are updated.
- Use `--template <local-template-path>` when testing local template checkout changes.

Deletion semantics:

- If a managed entry disappears from the new template manifest, report `D` for
  files under the old managed path.
- Do not delete files by default; require `--force` because the old path may
  contain project business changes.
- If the removed path overlaps another still-managed path, do not plan deletion
  for the overlapping path.

Safety constraint:

- There is currently no per-file baseline hash. Treat every existing file that
  differs from the freshly rendered template as potentially project-modified.
  This is why update requires explicit `--force` for `M` and `D`.

## Core Version Rules

Generated `go.mod` must use a concrete module version. Never render `@latest` into `go.mod`.

Resolution rules:

- Explicit `--core-version vX.Y.Z` wins.
- `FBAGO_CORE_VERSION` is used when the flag is absent.
- Explicit `--core-version latest` resolves through `go list`.
- When a local replace exists, use `v0.0.0` because replace makes the selected version irrelevant.
- Release builds use the binary build version when no replace is present.
- Development fallback is `v0.0.0`.

Local development should produce `v0.0.0 + replace`. Published use should produce a semver version.

## Core Replace Rules

`CoreReplace` comes from:

1. `--core-replace`
2. `FBAGO_CORE_REPLACE`
3. auto-discovered local module root for development builds

Released binaries should not force a local replace.

## Local Template Rules

Local templates are expected to be runnable repositories. They may include:

- real `go.mod` for testing the template itself
- `.fbago-template.yaml` with source module
- `go.mod.tmpl` for generated projects
- `.tmpl` files for renderable content

Skipped directories:

- `.cache`
- `.codegraph`
- `.git`
- `.hg`
- `.svn`
- `bin`
- `node_modules`
- `tmp`

Skipped files:

- `.DS_Store`
- `.fbago-template.yaml`
- `Thumbs.db`

## Remote Git Templates

Supported forms:

```bash
github.com/yuWorm/fba-go-template/admin@v0.0.1
https://github.com/yuWorm/fba-go-template.git//admin@v0.0.1
git+https://github.com/yuWorm/fba-go-template.git//admin@v0.0.1
git@github.com:yuWorm/fba-go-template.git//admin@v0.0.1
```

Document production examples with release tags, not `@master`.

## Verification

Scaffold package:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./cmd/fbago ./cmd/fbago/internal/scaffold
```

Generated admin smoke test:

```bash
tmpdir="$(mktemp -d /private/tmp/fbago-gen-admin.XXXXXX)"
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go run ./cmd/fbago init github.com/acme/admin --template templates/fba-go-template/admin --dir "$tmpdir" --core-replace "$(pwd)"
(cd "$tmpdir" && GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./...)
```
