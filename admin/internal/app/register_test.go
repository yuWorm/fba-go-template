package app_test

import (
	"context"
	"testing"

	templateapp "github.com/yuWorm/fba-go-template/admin/internal/app"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/di"
	"github.com/yuWorm/fba-go/core/plugin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegisterIncludesDefaultBusinessModules(t *testing.T) {
	registry := plugin.NewRegistry()

	if err := templateapp.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	entries, err := registry.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	got := make(map[string]bool, len(entries))
	for _, entry := range entries {
		got[entry.Module.Meta().ID] = true
	}
	for _, id := range []string{"admin", "config", "dict", "email", "notice", "oauth2", "task"} {
		if !got[id] {
			t.Fatalf("module %q was not registered; got %v", id, got)
		}
	}
}

func TestDefaultModuleMigrationsSeedSQLInitialData(t *testing.T) {
	provider := newSQLiteProvider(t)
	container := di.New()
	if err := container.Provide(func() db.Provider {
		return provider
	}); err != nil {
		t.Fatalf("Provide(db.Provider) error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})
	registry := plugin.NewRegistry()
	if err := templateapp.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.RegisterAll(ctx); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}

	migrations := ctx.Migrations()
	if len(migrations) != 14 {
		t.Fatalf("migration count = %d, want 14", len(migrations))
	}
	for _, migration := range migrations {
		if err := migration.Up(context.Background()); err != nil {
			t.Fatalf("migration %s/%s error = %v", migration.Scope, migration.Version, err)
		}
	}

	assertTableCount(t, provider, "sys_user", "username = 'admin'", 1)
	assertTableCount(t, provider, "sys_menu", "name in ('System', 'PluginConfig', 'PluginDict', 'PluginNotice', 'AddScheduler')", 5)
	assertTableCount(t, provider, "sys_config", "key = 'LOGIN_CAPTCHA_ENABLED'", 1)
	assertTableCount(t, provider, "sys_dict_type", "code = 'sys_status'", 1)
	assertTableCount(t, provider, "sys_dict_data", "type_code = 'task_period_type'", 5)
	assertTableCount(t, provider, "sys_notice", "title = 'hahahahahaahahaha'", 1)
	assertTableCount(t, provider, "sys_user_social", "1 = 1", 0)
	assertTableCount(t, provider, "task_scheduler", "name = 'Fixture'", 1)
}

func newSQLiteProvider(t *testing.T) db.Provider {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open("file:init_migrations?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db.NewGORMProvider(gormDB, nil)
}

func assertTableCount(t *testing.T, provider db.Provider, table string, where string, want int64) {
	t.Helper()
	var got int64
	if err := provider.Read().Raw("select count(*) from " + table + " where " + where).Scan(&got).Error; err != nil {
		t.Fatalf("count %s where %s error = %v", table, where, err)
	}
	if got != want {
		t.Fatalf("count %s where %s = %d, want %d", table, where, got, want)
	}
}
