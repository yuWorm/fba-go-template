package service_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	for _, code := range []string{model.DefaultSceneCode, model.SceneAttachment} {
		exts := ptrValue(got[code].AllowedExts)
		if !strings.Contains(exts, `"avi"`) || !strings.Contains(exts, `"flv"`) {
			t.Fatalf("scene %q allowed_exts = %s, want legacy video extensions", code, exts)
		}
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
	}, 1, 20, actor)
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

func TestServiceDefaultsUploadOwnerToCurrentUser(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "owned.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("owned"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	ref, err := repository.GetRef(ctx, uploaded.Ref.ID)
	if err != nil {
		t.Fatalf("GetRef() error = %v", err)
	}
	if ptrValue(ref.OwnerType) != "user" || ptrValue(ref.OwnerID) != "7" {
		t.Fatalf("ref owner = %v/%v, want user/7", ref.OwnerType, ref.OwnerID)
	}
}

func TestServiceRejectsNormalUserAccessToForeignOwnedFile(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "private.txt",
		ContentType: "text/plain",
		Size:        7,
		Reader:      strings.NewReader("private"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload(owner) error = %v", err)
	}
	if err := svc.DeleteFiles(ctx, []int{uploaded.File.ID}, other); err == nil {
		t.Fatal("DeleteFiles() by foreign owner succeeded")
	}
	if _, err := svc.CreateShare(ctx, service.ShareInput{FileID: uploaded.File.ID, Actor: other}); err == nil {
		t.Fatal("CreateShare() by foreign owner succeeded")
	}
}

func TestServiceListFilesWithForeignOwnerFilterReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	otherOwnerType := "user"
	otherOwnerID := "8"

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "owned.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("owned"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	ownFiles, err := svc.ListFiles(ctx, repo.ObjectFilter{}, 1, 20, owner)
	if err != nil {
		t.Fatalf("ListFiles(owner) error = %v", err)
	}
	if len(ownFiles.Items) != 1 || ownFiles.Items[0].ID != uploaded.File.ID {
		t.Fatalf("owner files = %+v, want uploaded file", ownFiles)
	}
	foreignFiles, err := svc.ListFiles(ctx, repo.ObjectFilter{
		OwnerType: otherOwnerType,
		OwnerID:   otherOwnerID,
	}, 1, 20, owner)
	if err != nil {
		t.Fatalf("ListFiles(foreign owner filter) error = %v", err)
	}
	if len(foreignFiles.Items) != 0 {
		t.Fatalf("foreign owner filtered files = %+v, want empty", foreignFiles)
	}

	foreignRefs, err := svc.ListRefs(ctx, repo.RefFilter{
		OwnerType: otherOwnerType,
		OwnerID:   otherOwnerID,
	}, 1, 20, owner)
	if err != nil {
		t.Fatalf("ListRefs(foreign owner filter) error = %v", err)
	}
	if len(foreignRefs.Items) != 0 {
		t.Fatalf("foreign owner filtered refs = %+v, want empty", foreignRefs)
	}
}

func TestServiceDeletedRefDoesNotAuthorizeOwnerAccess(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	ownerType := "user"
	ownerID := "7"
	object, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "deleted-ref-file",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/deleted-ref-file.txt",
		OriginalName: "deleted-ref-file.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         4,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	ref, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    object.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusDeleted,
		OwnerType: &ownerType,
		OwnerID:   &ownerID,
	})
	if err != nil {
		t.Fatalf("CreateRef() error = %v", err)
	}
	if ref.Status != model.RefStatusDeleted {
		t.Fatalf("ref status = %q, want deleted", ref.Status)
	}
	if _, err := svc.GetFile(ctx, object.ID, service.Actor{UserID: intPtr(7)}); err == nil {
		t.Fatal("GetFile() succeeded through deleted ref owner")
	}
}

