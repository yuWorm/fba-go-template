---
name: fba-go-template-dev
description: Maintain FBA Go scaffold templates and generated project behavior. Use when changing fbago init, embedded basic templates, remote or local template loading, go.mod.tmpl version replacement, admin template files, template tests, template tags, or behavior under cmd/fbago/internal/scaffold and templates/fba-go-template.
---

# FBA Go Template Development

## Workflow

Use this skill to keep generated projects reproducible, release-safe, and aligned with the runnable admin template.

1. Identify the template surface: embedded `basic`, local admin template, remote Git template behavior, or release documentation.
2. Keep runnable template files and generated template files intentionally different only where needed. `admin/go.mod` is local-runnable; `admin/go.mod.tmpl` is generated-project output.
3. Preserve `[[ .Module ]]`, `[[ .TemplateModule ]]`, `[[ .CoreVersion ]]`, and `[[ .CoreReplace ]]` semantics.
4. For Python-aligned admin behavior, compare `sources/fastapi-best-architecture/` before changing routes, models, migrations, or seed data.
5. Verify both the scaffold package and an actual generated project.

## Load References

- Read `references/scaffold-generation.md` before changing `fbago init`, template parsing, remote Git templates, or version replacement.
- Read `references/admin-template.md` before changing the official admin template or generated project behavior.
- Read `references/python-alignment.md` before migrating or aligning Python admin behavior.

## Guardrails

- Do not write `@latest` into generated `go.mod`. Use a resolved semver or pseudo-version.
- Keep local development safe with `v0.0.0 + replace`.
- Do not copy repository metadata or local build artifacts into generated projects.
- Do not remove `.fbago-template.yaml`; it is required to rewrite runnable template imports.
- Keep remote template examples pinned to release tags when documenting production usage.

## Verification

Run targeted scaffold tests and template tests. For admin template changes, also generate a project into a temporary directory and run its tests:

```bash
GOWORK=off GOCACHE=/private/tmp/fba-go-gocache go test ./cmd/fbago ./cmd/fbago/internal/scaffold
make -C templates/fba-go-template/admin L=1 test
```
