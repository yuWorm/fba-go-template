# FBA Admin Template

This is the runnable admin starter template for `fbago init`.

```bash
go test ./...
go run ./cmd/api
```

Generate a project from this template:

```bash
fbago init github.com/your-org/my-backend --template templates/fba-go-template/admin
fbago init github.com/your-org/my-backend --template github.com/yuWorm/fba-go-template/admin@master
```

From the repository root, run `make verify-admin` to test this template and a generated backend project end to end.