func TestServiceOpensPrivateFileForOwnerOnly(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "private.txt",
		ContentType: "text/plain",
		Size:        7,
		Reader:      strings.NewReader("private"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload(owner) error = %v", err)
	}

	reader, detail, err := svc.OpenFile(ctx, uploaded.File.ID, owner)
	if err != nil {
		t.Fatalf("OpenFile(owner) error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "private" || detail.ID != uploaded.File.ID {
		t.Fatalf("OpenFile body/detail = %q/%+v", body, detail)
	}
	if _, _, err := svc.OpenFile(ctx, uploaded.File.ID, other); err == nil {
		t.Fatal("OpenFile(other) succeeded")
	}
}

func TestServiceCreatesTemporaryAccessTokenForPrivateFile(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now: func() time.Time {
			return now
		},
	})
	owner := service.Actor{UserID: intPtr(7)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "temporary.txt",
		ContentType: "text/plain",
		Size:        9,
		Reader:      strings.NewReader("temporary"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload(owner) error = %v", err)
	}
	if _, _, err := svc.OpenPublicFile(ctx, uploaded.File.UUID, ""); err == nil {
		t.Fatal("OpenPublicFile(private without token) succeeded")
	}

	token, err := svc.CreateFileAccessToken(ctx, uploaded.File.ID, service.FileAccessTokenInput{
		TTL:   5 * time.Minute,
		Actor: owner,
	})
	if err != nil {
		t.Fatalf("CreateFileAccessToken() error = %v", err)
	}
	if token.DownloadToken == "" || !strings.Contains(token.DownloadURL, "download_token=") || token.ExpiresAt != "2026-06-07 12:05:00" {
		t.Fatalf("access token = %+v", token)
	}
	reader, detail, err := svc.OpenPublicFile(ctx, uploaded.File.UUID, token.DownloadToken)
	if err != nil {
		t.Fatalf("OpenPublicFile(private with token) error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "temporary" || detail.ID != uploaded.File.ID {
		t.Fatalf("temporary access body/detail = %q/%+v", body, detail)
	}

	now = now.Add(6 * time.Minute)
	if _, _, err := svc.OpenPublicFile(ctx, uploaded.File.UUID, token.DownloadToken); err == nil {
		t.Fatal("OpenPublicFile(private expired token) succeeded")
	}
}

func TestServiceRejectsFileAccessTokenTTLAboveMax(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{
		Now:                   func() time.Time { return now },
		TokenSecret:           []byte("test-secret"),
		DownloadTokenTTL:      time.Hour,
		FileAccessTokenMaxTTL: 30 * time.Minute,
	})
	owner := service.Actor{UserID: intPtr(7)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "private.txt",
		ContentType: "text/plain",
		Size:        7,
		Reader:      strings.NewReader("private"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if _, err := svc.CreateFileAccessToken(ctx, uploaded.File.ID, service.FileAccessTokenInput{
		TTL:   time.Hour,
		Actor: owner,
	}); err == nil {
		t.Fatal("CreateFileAccessToken() accepted ttl above max")
	}
	token, err := svc.CreateFileAccessToken(ctx, uploaded.File.ID, service.FileAccessTokenInput{
		TTL:   30 * time.Minute,
		Actor: owner,
	})
	if err != nil {
		t.Fatalf("CreateFileAccessToken() at max error = %v", err)
	}
	if token.DownloadToken == "" {
		t.Fatalf("token is empty: %+v", token)
	}
	defaultToken, err := svc.CreateFileAccessToken(ctx, uploaded.File.ID, service.FileAccessTokenInput{Actor: owner})
	if err != nil {
		t.Fatalf("CreateFileAccessToken() default ttl error = %v", err)
	}
	if defaultToken.ExpiresAt != "2026-06-07 12:30:00" {
		t.Fatalf("default token expires_at = %q, want capped max ttl", defaultToken.ExpiresAt)
	}
}

func TestServiceScopesShareListAndDisableToActor(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "share.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("share"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	share, err := svc.CreateShare(ctx, service.ShareInput{FileID: uploaded.File.ID, Actor: owner})
	if err != nil {
		t.Fatalf("CreateShare() error = %v", err)
	}
	foreign, err := svc.ListShares(ctx, repo.ShareFilter{}, 1, 20, other)
	if err != nil {
		t.Fatalf("ListShares(other) error = %v", err)
	}
	if len(foreign.Items) != 0 {
		t.Fatalf("foreign shares = %+v, want empty", foreign)
	}
	if err := svc.DisableShare(ctx, share.ID, other); err == nil {
		t.Fatal("DisableShare() by foreign user succeeded")
	}
}

func TestServiceCreatesAndCompletesPresignedUpload(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &fakePresignBackend{headInfo: storage.ObjectInfo{Size: 6, ContentType: "text/plain"}}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:            []byte("test-secret"),
		DirectUploadPresignTTL: 10 * time.Minute,
	})
	actor := service.Actor{UserID: intPtr(7)}

	result, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "direct.txt",
		ContentType: "text/plain",
		Size:        6,
		SceneCode:   model.DefaultSceneCode,
		Actor:       actor,
	})
	if err != nil {
		t.Fatalf("CreatePresignedUpload() error = %v", err)
	}
	if result.Presigned.Method != http.MethodPut || result.Presigned.URL == "" || result.Presigned.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("presigned = %+v", result.Presigned)
	}
	if result.File.Status != model.StatusPending || result.Ref.Status != model.RefStatusTemp {
		t.Fatalf("result file/ref status = %q/%q, want pending/temp", result.File.Status, result.Ref.Status)
	}
	object, err := repository.GetObject(ctx, result.File.ID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if object.Status != model.StatusPending || object.UploadedBy == nil || *object.UploadedBy != 7 {
		t.Fatalf("object = %+v, want pending uploaded_by 7", object)
	}

	completed, err := svc.CompletePresignedUpload(ctx, result.File.ID, actor)
	if err != nil {
		t.Fatalf("CompletePresignedUpload() error = %v", err)
	}
	if completed.Status != model.StatusActive {
		t.Fatalf("completed status = %q, want active", completed.Status)
	}
	if backend.headKey == "" {
		t.Fatal("CompletePresignedUpload() did not validate storage metadata")
	}
}

