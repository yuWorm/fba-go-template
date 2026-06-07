package uploadfile_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	adminservice "github.com/yuWorm/fba-go-template/admin/internal/app/admin/service"
	uploadfile "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	uploadrepo "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go/core/command"
	"github.com/yuWorm/fba-go/core/config"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/di"
	"github.com/yuWorm/fba-go/core/plugin"
	coretask "github.com/yuWorm/fba-go/core/task"
	"gorm.io/driver/sqlite"
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
		"POST /sys/upload/files/presign":            {authRequired: true, permission: "sys:upload:file:add"},
		"POST /sys/upload/files/:pk/complete":       {authRequired: true, permission: "sys:upload:file:add"},
		"GET /sys/upload/files/:pk":                 {authRequired: true},
		"GET /sys/upload/files/:pk/download":        {authRequired: true},
		"POST /sys/upload/files/:pk/access-token":   {authRequired: true},
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

func TestUploadfilePluginInitialDataUsesEnvConfig(t *testing.T) {
	envFile := writeUploadfileEnvFile(t, `
UPLOADFILE_STORAGE_PROVIDER=s3
UPLOADFILE_STORAGE_BUCKET=plugin-bucket
UPLOADFILE_STORAGE_REGION=ap-southeast-1
UPLOADFILE_STORAGE_BASE_URL=https://cdn.example.test/files
UPLOADFILE_S3_FORCE_PATH_STYLE=true
UPLOADFILE_DEFAULT_MAX_SIZE=12345
UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS=600
`)
	t.Setenv("FBA_ENV_FILE", envFile)
	container := di.New()
	gormDB, err := gorm.Open(sqlite.Open("file:uploadfile_plugin_config?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	provider := db.NewGORMProvider(gormDB, nil)
	if err := container.Provide(func() db.Provider {
		return provider
	}); err != nil {
		t.Fatalf("Provide() error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	for _, migration := range ctx.Migrations() {
		if err := migration.Up(context.Background()); err != nil {
			t.Fatalf("migration %s Up() error = %v", migration.Version, err)
		}
	}

	repository := uploadrepo.NewGORMRepository(provider)
	storageConfig, err := repository.GetStorage(context.Background(), model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("GetStorage() error = %v", err)
	}
	if storageConfig.Provider != model.ProviderS3 || ptrValue(storageConfig.Bucket) != "plugin-bucket" || ptrValue(storageConfig.Region) != "ap-southeast-1" {
		t.Fatalf("storage = %+v, want env configured S3 storage", storageConfig)
	}
	if ptrValue(storageConfig.BaseURL) != "https://cdn.example.test/files" || ptrValue(storageConfig.Config) != `{"force_path_style":true}` {
		t.Fatalf("storage url/config = %v / %v", storageConfig.BaseURL, storageConfig.Config)
	}
	scene, err := repository.GetScene(context.Background(), model.DefaultSceneCode)
	if err != nil {
		t.Fatalf("GetScene() error = %v", err)
	}
	if scene.MaxSize != 12345 || scene.TempTTLSeconds != 600 {
		t.Fatalf("scene = %+v, want env configured limits", scene)
	}
}

func TestUploadfilePluginRegistersAdminUploadBackend(t *testing.T) {
	envFile := writeUploadfileEnvFile(t, "UPLOADFILE_LOCAL_ROOT="+t.TempDir()+"\n")
	t.Setenv("FBA_ENV_FILE", envFile)
	container := di.New()
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	var backend adminservice.FileUploadBackend
	if !container.Resolve(&backend) || backend == nil {
		t.Fatal("admin FileUploadBackend was not registered")
	}
	userID := 7
	uploaded, err := backend.Upload(context.Background(), adminservice.FileUploadInput{
		Filename:    "compat.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("hello"),
		UserID:      &userID,
	})
	if err != nil {
		t.Fatalf("backend Upload() error = %v", err)
	}
	if !strings.HasPrefix(uploaded.URL, "/api/v1/public/upload/files/") {
		t.Fatalf("uploaded URL = %q, want uploadfile public URL", uploaded.URL)
	}
}

func TestUploadfilePluginAppliesQuotaEnvToAdminUploadBackend(t *testing.T) {
	envFile := writeUploadfileEnvFile(t, "UPLOADFILE_LOCAL_ROOT="+t.TempDir()+"\nUPLOADFILE_MAX_TOTAL_BYTES=4\n")
	t.Setenv("FBA_ENV_FILE", envFile)
	container := di.New()
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	var backend adminservice.FileUploadBackend
	if !container.Resolve(&backend) || backend == nil {
		t.Fatal("admin FileUploadBackend was not registered")
	}
	userID := 7
	_, err := backend.Upload(context.Background(), adminservice.FileUploadInput{
		Filename:    "quota.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("quota"),
		UserID:      &userID,
	})
	if err == nil {
		t.Fatal("backend Upload() succeeded over env total byte quota")
	}
}

func TestUploadfilePluginRegistersCleanupTaskMetadata(t *testing.T) {
	ctx := plugin.NewContext(plugin.ContextOptions{})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	tasks := ctx.Tasks()
	for _, task := range tasks {
		if task.Type == "uploadfile.cleanup" {
			if task.Name != "Cleanup expired upload files" || task.Queue != "default" {
				t.Fatalf("cleanup task = %+v, want name and default queue", task)
			}
			return
		}
	}
	t.Fatalf("cleanup task metadata not registered: %+v", tasks)
}

func TestUploadfilePluginRegistersExecutableCleanupTask(t *testing.T) {
	registry := coretask.NewRegistry()
	container := di.New()
	if err := container.Provide(func() *coretask.Registry {
		return registry
	}); err != nil {
		t.Fatalf("Provide(task registry) error = %v", err)
	}
	ctx := plugin.NewContext(plugin.ContextOptions{Container: container})
	if err := uploadfile.FBAPlugin().Register(ctx); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	definitions := registry.All()
	for _, definition := range definitions {
		if definition.Type == "uploadfile.cleanup" {
			if definition.Name != "Cleanup expired upload files" || definition.Queue != "default" || definition.Handler == nil {
				t.Fatalf("cleanup definition = %+v, want executable default task", definition)
			}
			if err := definition.Handler.ProcessTask(context.Background(), asynq.NewTask(definition.Type, []byte("{}"))); err != nil {
				t.Fatalf("cleanup handler ProcessTask() error = %v", err)
			}
			return
		}
	}
	t.Fatalf("cleanup task definition not registered: %+v", definitions)
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
	if !strings.Contains(output, "expired_refs=0") || !strings.Contains(output, "pending_files=0") || !strings.Contains(output, "deleted_files=0") || !strings.Contains(output, "dry_run=false") {
		t.Fatalf("cleanup output = %q, want cleanup counters", output)
	}

	out.Reset()
	err = command.Execute(context.Background(), command.ExecuteOptions{
		Use:      "admin",
		Runtime:  testCommandRuntime{container: di.New(), out: &out},
		Commands: ctx.Commands(),
	}, []string{"uploadfile", "cleanup", "--dry-run"})
	if err != nil {
		t.Fatalf("Execute(uploadfile cleanup --dry-run) error = %v", err)
	}
	output = out.String()
	if !strings.Contains(output, "expired_refs=0") || !strings.Contains(output, "pending_files=0") || !strings.Contains(output, "deleted_files=0") || !strings.Contains(output, "dry_run=true") {
		t.Fatalf("cleanup --dry-run output = %q, want dry-run cleanup counters", output)
	}
}

func routeKeys(routes map[string]plugin.Route) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}
	return keys
}

func writeUploadfileEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
