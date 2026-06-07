package service_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/dto"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
)

func TestSeedStoragesAndScenesProvideLocalDefaults(t *testing.T) {
	storages := model.SeedStorages()
	if len(storages) == 0 {
		t.Fatal("SeedStorages() returned no storages")
	}
	if storages[0].Code != model.DefaultStorageCode || storages[0].Provider != model.ProviderLocal || !storages[0].Enabled {
		t.Fatalf("first storage = %+v, want enabled local default", storages[0])
	}

	scenes := model.SeedScenes()
	got := map[string]model.Scene{}
	for _, scene := range scenes {
		got[scene.Code] = scene
	}
	for _, code := range []string{model.DefaultSceneCode, model.SceneAvatar, model.SceneAttachment} {
		if !got[code].Enabled {
			t.Fatalf("scene %q missing or disabled: %+v", code, got[code])
		}
	}
	if got[model.SceneAvatar].MaxSize <= 0 {
		t.Fatalf("avatar max size = %d, want positive", got[model.SceneAvatar].MaxSize)
	}
}

func TestDTOsExposeFileAndRefDetailsWithoutInternalPaths(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	object := model.FileObject{
		ID:           7,
		UUID:         "file-uuid",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/avatar/2026/06/07/file.png",
		OriginalName: "avatar.png",
		Ext:          "png",
		Mime:         "image/png",
		Size:         123,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
		CreatedTime:  now,
	}
	ref := model.FileRef{
		ID:          9,
		FileID:      object.ID,
		SceneCode:   model.SceneAvatar,
		SubjectType: strPtr("user"),
		SubjectID:   strPtr("1"),
		Field:       strPtr("avatar"),
		Status:      model.RefStatusActive,
		CreatedTime: now,
	}

	detail := dto.FileDetailFromModel(object, "/download/file-uuid")
	if detail.ID != object.ID || detail.UUID != object.UUID || detail.URL != "/download/file-uuid" {
		t.Fatalf("file detail = %+v", detail)
	}
	if detail.ObjectKey != "" {
		t.Fatalf("file detail leaked object key %q", detail.ObjectKey)
	}

	refDetail := dto.RefDetailFromModel(ref, object, "/download/file-uuid")
	if refDetail.ID != ref.ID || refDetail.File.ID != object.ID || refDetail.SceneCode != model.SceneAvatar {
		t.Fatalf("ref detail = %+v", refDetail)
	}
	if refDetail.File.ObjectKey != "" {
		t.Fatalf("ref detail leaked object key %q", refDetail.File.ObjectKey)
	}
}

func strPtr(value string) *string {
	return &value
}

func TestServiceUploadsBindsSharesAndDownloadsLocalFiles(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	actor := service.Actor{UserID: intPtr(1)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "report.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("hello"),
		SceneCode:   model.DefaultSceneCode,
		Field:       "attachments",
		Actor:       actor,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if uploaded.File.ID == 0 || uploaded.File.UUID == "" || uploaded.Ref.Status != model.RefStatusTemp {
		t.Fatalf("Upload() = %+v", uploaded)
	}
	if uploaded.File.UploadedBy == nil || *uploaded.File.UploadedBy != 1 {
		t.Fatalf("uploaded_by = %v, want 1", uploaded.File.UploadedBy)
	}

	if err := svc.Bind(ctx, service.BindInput{
		FileIDs:     []int{uploaded.File.ID},
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "notice",
		SubjectID:   "1001",
		Field:       "attachments",
		Actor:       actor,
	}); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	refs, err := svc.ListRefs(ctx, repo.RefFilter{
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "notice",
		SubjectID:   "1001",
		Field:       "attachments",
		Status:      model.RefStatusActive,
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListRefs() error = %v", err)
	}
	if len(refs.Items) != 1 || refs.Items[0].File.ID != uploaded.File.ID {
		t.Fatalf("ListRefs() = %+v", refs)
	}

	share, err := svc.CreateShare(ctx, service.ShareInput{
		FileID:   uploaded.File.ID,
		Password: strPtr("secret"),
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("CreateShare() error = %v", err)
	}
	if share.Token == "" {
		t.Fatalf("share token is empty: %+v", share)
	}
	if _, err := svc.VerifySharePassword(ctx, share.Token, "wrong"); err == nil {
		t.Fatal("VerifySharePassword() with wrong password succeeded")
	}
	downloadToken, err := svc.VerifySharePassword(ctx, share.Token, "secret")
	if err != nil {
		t.Fatalf("VerifySharePassword() error = %v", err)
	}
	reader, detail, err := svc.OpenShare(ctx, share.Token, downloadToken)
	if err != nil {
		t.Fatalf("OpenShare() error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(body, []byte("hello")) || detail.ID != uploaded.File.ID {
		t.Fatalf("download body=%q detail=%+v", body, detail)
	}
	reloaded, err := repository.GetShareByToken(ctx, share.Token)
	if err != nil {
		t.Fatalf("GetShareByToken() error = %v", err)
	}
	if reloaded.DownloadCount != 1 {
		t.Fatalf("DownloadCount = %d, want 1", reloaded.DownloadCount)
	}
}

func TestServiceValidatesSceneExtensionAndSize(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "avatar.txt",
		ContentType: "text/plain",
		Size:        1,
		Reader:      strings.NewReader("x"),
		SceneCode:   model.SceneAvatar,
	}); err == nil {
		t.Fatal("Upload() accepted invalid avatar extension")
	}
	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "avatar.png",
		ContentType: "image/png",
		Size:        6 * 1024 * 1024,
		Reader:      strings.NewReader("x"),
		SceneCode:   model.SceneAvatar,
	}); err == nil {
		t.Fatal("Upload() accepted oversized avatar")
	}
}

func intPtr(value int) *int {
	return &value
}
