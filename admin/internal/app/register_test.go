package app_test

import (
	"testing"

	templateapp "github.com/yuWorm/fba-go-template/admin/internal/app"
	"github.com/yuWorm/fba-go/core/plugin"
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