func TestServiceRejectsPresignedUploadCompletionWhenStorageMetadataDiffers(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &fakePresignBackend{headInfo: storage.ObjectInfo{Size: 5, ContentType: "text/plain"}}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	actor := service.Actor{UserID: intPtr(7)}

	result, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "direct.txt",
		ContentType: "text/plain",
		Size:        6,
		SceneCode:   model.DefaultSceneCode,
		Actor:       actor,
	})
	if err != nil {
		t.Fatalf("CreatePresignedUpload() error = %v", err)
	}
	if _, err := svc.CompletePresignedUpload(ctx, result.File.ID, actor); err == nil {
		t.Fatal("CompletePresignedUpload() accepted mismatched uploaded size")
	}
	object, err := repository.GetObject(ctx, result.File.ID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if object.Status != model.StatusPending {
		t.Fatalf("object status = %q, want pending after failed completion", object.Status)
	}
}

func TestServiceRejectsForeignPresignedUploadCompletion(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, &fakePresignBackend{})
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	owner := service.Actor{UserID: intPtr(7)}
	other := service.Actor{UserID: intPtr(8)}

	result, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "direct.txt",
		ContentType: "text/plain",
		Size:        6,
		SceneCode:   model.DefaultSceneCode,
		Actor:       owner,
	})
	if err != nil {
		t.Fatalf("CreatePresignedUpload() error = %v", err)
	}
	if _, err := svc.CompletePresignedUpload(ctx, result.File.ID, other); err == nil {
		t.Fatal("CompletePresignedUpload() by foreign owner succeeded")
	}
}

func TestServiceResolvesLocalBackendFromStorageConfig(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageConfigJSON := `{"root":` + quoteJSON(root) + `,"base_url":"https://cdn.example.test/files"}`
	archive := "archive"
	repository := repo.NewMemoryRepository(repo.Seed{
		Storages: []model.Storage{
			{
				ID:        1,
				Code:      archive,
				Provider:  model.ProviderLocal,
				Prefix:    "custom",
				BaseURL:   strPtr("https://cdn.example.test/files"),
				IsDefault: true,
				Enabled:   true,
				Config:    &storageConfigJSON,
			},
		},
		Scenes: []model.Scene{
			{
				ID:                 1,
				Code:               "archive",
				Name:               "Archive",
				MaxSize:            1024,
				AllowedExts:        strPtr(`["txt"]`),
				DefaultStorageCode: &archive,
				DefaultVisibility:  model.VisibilityPrivate,
				TempTTLSeconds:     60,
				Enabled:            true,
			},
		},
	})
	svc := service.New(repository, storage.NewRegistry(), service.Options{TokenSecret: []byte("test-secret")})

	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "archive.txt",
		ContentType: "text/plain",
		Size:        7,
		Reader:      strings.NewReader("archive"),
		SceneCode:   "archive",
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if uploaded.File.StorageCode != archive {
		t.Fatalf("storage_code = %q, want archive", uploaded.File.StorageCode)
	}
	object, err := repository.GetObject(ctx, uploaded.File.ID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(object.ObjectKey))); err != nil {
		t.Fatalf("stored object %q under root %q not found: %v", object.ObjectKey, root, err)
	}
}

