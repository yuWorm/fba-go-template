package config_test

import (
	"os"
	"path/filepath"
	"testing"

	uploadconfig "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/config"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
)

func TestLoadReadsDotenvAndAppliesLocalSeed(t *testing.T) {
	envFile := writeEnvFile(t, `
UPLOADFILE_STORAGE_PROVIDER=local
UPLOADFILE_STORAGE_PREFIX=assets
UPLOADFILE_STORAGE_BASE_URL=https://cdn.example.test/files
UPLOADFILE_LOCAL_ROOT=/var/lib/fba/uploads
UPLOADFILE_DEFAULT_MAX_SIZE=12345
UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS=600
UPLOADFILE_DEFAULT_ALLOWED_EXTS=txt,pdf
UPLOADFILE_DEFAULT_ALLOWED_MIMES=["text/plain","application/pdf"]
UPLOADFILE_DOWNLOAD_TOKEN_TTL_SECONDS=120
UPLOADFILE_FILE_ACCESS_TOKEN_MAX_TTL_SECONDS=3600
UPLOADFILE_DIRECT_UPLOAD_PRESIGN_TTL_SECONDS=300
UPLOADFILE_PENDING_UPLOAD_TTL_SECONDS=900
UPLOADFILE_MAX_TOTAL_BYTES=5555
UPLOADFILE_MAX_OWNER_BYTES=4444
UPLOADFILE_MAX_TOTAL_FILES=33
UPLOADFILE_MAX_OWNER_FILES=22
`)

	opts, err := uploadconfig.Load(uploadconfig.LoadOptions{EnvFile: envFile})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	seed, err := uploadconfig.ApplyToSeed(repo.SeedData(), opts)
	if err != nil {
		t.Fatalf("ApplyToSeed() error = %v", err)
	}

	storage := seed.Storages[0]
	if storage.Provider != model.ProviderLocal || storage.Prefix != "assets" || ptrValue(storage.BaseURL) != "https://cdn.example.test/files" {
		t.Fatalf("storage = %+v, want local assets with base url", storage)
	}
	if storage.Config == nil || *storage.Config != `{"root":"/var/lib/fba/uploads","base_url":"https://cdn.example.test/files"}` {
		t.Fatalf("storage config = %v, want local root/base_url JSON", storage.Config)
	}
	scene := findScene(t, seed, model.DefaultSceneCode)
	if scene.MaxSize != 12345 || scene.TempTTLSeconds != 600 {
		t.Fatalf("default scene limits = size %d ttl %d", scene.MaxSize, scene.TempTTLSeconds)
	}
	if ptrValue(scene.AllowedExts) != `["txt","pdf"]` || ptrValue(scene.AllowedMimes) != `["text/plain","application/pdf"]` {
		t.Fatalf("default scene allow lists = %v / %v", scene.AllowedExts, scene.AllowedMimes)
	}
	if opts.DownloadTokenTTLSeconds != 120 || opts.FileAccessTokenMaxTTLSeconds != 3600 || opts.DirectUploadPresignTTLSeconds != 300 || opts.PendingUploadTTLSeconds != 900 {
		t.Fatalf("service ttl options = %+v, want configured lifecycle seconds", opts)
	}
	if opts.MaxTotalBytes != 5555 || opts.MaxOwnerBytes != 4444 || opts.MaxTotalFiles != 33 || opts.MaxOwnerFiles != 22 {
		t.Fatalf("quota options = %+v, want configured quota limits", opts)
	}
}

func TestLoadProcessEnvOverridesDotenvAndAppliesS3Seed(t *testing.T) {
	envFile := writeEnvFile(t, `
UPLOADFILE_STORAGE_PROVIDER=local
UPLOADFILE_STORAGE_BUCKET=dotenv-bucket
UPLOADFILE_STORAGE_REGION=us-east-1
`)
	t.Setenv("UPLOADFILE_STORAGE_PROVIDER", "s3")
	t.Setenv("UPLOADFILE_STORAGE_BUCKET", "env-bucket")
	t.Setenv("UPLOADFILE_STORAGE_REGION", "ap-southeast-1")
	t.Setenv("UPLOADFILE_STORAGE_ENDPOINT", "https://s3.example.test")
	t.Setenv("UPLOADFILE_S3_FORCE_PATH_STYLE", "true")

	opts, err := uploadconfig.Load(uploadconfig.LoadOptions{EnvFile: envFile})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	seed, err := uploadconfig.ApplyToSeed(repo.SeedData(), opts)
	if err != nil {
		t.Fatalf("ApplyToSeed() error = %v", err)
	}

	storage := seed.Storages[0]
	if storage.Provider != model.ProviderS3 || ptrValue(storage.Bucket) != "env-bucket" || ptrValue(storage.Region) != "ap-southeast-1" {
		t.Fatalf("storage = %+v, want env S3 values", storage)
	}
	if ptrValue(storage.Endpoint) != "https://s3.example.test" || ptrValue(storage.Config) != `{"force_path_style":true}` {
		t.Fatalf("endpoint/config = %v / %v", storage.Endpoint, storage.Config)
	}
}

func TestLoadAppliesOSSSeed(t *testing.T) {
	envFile := writeEnvFile(t, `
UPLOADFILE_STORAGE_PROVIDER=oss
UPLOADFILE_STORAGE_BUCKET=oss-bucket
UPLOADFILE_STORAGE_REGION=cn-hangzhou
UPLOADFILE_STORAGE_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
UPLOADFILE_OSS_USE_PATH_STYLE=true
UPLOADFILE_OSS_USE_CNAME=true
`)

	opts, err := uploadconfig.Load(uploadconfig.LoadOptions{EnvFile: envFile})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	seed, err := uploadconfig.ApplyToSeed(repo.SeedData(), opts)
	if err != nil {
		t.Fatalf("ApplyToSeed() error = %v", err)
	}

	storage := seed.Storages[0]
	if storage.Provider != model.ProviderOSS || ptrValue(storage.Bucket) != "oss-bucket" || ptrValue(storage.Region) != "cn-hangzhou" {
		t.Fatalf("storage = %+v, want OSS values", storage)
	}
	if ptrValue(storage.Config) != `{"use_path_style":true,"use_cname":true}` {
		t.Fatalf("storage config = %v, want OSS JSON", storage.Config)
	}
}

func TestLoadRejectsInvalidProvider(t *testing.T) {
	envFile := writeEnvFile(t, "UPLOADFILE_STORAGE_PROVIDER=ftp\n")
	opts, err := uploadconfig.Load(uploadconfig.LoadOptions{EnvFile: envFile})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := uploadconfig.ApplyToSeed(repo.SeedData(), opts); err == nil {
		t.Fatal("ApplyToSeed() accepted invalid provider")
	}
}

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func findScene(t *testing.T, seed repo.Seed, code string) model.Scene {
	t.Helper()
	for _, scene := range seed.Scenes {
		if scene.Code == code {
			return scene
		}
	}
	t.Fatalf("scene %q not found in %+v", code, seed.Scenes)
	return model.Scene{}
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
