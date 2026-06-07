package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	uploadapi "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/api"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	uploadservice "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
	"github.com/yuWorm/fba-go/core/middleware"
	"github.com/yuWorm/fba-go/core/plugin"
	"github.com/yuWorm/fba-go/core/rbac"
)

func TestUploadAPIUploadsBindsListsSharesAndDownloads(t *testing.T) {
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	root := t.TempDir()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: root}))
	svc := uploadservice.New(repository, registry, uploadservice.Options{TokenSecret: []byte("api-test-secret")})
	app := newUploadApp(t, svc)

	resp, body := requestMultipart(t, app, http.MethodPost, "/api/v1/sys/upload/files", map[string]string{
		"file": "invoice.txt",
	}, map[string]string{
		"scene_code":   "default",
		"subject_type": "order",
		"subject_id":   "SO-1",
		"field":        "invoice",
		"owner_type":   "user",
		"owner_id":     "42",
	}, []byte("invoice body"))
	assertStatusOK(t, resp)
	data := envelopeMap(t, body)
	file := assertMap(t, data["file"])
	if file["original_name"] != "invoice.txt" {
		t.Fatalf("original_name = %v, want invoice.txt", file["original_name"])
	}
	if file["uploaded_by"] != float64(42) {
		t.Fatalf("uploaded_by = %v, want 42", file["uploaded_by"])
	}
	if _, ok := file["object_key"]; ok {
		t.Fatalf("file detail leaked object_key: %v", file["object_key"])
	}
	fileID := int(file["id"].(float64))
	if info, err := os.Stat(filepath.Join(root, "uploads")); err != nil || !info.IsDir() {
		t.Fatalf("uploaded file prefix not found under local root: info=%v err=%v", info, err)
	}
	publicURL, ok := file["url"].(string)
	if !ok || publicURL == "" {
		t.Fatalf("file url = %v, want non-empty", file["url"])
	}

	resp, raw := requestRaw(t, app, http.MethodGet, "/api/v1/sys/upload/files/"+itoa(fileID)+"/download", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("private download status = %d body=%s", resp.StatusCode, string(raw))
	}
	if string(raw) != "invoice body" {
		t.Fatalf("private download body = %q, want invoice body", string(raw))
	}

	resp, raw = requestRaw(t, app, http.MethodGet, publicURL, "", "")
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("private public-url status = %d body=%s, want 403", resp.StatusCode, string(raw))
	}

	resp, body = requestJSON(t, app, http.MethodPost, "/api/v1/sys/upload/files/"+itoa(fileID)+"/access-token", `{"ttl_seconds":300}`)
	assertStatusOK(t, resp)
	access := envelopeMap(t, body)
	downloadURL, ok := access["download_url"].(string)
	if !ok || downloadURL == "" {
		t.Fatalf("download_url = %v, want non-empty", access["download_url"])
	}
	if token, ok := access["download_token"].(string); !ok || token == "" {
		t.Fatalf("download_token = %v, want non-empty", access["download_token"])
	}
	resp, raw = requestRaw(t, app, http.MethodGet, downloadURL, "", "")
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("temporary download status = %d body=%s", resp.StatusCode, string(raw))
	}
	if string(raw) != "invoice body" {
		t.Fatalf("temporary download body = %q, want invoice body", string(raw))
	}

	resp, body = requestJSON(t, app, http.MethodGet, "/api/v1/sys/upload/refs?scene_code=default&subject_type=order&subject_id=SO-1&field=invoice&owner_type=user&owner_id=42", "")
	assertStatusOK(t, resp)
	page := envelopeMap(t, body)
	items := assertSlice(t, page["items"])
	if len(items) != 1 {
		t.Fatalf("refs length = %d, want 1; body=%v", len(items), body)
	}
	ref := assertMap(t, items[0])
	if ref["status"] != "active" {
		t.Fatalf("ref status = %v, want active", ref["status"])
	}

	resp, body = requestJSON(t, app, http.MethodPost, "/api/v1/sys/upload/shares", `{"file_id":`+itoa(fileID)+`,"password":"secret","max_downloads":2}`)
	assertStatusOK(t, resp)
	share := envelopeMap(t, body)
	token, ok := share["token"].(string)
	if !ok || token == "" {
		t.Fatalf("share token = %v, want non-empty", share["token"])
	}

	resp, body = requestJSON(t, app, http.MethodPost, "/api/v1/public/upload/shares/"+token+"/verify", `{"password":"wrong"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("verify wrong password status = %d, want 403; body=%v", resp.StatusCode, body)
	}

	resp, body = requestJSON(t, app, http.MethodPost, "/api/v1/public/upload/shares/"+token+"/verify", `{"password":"secret"}`)
	assertStatusOK(t, resp)
	verify := envelopeMap(t, body)
	downloadToken, ok := verify["download_token"].(string)
	if !ok || downloadToken == "" {
		t.Fatalf("download_token = %v, want non-empty", verify["download_token"])
	}

	resp, raw = requestRaw(t, app, http.MethodGet, "/api/v1/public/upload/shares/"+token+"/download?download_token="+downloadToken, "", "")
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("download status = %d body=%s", resp.StatusCode, string(raw))
	}
	if string(raw) != "invoice body" {
		t.Fatalf("download body = %q, want invoice body", string(raw))
	}
}

func TestUploadAPIUpdatesLocalStorageConfigAndUsesIt(t *testing.T) {
	repository := repo.NewMemoryRepository(repo.SeedData())
	svc := uploadservice.New(repository, storage.NewRegistry(), uploadservice.Options{TokenSecret: []byte("api-test-secret")})
	app := newUploadApp(t, svc)
	root := t.TempDir()
	config, err := json.Marshal(map[string]string{"root": root})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"provider":   "local",
		"prefix":     "managed",
		"is_default": true,
		"enabled":    true,
		"config":     string(config),
	})
	if err != nil {
		t.Fatalf("json.Marshal(storage body) error = %v", err)
	}

	resp, decoded := requestJSON(t, app, http.MethodPut, "/api/v1/sys/upload/storages/local", string(body))
	assertStatusOK(t, resp)
	storageDetail := envelopeMap(t, decoded)
	if storageDetail["prefix"] != "managed" {
		t.Fatalf("storage prefix = %v, want managed", storageDetail["prefix"])
	}

	resp, decoded = requestMultipart(t, app, http.MethodPost, "/api/v1/sys/upload/files", map[string]string{
		"file": "managed.txt",
	}, map[string]string{
		"scene_code": "default",
	}, []byte("managed"))
	assertStatusOK(t, resp)
	file := assertMap(t, envelopeMap(t, decoded)["file"])
	object, err := repository.GetObject(context.Background(), int(file["id"].(float64)))
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(object.ObjectKey))); err != nil {
		t.Fatalf("stored object %q under managed root %q not found: %v", object.ObjectKey, root, err)
	}
}

func TestUploadAPICreatesSceneAndUsesItForUpload(t *testing.T) {
	repository := repo.NewMemoryRepository(repo.SeedData())
	root := t.TempDir()
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: root}))
	svc := uploadservice.New(repository, registry, uploadservice.Options{TokenSecret: []byte("api-test-secret")})
	app := newUploadApp(t, svc)

	resp, body := requestJSON(t, app, http.MethodPost, "/api/v1/sys/upload/scenes", `{"code":"contract","name":"Contract","max_size":1024,"allowed_exts":"[\"txt\"]","default_storage_code":"local","default_visibility":"private","temp_ttl_seconds":120,"enabled":true}`)
	assertStatusOK(t, resp)
	scene := envelopeMap(t, body)
	if scene["code"] != "contract" || scene["max_size"] != float64(1024) {
		t.Fatalf("created scene = %v, want contract max_size 1024", scene)
	}

	resp, body = requestJSON(t, app, http.MethodGet, "/api/v1/sys/upload/scenes", "")
	assertStatusOK(t, resp)
	scenes := assertSlice(t, body["data"])
	found := false
	for _, raw := range scenes {
		item := assertMap(t, raw)
		if item["code"] == "contract" {
			found = true
		}
	}
	if !found {
		t.Fatalf("contract scene missing from list: %v", scenes)
	}

	resp, body = requestMultipart(t, app, http.MethodPost, "/api/v1/sys/upload/files", map[string]string{
		"file": "contract.txt",
	}, map[string]string{
		"scene_code": "contract",
	}, []byte("contract"))
	assertStatusOK(t, resp)
	file := assertMap(t, envelopeMap(t, body)["file"])
	object, err := repository.GetObject(context.Background(), int(file["id"].(float64)))
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if object.StorageCode != model.DefaultStorageCode || object.Ext != "txt" {
		t.Fatalf("uploaded object = %+v, want local txt", object)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(object.ObjectKey))); err != nil {
		t.Fatalf("contract file not stored under local root: %v", err)
	}
}

func TestUploadAPIUpdatesAndDeletesScene(t *testing.T) {
	repository := repo.NewMemoryRepository(repo.SeedData())
	svc := uploadservice.New(repository, storage.NewRegistry(), uploadservice.Options{TokenSecret: []byte("api-test-secret")})
	app := newUploadApp(t, svc)

	resp, body := requestJSON(t, app, http.MethodPost, "/api/v1/sys/upload/scenes", `{"code":"contracts","name":"Contracts","max_size":1024,"allowed_exts":"[\"txt\"]","default_visibility":"private","temp_ttl_seconds":120,"enabled":true}`)
	assertStatusOK(t, resp)
	scene := envelopeMap(t, body)
	if scene["code"] != "contracts" {
		t.Fatalf("created scene = %v, want contracts", scene)
	}

	resp, body = requestJSON(t, app, http.MethodPut, "/api/v1/sys/upload/scenes/contracts", `{"name":"Contract Files","max_size":2048,"allowed_exts":"[\"txt\",\"pdf\"]","enabled":false}`)
	assertStatusOK(t, resp)
	scene = envelopeMap(t, body)
	if scene["name"] != "Contract Files" || scene["max_size"] != float64(2048) || scene["enabled"] != false {
		t.Fatalf("updated scene = %v, want renamed disabled scene", scene)
	}

	resp, body = requestJSON(t, app, http.MethodDelete, "/api/v1/sys/upload/scenes/contracts", "")
	assertStatusOK(t, resp)

	resp, body = requestJSON(t, app, http.MethodGet, "/api/v1/sys/upload/scenes", "")
	assertStatusOK(t, resp)
	for _, raw := range assertSlice(t, body["data"]) {
		item := assertMap(t, raw)
		if item["code"] == "contracts" {
			t.Fatalf("deleted scene still listed: %v", body)
		}
	}
}

func TestUploadAPIScopesFilesAndSharesByOwnerForNormalUsers(t *testing.T) {
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := uploadservice.New(repository, registry, uploadservice.Options{TokenSecret: []byte("api-test-secret")})
	ownerApp := newUploadAppWithUser(t, svc, uploadUser(7, "owner"))
	otherApp := newUploadAppWithUser(t, svc, uploadUser(8, "other"))
	adminApp := newUploadAppWithUser(t, svc, &rbac.CurrentUser{ID: 1, Username: "admin", IsStaff: true, IsSuperAdmin: true})

	resp, body := requestMultipart(t, ownerApp, http.MethodPost, "/api/v1/sys/upload/files", map[string]string{
		"file": "owned.txt",
	}, map[string]string{
		"scene_code": "default",
	}, []byte("owned"))
	assertStatusOK(t, resp)
	file := assertMap(t, envelopeMap(t, body)["file"])
	fileID := int(file["id"].(float64))

	resp, body = requestJSON(t, ownerApp, http.MethodGet, "/api/v1/sys/upload/files", "")
	assertStatusOK(t, resp)
	if items := assertSlice(t, envelopeMap(t, body)["items"]); len(items) != 1 {
		t.Fatalf("owner list length = %d, want 1; body=%v", len(items), body)
	}

	resp, body = requestJSON(t, otherApp, http.MethodGet, "/api/v1/sys/upload/files", "")
	assertStatusOK(t, resp)
	if items := assertSlice(t, envelopeMap(t, body)["items"]); len(items) != 0 {
		t.Fatalf("other list length = %d, want 0; body=%v", len(items), body)
	}

	resp, body = requestJSON(t, otherApp, http.MethodPost, "/api/v1/sys/upload/shares", `{"file_id":`+itoa(fileID)+`}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("foreign share status = %d, want 403; body=%v", resp.StatusCode, body)
	}

	resp, body = requestJSON(t, adminApp, http.MethodGet, "/api/v1/sys/upload/files", "")
	assertStatusOK(t, resp)
	if items := assertSlice(t, envelopeMap(t, body)["items"]); len(items) != 1 {
		t.Fatalf("admin list length = %d, want 1; body=%v", len(items), body)
	}
}