func TestServiceUsesUpdatedStorageConfigAfterPreviousUpload(t *testing.T) {
	ctx := context.Background()
	root1 := t.TempDir()
	root2 := t.TempDir()
	config1 := `{"root":` + quoteJSON(root1) + `}`
	seed := repo.SeedData()
	seed.Storages[0].Config = &config1
	repository := repo.NewMemoryRepository(seed)
	svc := service.New(repository, storage.NewRegistry(), service.Options{TokenSecret: []byte("test-secret")})

	first, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "first.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("first"),
		SceneCode:   model.DefaultSceneCode,
	})
	if err != nil {
		t.Fatalf("first Upload() error = %v", err)
	}
	firstObject, err := repository.GetObject(ctx, first.File.ID)
	if err != nil {
		t.Fatalf("GetObject(first) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root1, filepath.FromSlash(firstObject.ObjectKey))); err != nil {
		t.Fatalf("first object not stored under root1: %v", err)
	}

	config2 := `{"root":` + quoteJSON(root2) + `}`
	if _, err := svc.UpdateStorage(ctx, model.DefaultStorageCode, dto.StorageParam{
		Provider:  model.ProviderLocal,
		Prefix:    "uploads",
		IsDefault: boolPtr(true),
		Enabled:   boolPtr(true),
		Config:    &config2,
	}); err != nil {
		t.Fatalf("UpdateStorage() error = %v", err)
	}
	second, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "second.txt",
		ContentType: "text/plain",
		Size:        6,
		Reader:      strings.NewReader("second"),
		SceneCode:   model.DefaultSceneCode,
	})
	if err != nil {
		t.Fatalf("second Upload() error = %v", err)
	}
	secondObject, err := repository.GetObject(ctx, second.File.ID)
	if err != nil {
		t.Fatalf("GetObject(second) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root2, filepath.FromSlash(secondObject.ObjectKey))); err != nil {
		t.Fatalf("second object not stored under updated root2: %v", err)
	}
}

func TestServicePreventsDeletingSceneWithRefs(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	if _, err := svc.CreateScene(ctx, dto.SceneParam{
		Code:               "contract",
		Name:               "Contract",
		MaxSize:            1024,
		AllowedExts:        strPtr(`["txt"]`),
		DefaultStorageCode: strPtr(model.DefaultStorageCode),
		DefaultVisibility:  model.VisibilityPrivate,
		TempTTLSeconds:     60,
		Enabled:            boolPtr(true),
	}); err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}
	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "contract.txt",
		ContentType: "text/plain",
		Size:        8,
		Reader:      strings.NewReader("contract"),
		SceneCode:   "contract",
		Actor:       service.Actor{UserID: intPtr(1), IsSuperAdmin: true},
	})
	if err != nil || uploaded.Ref.ID == 0 {
		t.Fatalf("Upload() = %+v, %v", uploaded, err)
	}

	if err := svc.DeleteScene(ctx, "contract"); err == nil {
		t.Fatal("DeleteScene() deleted referenced scene")
	}
}

