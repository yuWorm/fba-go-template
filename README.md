# FBA Go Templates

Runnable project templates for `fbago init`.

## Templates

- `admin`: admin starter backend with project-owned `admin`, `config`, `dict`, and `notice` modules under `internal/app`.

## Usage

```bash
fbago init github.com/your-org/my-backend --template /path/to/fba-go-template/admin
fbago init github.com/your-org/my-backend --template github.com/your-org/fba-go-template/admin@v0.1.0
```

## Verification

```bash
make verify
```

`make verify` checks the runnable template itself, generates a temporary backend with `fbago init`, and then runs `make tidy`, `make test`, and `make build` inside the generated project. Set `FBA_GO_ROOT=/path/to/fba-go` when the core checkout is not next to this repository.