func newUploadApp(t *testing.T, svc *uploadservice.Service) *fiber.App {
	t.Helper()
	return newUploadAppWithUser(t, svc, &rbac.CurrentUser{ID: 42, Username: "api-user", IsStaff: true, IsSuperAdmin: true})
}

func newUploadAppWithUser(t *testing.T, svc *uploadservice.Service, user *rbac.CurrentUser) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
	ctx := plugin.NewContext(plugin.ContextOptions{APIGroup: app.Group("/api/v1")})
	handler := uploadapi.NewHandler(svc)
	if err := plugin.RegisterRoutes(ctx, uploadapi.Routes(handler)); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}
	plugin.MountRoutes(ctx.APIGroup(), ctx.Routes(), plugin.WithAuthenticator(plugin.AuthenticatorFunc(func(fiber.Ctx) (*rbac.CurrentUser, error) {
		return user, nil
	})))
	return app
}

func uploadUser(id int64, username string) *rbac.CurrentUser {
	return &rbac.CurrentUser{
		ID:       id,
		Username: username,
		IsStaff:  true,
		Roles: []rbac.Role{
			{
				ID:        id,
				Enabled:   true,
				MenuCount: 1,
				Permissions: []string{
					"sys:upload:file:add",
					"sys:upload:share:add",
				},
			},
		},
	}
}