func TestServicePreventsDeletingStorageInUse(t *testing.T) {
	ctx := context.Background()
	archive := "archive"
	repository := repo.NewMemoryRepository(repo.Seed{
		Storages: append(repo.SeedData().Storages, model.Storage{
			ID:        2,
			Code:      archive,
			Provider:  model.ProviderLocal,
			Prefix:    "archive",
			Enabled:   true,
			IsDefault: false,
		}),
		Scenes: append(repo.SeedData().Scenes, model.Scene{
			ID:                 4,
			Code:               "archive",
			Name:               "Archive",
			MaxSize:            1024,
			AllowedExts:        strPtr(`["txt"]`),
			DefaultStorageCode: &archive,
			DefaultVisibility:  model.VisibilityPrivate,
			TempTTLSeconds:     60,
			Enabled:            true,
		}),
	})
	registry := storage.NewRegistry()
	registry.Add(archive, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{TokenSecret: []byte("test-secret")})
	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "archive.txt",
		ContentType: "text/plain",
		Size:        7,
		Reader:      strings.NewReader("archive"),
		SceneCode:   "archive",
		Actor:       service.Actor{UserID: intPtr(1), IsSuperAdmin: true},
	})
	if err != nil || uploaded.File.ID == 0 {
		t.Fatalf("Upload() = %+v, %v", uploaded, err)
	}

	if err := svc.DeleteStorage(ctx, archive); err == nil {
		t.Fatal("DeleteStorage() deleted storage with objects")
	}
}

func TestServiceCleansExpiredTemporaryFilesAndKeepsActiveRefs(t *testing.T) {
	ctx := context.Background()
	current := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: root}))
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now: func() time.Time {
			return current
		},
	})

	tempUpload, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "temp.txt",
		ContentType: "text/plain",
		Size:        4,
		Reader:      strings.NewReader("temp"),
		SceneCode:   model.DefaultSceneCode,
	})
	if err != nil {
		t.Fatalf("temp Upload() error = %v", err)
	}
	activeUpload, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "active.txt",
		ContentType: "text/plain",
		Size:        6,
		Reader:      strings.NewReader("active"),
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "notice",
		SubjectID:   "1001",
	})
	if err != nil {
		t.Fatalf("active Upload() error = %v", err)
	}
	tempObject, err := repository.GetObject(ctx, tempUpload.File.ID)
	if err != nil {
		t.Fatalf("GetObject(temp) error = %v", err)
	}
	activeObject, err := repository.GetObject(ctx, activeUpload.File.ID)
	if err != nil {
		t.Fatalf("GetObject(active) error = %v", err)
	}
	tempPath := filepath.Join(root, filepath.FromSlash(tempObject.ObjectKey))
	activePath := filepath.Join(root, filepath.FromSlash(activeObject.ObjectKey))
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("temp file not created: %v", err)
	}
	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("active file not created: %v", err)
	}

	current = current.Add(25 * time.Hour)
	result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{})
	if err != nil {
		t.Fatalf("CleanupExpiredTemps() error = %v", err)
	}
	if result.ExpiredRefs != 1 || result.DeletedFiles != 1 {
		t.Fatalf("CleanupExpiredTemps() = %+v, want 1 expired ref and 1 deleted file", result)
	}
	reloadedTemp, err := repository.GetObject(ctx, tempUpload.File.ID)
	if err != nil {
		t.Fatalf("GetObject(reloaded temp) error = %v", err)
	}
	if reloadedTemp.Status != model.StatusDeleted {
		t.Fatalf("temp object status = %q, want deleted", reloadedTemp.Status)
	}
	reloadedActive, err := repository.GetObject(ctx, activeUpload.File.ID)
	if err != nil {
		t.Fatalf("GetObject(reloaded active) error = %v", err)
	}
	if reloadedActive.Status != model.StatusActive {
		t.Fatalf("active object status = %q, want active", reloadedActive.Status)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temp file stat error = %v, want not exists", err)
	}
	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("active file should remain: %v", err)
	}
}

func TestServiceCleanupExpiredTempsDryRunDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{Root: t.TempDir()}))
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now:         func() time.Time { return now },
	})
	uploaded, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "temp.txt",
		ContentType: "text/plain",
		Size:        4,
		Reader:      strings.NewReader("temp"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	svc = service.New(repository, registry, service.Options{
		TokenSecret: []byte("test-secret"),
		Now:         func() time.Time { return now.Add(48 * time.Hour) },
	})
	result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CleanupExpiredTemps(dry-run) error = %v", err)
	}
	if result.ExpiredRefs != 1 || result.DeletedFiles != 1 {
		t.Fatalf("dry-run result = %+v, want projected cleanup", result)
	}
	ref, err := repository.GetRef(ctx, uploaded.Ref.ID)
	if err != nil || ref.Status != model.RefStatusTemp {
		t.Fatalf("ref after dry-run = %+v, %v; want temp", ref, err)
	}
}

