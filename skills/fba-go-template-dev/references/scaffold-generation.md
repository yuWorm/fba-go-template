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
7. renders `.tmpl` files
8. formats generated Go files
9. writes files

## Template Data

Template files can use:

- `[[ .Module ]]`: target module name.
- `[[ .TemplateModule ]]`: source runnable template module.
- `[[ .CoreReplace ]]`: optional replace path for local core module.
- `[[ .CoreVersion ]]`: resolved FBA Go core module version.

Keep delimiters as `[[` and `]]` because Go module files and other template content may contain normal braces.

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
