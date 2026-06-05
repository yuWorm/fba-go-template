# Plugin Contract

## Source Map

- Module contract: `core/plugin/module.go`
- Runtime context: `core/plugin/context.go`
- Registry and dependency resolution: `core/plugin/registry.go`
- Plugin modes: `core/plugin/mode.go`
- Generated registration: `cmd/fbago/internal/plugin/generate.go`
- Official registration example: `templates/fba-go-template/admin/internal/app/register.go`
- Module examples: `templates/fba-go-template/admin/internal/app/admin/module.go`, `templates/fba-go-template/admin/plugins/task/module.go`

## Required Shape

Every plugin or app module exposes:

```go
func FBAPlugin() plugin.Module {
	return Module{}
}

type Module struct{}

func (Module) Meta() plugin.Meta {
	return plugin.Meta{ID: "task", Name: "Task Plugin", Version: "0.1.0"}
}

func (Module) Register(ctx plugin.Context) error {
	return nil
}
```

`FBAPlugin` is the scanner and generator entry point. Keep it simple and deterministic.

## Meta Fields

Use `plugin.Meta` to describe:

- `ID`: stable unique plugin ID.
- `Name`: human-readable name.
- `Version`: plugin version.
- `Description`: short purpose.
- `Author`: author or vendor.
- `Tags`: searchable capabilities.
- `DependsOn`: plugin dependencies.
- `Provides`: capabilities exported for discovery.
- `AutoInjectDefault`: whether templates should auto-inject by default.
- `PureDependencyDefault`: whether the plugin is normally only a dependency.

Dependencies use:

```go
plugin.Dependency{ID: "admin", Version: ">=0.1.0", Optional: true}
```

Current registry validates ID presence, duplicate IDs, dependency presence, disabled dependencies, and cycles. It does not currently enforce semver ranges, so version is metadata until enforcement is added.

## Registration Rules

During `Register`, declare capabilities through the context:

- `ctx.Provide`: register constructors into DI.
- `ctx.Route`: register route declarations.
- `ctx.Migration`: register migrations.
- `ctx.Command`: register CLI commands.
- `ctx.Task`: register task declarations.
- `ctx.Swagger`: register OpenAPI fragments.

Do not mount routes directly unless the plugin is explicitly lower-level infrastructure. Normal application routes should be declared and mounted by runtime.

## Repository Selection

Use the admin pattern:

1. Start with memory repository seed data when possible.
2. Resolve an injected repository override if the module supports tests or custom wiring.
3. Resolve `db.Provider`.
4. If a write DB exists, switch to GORM repository and register migrations.

This keeps generated projects runnable without a database while still enabling production persistence.

## Registration Order

The runtime calls `registry.RegisterAll`. The registry resolves dependencies first and only registers `ModeAuto` entries.

Do not assume source import order. If a plugin needs another plugin's `ctx.Provide` result, declare the dependency in `Meta().DependsOn`.

## Generated Registration

`fbago plugin scan` can merge plugins from:

- manifest
- local `plugin.yaml`
- blank imports

`GenerateRegistration` imports each plugin module and writes `RegisterPlugins(reg *plugin.Registry) error`.

Generated registration calls:

```go
reg.Add(plugin0.FBAPlugin(), plugin.ModeAuto)
```

Keep plugin module paths importable and side-effect free.