func TestServiceCleanupDeletesExpiredPendingPresignedUploads(t *testing.T) {
	ctx := context.Background()
	current := time.Now()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &fakePresignBackend{}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:            []byte("test-secret"),
		DirectUploadPresignTTL: time.Minute,
		Now:                    func() time.Time { return current },
	})

	result, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "pending.txt",
		ContentType: "text/plain",
		Size:        7,
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	})
	if err != nil {
		t.Fatalf("CreatePresignedUpload() error = %v", err)
	}

	current = current.Add(2 * time.Minute)
	cleanup, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{PendingTTL: time.Minute})
	if err != nil {
		t.Fatalf("CleanupExpiredTemps() error = %v", err)
	}
	if cleanup.PendingFiles != 1 || cleanup.DeletedFiles != 1 {
		t.Fatalf("CleanupExpiredTemps() = %+v, want 1 pending and 1 deleted file", cleanup)
	}
	object, err := repository.GetObject(ctx, result.File.ID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if object.Status != model.StatusDeleted {
		t.Fatalf("object status = %q, want deleted", object.Status)
	}
	ref, err := repository.GetRef(ctx, result.Ref.ID)
	if err != nil {
		t.Fatalf("GetRef() error = %v", err)
	}
	if ref.Status != model.RefStatusDeleted {
		t.Fatalf("ref status = %q, want deleted", ref.Status)
	}
	if len(backend.deletedKeys) != 1 {
		t.Fatalf("deleted keys = %+v, want one storage delete", backend.deletedKeys)
	}
}

func TestServiceRejectsMultipartUploadWhenTotalByteQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &quotaBackend{}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:   []byte("test-secret"),
		MaxTotalBytes: 8,
	})

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "first.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("first"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err != nil {
		t.Fatalf("first Upload() error = %v", err)
	}
	if backend.putCalls != 1 {
		t.Fatalf("put calls after first upload = %d, want 1", backend.putCalls)
	}

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "second.txt",
		ContentType: "text/plain",
		Size:        4,
		Reader:      strings.NewReader("next"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(8)},
	}); err == nil {
		t.Fatal("second Upload() succeeded over total byte quota")
	}
	if backend.putCalls != 1 {
		t.Fatalf("put calls after quota rejection = %d, want still 1", backend.putCalls)
	}
}

func TestServiceRejectsMultipartUploadWhenTotalFileQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &quotaBackend{}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:   []byte("test-secret"),
		MaxTotalFiles: 1,
	})

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "first.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("first"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err != nil {
		t.Fatalf("first Upload() error = %v", err)
	}
	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "second.txt",
		ContentType: "text/plain",
		Size:        1,
		Reader:      strings.NewReader("x"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(8)},
	}); err == nil {
		t.Fatal("second Upload() succeeded over total file quota")
	}
	if backend.putCalls != 1 {
		t.Fatalf("put calls after file quota rejection = %d, want 1", backend.putCalls)
	}
}

func TestServiceRejectsPresignWhenOwnerByteQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &quotaBackend{}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:   []byte("test-secret"),
		MaxOwnerBytes: 8,
	})

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "first.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("first"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err != nil {
		t.Fatalf("first Upload() error = %v", err)
	}
	if _, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "second.txt",
		ContentType: "text/plain",
		Size:        4,
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err == nil {
		t.Fatal("CreatePresignedUpload() succeeded over owner byte quota")
	}
	if backend.presignPutCalls != 0 {
		t.Fatalf("presign calls after quota rejection = %d, want 0", backend.presignPutCalls)
	}
}

