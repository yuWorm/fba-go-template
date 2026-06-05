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

From the repository root, run `make verify-admin` to test this template and a generated backend project end to end.
