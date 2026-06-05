package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	fba "github.com/yuWorm/fba-go"
	adminruntime "github.com/yuWorm/fba-go-template/admin/internal/runtime"
	"github.com/yuWorm/fba-go/core/config"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/di"
	"github.com/yuWorm/fba-go/core/migration"
	"github.com/yuWorm/fba-go/core/plugin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeDefaultsToServerCommand(t *testing.T) {
	app := newFakeApplication()
	ctx := plugin.NewContext(plugin.ContextOptions{Container: app.Container(), Config: fba.Options{}})
	runtime, err := adminruntime.NewWithOptions(adminruntime.Options{
		Config:        fba.Options{},
		Application:   app,
		PluginContext: ctx,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if err := runtime.Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if app.runCalls != 1 {
		t.Fatalf("Run() calls = %d, want 1", app.runCalls)
	}
}

func TestRuntimeMigrateUpRunsRegisteredMigrationsOnce(t *testing.T) {
	app := newFakeApplication()
	provider := newSQLiteProvider(t)
	if err := app.Container().Provide(func() db.Provider { return provider }); err != nil {
		t.Fatalf("Provide(db.Provider) error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: app.Container(), Config: fba.Options{}})
	var calls int
	if err := ctx.Migration(migration.Migration{
		Scope:   "plugin:test",
		Version: "0001",
		Name:    "fixture",
		Up: func(context.Context) error {
			calls++
			return nil
		},
	}); err != nil {
		t.Fatalf("Migration() error = %v", err)
	}
	runtime, err := adminruntime.NewWithOptions(adminruntime.Options{
		Config:        fba.Options{},
		Application:   app,
		PluginContext: ctx,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if err := runtime.Execute(context.Background(), []string{"migrate", "up"}); err != nil {
		t.Fatalf("Execute(migrate up) error = %v", err)
	}
	if err := runtime.Execute(context.Background(), []string{"migrate", "up"}); err != nil {
		t.Fatalf("Execute(migrate up retry) error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("migration calls = %d, want 1", calls)
	}
}

func TestRuntimeMigrateStatusReportsRegisteredMigration(t *testing.T) {
	app := newFakeApplication()
	provider := newSQLiteProvider(t)
	if err := app.Container().Provide(func() db.Provider { return provider }); err != nil {
		t.Fatalf("Provide(db.Provider) error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: app.Container(), Config: fba.Options{}})
	if err := ctx.Migration(migration.Migration{Scope: "plugin:test", Version: "0001", Name: "fixture"}); err != nil {
		t.Fatalf("Migration() error = %v", err)
	}
	var out bytes.Buffer
	runtime, err := adminruntime.NewWithOptions(adminruntime.Options{
		Config:        fba.Options{},
		Application:   app,
		PluginContext: ctx,
		Out:           &out,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if err := runtime.Execute(context.Background(), []string{"migrate", "status"}); err != nil {
		t.Fatalf("Execute(migrate status) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "[ ] plugin:test 0001 fixture") {
		t.Fatalf("status output = %q, want pending migration", got)
	}
}

func TestRuntimeProvidesDatabaseBeforeRegisteringModules(t *testing.T) {
	app := newFakeApplication()
	provider := newSQLiteProvider(t)
	module := &dbAwareModule{}

	_, err := adminruntime.NewWithOptions(adminruntime.Options{
		Config: fba.Options{
			Database: config.DatabaseOptions{Driver: "sqlite", WriteDSN: "file:bootstrap?mode=memory&cache=shared"},
		},
		Application: app,
		Register: func(registry *plugin.Registry) error {
			return registry.Add(module, plugin.ModeAuto)
		},
		OpenDatabase: func(config.DatabaseOptions) (db.Provider, error) {
			return provider, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if !module.sawDBProvider {
		t.Fatal("module did not see db.Provider during registration")
	}
}

type fakeApplication struct {
	container *di.Container
	http      *fiber.App
	runCalls  int
}

type dbAwareModule struct {
	sawDBProvider bool
}

func (m *dbAwareModule) Meta() plugin.Meta {
	return plugin.Meta{ID: "db-aware", Version: "0.1.0"}
}

func (m *dbAwareModule) Register(ctx plugin.Context) error {
	var provider db.Provider
	m.sawDBProvider = ctx.Container().Resolve(&provider) && provider != nil && provider.Write() != nil
	if m.sawDBProvider {
		return ctx.Migration(migration.Migration{Scope: "plugin:db-aware", Version: "0001"})
	}
	return nil
}

func newFakeApplication() *fakeApplication {
	return &fakeApplication{
		container: di.New(),
		http:      fiber.New(),
	}
}

func (a *fakeApplication) HTTP() *fiber.App {
	return a.http
}

func (a *fakeApplication) Container() *di.Container {
	return a.container
}

func (a *fakeApplication) Run(context.Context) error {
	a.runCalls++
	return nil
}

func (a *fakeApplication) RunHTTP(ctx context.Context) error {
	return a.Run(ctx)
}

func (a *fakeApplication) Shutdown(context.Context) error {
	return nil
}

func newSQLiteProvider(t *testing.T) db.Provider {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db.NewGORMProvider(database, nil)
}
