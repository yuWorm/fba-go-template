# FBA Admin Template

This is the runnable admin starter template for `fbago init`.

Included project-owned app modules: `admin`, `config`, `dict`, and `notice`.
Included reference plugins: `email`, `oauth2`, and `task`.

```bash
go test ./...
go run ./cmd/api          # defaults to server
go run ./cmd/api server
go run ./cmd/api migrate up
go run ./cmd/api migrate status
```

Generate a project from this template:

```bash
fbago init github.com/your-org/my-backend --template templates/fba-go-template/admin
fbago init github.com/your-org/my-backend --template github.com/yuWorm/fba-go-template/admin@v0.0.1
```

Generated projects include `.fbago.yaml` so template-managed app/plugin source can be compared or refreshed later:

```bash
fbago template diff --template templates/fba-go-template/admin
fbago template update --dry-run --template templates/fba-go-template/admin
```

Set a managed entry to `mode: manual` in `.fbago.yaml` when that app/plugin has
project-specific changes and should no longer be touched by template updates.

From the repository root, run `make verify-admin` to test this template and a generated backend project end to end.