func requestMultipart(t *testing.T, app *fiber.App, method string, path string, files map[string]string, fields map[string]string, payload []byte) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, filename := range files {
		part, err := writer.CreateFormFile(name, filename)
		if err != nil {
			t.Fatalf("CreateFormFile() error = %v", err)
		}
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("multipart write error = %v", err)
		}
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("WriteField() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart Close() error = %v", err)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, path, err)
	}
	defer resp.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode %s %s response: %v", method, path, err)
	}
	return resp, decoded
}

func requestJSON(t *testing.T, app *fiber.App, method string, path string, body string) (*http.Response, map[string]any) {
	t.Helper()
	resp, raw := requestRaw(t, app, method, path, body, "application/json")
	defer resp.Body.Close()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode %s %s response: %v; body=%s", method, path, err, string(raw))
	}
	return resp, decoded
}

func requestRaw(t *testing.T, app *fiber.App, method string, path string, body string, contentType string) (*http.Response, []byte) {
	t.Helper()
	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, path, err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, path, err)
	}
	return resp, raw
}

func assertStatusOK(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func envelopeMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	if body["code"] != float64(200) {
		t.Fatalf("code = %v, want 200; body=%v", body["code"], body)
	}
	return assertMap(t, body["data"])
}

func assertMap(t *testing.T, value any) map[string]any {
	t.Helper()
	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v, want map", value)
	}
	return item
}

func assertSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v, want slice", value)
	}
	return items
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