func TestServiceRejectsPresignWhenOwnerFileQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
	registry := storage.NewRegistry()
	backend := &quotaBackend{}
	registry.Add(model.DefaultStorageCode, backend)
	svc := service.New(repository, registry, service.Options{
		TokenSecret:   []byte("test-secret"),
		MaxOwnerFiles: 1,
	})

	if _, err := svc.Upload(ctx, service.UploadInput{
		Filename:    "first.txt",
		ContentType: "text/plain",
		Size:        5,
		Reader:      strings.NewReader("first"),
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err != nil {
		t.Fatalf("first Upload() error = %v", err)
	}
	if _, err := svc.CreatePresignedUpload(ctx, service.PresignUploadInput{
		Filename:    "second.txt",
		ContentType: "text/plain",
		Size:        1,
		SceneCode:   model.DefaultSceneCode,
		Actor:       service.Actor{UserID: intPtr(7)},
	}); err == nil {
		t.Fatal("CreatePresignedUpload() succeeded over owner file quota")
	}
	if backend.presignPutCalls != 0 {
		t.Fatalf("presign calls after owner file quota rejection = %d, want 0", backend.presignPutCalls)
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

func boolPtr(value bool) *bool {
	return &value
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func quoteJSON(value string) string {
	quoted := strings.Builder{}
	quoted.WriteByte('"')
	for _, r := range value {
		if r == '\\' || r == '"' {
			quoted.WriteByte('\\')
		}
		quoted.WriteRune(r)
	}
	quoted.WriteByte('"')
	return quoted.String()
}

type fakePresignBackend struct {
	headInfo    storage.ObjectInfo
	headErr     error
	headKey     string
	deletedKeys []string
}

func (*fakePresignBackend) Put(context.Context, string, io.Reader, storage.PutOptions) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, storage.ErrUnsupported
}

func (*fakePresignBackend) Open(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, storage.ErrUnsupported
}

func (b *fakePresignBackend) Delete(_ context.Context, key string) error {
	b.deletedKeys = append(b.deletedKeys, key)
	return nil
}

func (b *fakePresignBackend) Head(_ context.Context, key string) (storage.ObjectInfo, error) {
	b.headKey = key
	if b.headErr != nil {
		return storage.ObjectInfo{}, b.headErr
	}
	info := b.headInfo
	info.Key = key
	return info, nil
}

func (*fakePresignBackend) PresignPut(_ context.Context, key string, ttl time.Duration, opts storage.PutOptions) (storage.PresignedURL, error) {
	return storage.PresignedURL{
		Method:    http.MethodPut,
		URL:       "https://signed.example.test/" + key,
		ExpiresAt: time.Now().Add(ttl),
		Headers:   map[string]string{"Content-Type": opts.ContentType},
	}, nil
}

func (*fakePresignBackend) PresignGet(context.Context, string, time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, storage.ErrUnsupported
}

func (*fakePresignBackend) PublicURL(key string) string {
	return "https://cdn.example.test/" + key
}

type quotaBackend struct {
	putCalls        int
	presignPutCalls int
	deletedKeys     []string
}

func (b *quotaBackend) Put(_ context.Context, key string, reader io.Reader, opts storage.PutOptions) (storage.ObjectInfo, error) {
	b.putCalls++
	body, err := io.ReadAll(reader)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{Key: key, Size: int64(len(body)), ContentType: opts.ContentType}, nil
}

func (*quotaBackend) Open(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, storage.ErrUnsupported
}

func (*quotaBackend) Head(_ context.Context, key string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Key: key}, nil
}

func (b *quotaBackend) Delete(_ context.Context, key string) error {
	b.deletedKeys = append(b.deletedKeys, key)
	return nil
}

func (b *quotaBackend) PresignPut(_ context.Context, key string, ttl time.Duration, opts storage.PutOptions) (storage.PresignedURL, error) {
	b.presignPutCalls++
	return storage.PresignedURL{
		Method:    http.MethodPut,
		URL:       "https://signed.example.test/" + key,
		ExpiresAt: time.Now().Add(ttl),
		Headers:   map[string]string{"Content-Type": opts.ContentType},
	}, nil
}

func (*quotaBackend) PresignGet(context.Context, string, time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, storage.ErrUnsupported
}

func (*quotaBackend) PublicURL(key string) string {
	return "https://cdn.example.test/" + key
}
