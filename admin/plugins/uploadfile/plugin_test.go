package uploadfile_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	uploadfile "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile"
	"github.com/yuWorm/fba-go/core/command"
	"github.com/yuWorm/fba-go/core/config"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/di"
	"github.com/yuWorm/fba-go/core/plugin"
	"gorm.io/gorm"
)

func TestUploadfilePluginRegistersRoutes(t *testing.T) {
	ctx := plugin.NewContext(plugin.ContextOptions{})

	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got := make(map[string]plugin.Route)
	for _, route := range ctx.Routes() {
		got[route.Method+" "+route.Path] = route
	}
	want := map[string]struct {
		authRequired bool
		permission   string
	}{
		"POST /sys/upload/files":                    {authRequired: true, permission: "sys:upload:file:add"},
		"GET /sys/upload/files/:pk":                 {authRequired: true},
		"GET /sys/upload/files":                     {authRequired: true},
		"DELETE /sys/upload/files":                  {authRequired: true, permission: "sys:upload:file:del"},
		"POST /sys/upload/refs/bind":                {authRequired: true, permission: "sys:upload:ref:bind"},
		"GET /sys/upload/refs":                      {authRequired: true},
		"GET /sys/upload/scenes":                    {authRequired: true},
		"POST /sys/upload/scenes":                   {authRequired: true, permission: "sys:upload:scene:add"},
		"PUT /sys/upload/scenes/:code":              {authRequired: true, permission: "sys:upload:scene:edit"},
		"DELETE /sys/upload/scenes/:code":           {authRequired: true, permission: "sys:upload:scene:del"},
		"GET /sys/upload/storages":                  {authRequired: true},
		"POST /sys/upload/storages":                 {authRequired: true, permission: "sys:upload:storage:add"},
		"PUT /sys/upload/storages/:code":            {authRequired: true, permission: "sys:upload:storage:edit"},
		"DELETE /sys/upload/storages/:code":         {authRequired: true, permission: "sys:upload:storage:del"},
		"POST /sys/upload/shares":                   {authRequired: true, permission: "sys:upload:share:add"},
		"GET /sys/upload/shares":                    {authRequired: true},
		"DELETE /sys/upload/shares/:pk":             {authRequired: true, permission: "sys:upload:share:del"},
		"GET /public/upload/files/:uuid":            {},
		"GET /public/upload/shares/:token":          {},
		"POST /public/upload/shares/:token/verify":  {},
		"GET /public/upload/shares/:token/download": {},
	}
	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d; routes=%v", len(got), len(want), routeKeys(got))
	}
	for key, expected := range want {
		route, ok := got[key]
		if !ok {
			t.Fatalf("route %s not registered; routes=%v", key, routeKeys(got))
		}
		if route.AuthRequired != expected.authRequired {
			t.Fatalf("%s AuthRequired = %v, want %v", key, route.AuthRequired, expected.authRequired)
		}
		if route.Permission != expected.permission {
			t.Fatalf("%s Permission = %q, want %q", key, route.Permission, expected.permission)
		}
	}
}

func TestUploadfilePluginRegistersMigrationsWhenDBProviderExists(t *testing.T) {
	container := di.New()
	if err := container.Provide(func() db.Provider {
		return db.NewGORMProvider(&gorm.DB{}, nil)
	}); err != nil {
		t.Fatalf("Provide() error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})

	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	migrations := ctx.Migrations()
	if len(migrations) != 2 {
		t.Fatalf("migrations = %d, want 2", len(migrations))
	}
	if migrations[0].Scope != "plugin:uploadfile" || migrations[0].Version != "0001" {
		t.Fatalf("auto migration = %s/%s, want plugin:uploadfile/0001", migrations[0].Scope, migrations[0].Version)
	}
	if migrations[1].Scope != "plugin:uploadfile" || migrations[1].Version != "0002" {
		t.Fatalf("seed migration = %s/%s, want plugin:uploadfile/0002", migrations[1].Scope, migrations[1].Version)
	}
}

func TestUploadfilePluginRegistersCleanupCommand(t *testing.T) {
	ctx := plugin.NewContext(plugin.ContextOptions{})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var out bytes.Buffer
	err := command.Execute(context.Background(), command.ExecuteOptions{
		Use:      "admin",
		Runtime:  testCommandRuntime{container: di.New(), out: &out},
		Commands: ctx.Commands(),
	}, []string{"uploadfile", "cleanup"})
	if err != nil {
		t.Fatalf("Execute(uploadfile cleanup) error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "expired_refs=0") || !strings.Contains(output, "deleted_files=0") {
		t.Fatalf("cleanup output = %q, want cleanup counters", output)
	}
}

func routeKeys(routes map[string]plugin.Route) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}
	return keys
}

type testCommandRuntime struct {
	container *di.Container
	out       io.Writer
}

func (r testCommandRuntime) Container() *di.Container {
	return r.container
}

func (testCommandRuntime) Config() config.Options {
	return config.Options{}
}

func (r testCommandRuntime) Output() io.Writer {
	if r.out == nil {
		return io.Discard
	}
	return r.out
}

func (testCommandRuntime) ErrorOutput() io.Writer {
	return io.Discard
}
