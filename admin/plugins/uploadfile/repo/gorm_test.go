package repo_test

import (
	"context"
	"strings"
	"testing"

	uploadmigration "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/migration"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go/core/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGORMRepositoryPersistsUploadLifecycleAndOwnerFilters(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := uploadmigration.InitialData(provider).Up(ctx); err != nil {
		t.Fatalf("InitialData() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)

	object, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "file-1",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/file-1.txt",
		OriginalName: "file-1.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         6,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
		UploadedBy:   intPtrGORM(7),
	})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	ref, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:      object.ID,
		SceneCode:   model.DefaultSceneCode,
		SubjectType: strPtrGORM("order"),
		SubjectID:   strPtrGORM("SO-1"),
		Field:       strPtrGORM("invoice"),
		Status:      model.RefStatusActive,
		OwnerType:   strPtrGORM("user"),
		OwnerID:     strPtrGORM("7"),
		CreatedBy:   intPtrGORM(7),
	})
	if err != nil {
		t.Fatalf("CreateRef() error = %v", err)
	}
	loadedRef, err := repository.GetRef(ctx, ref.ID)
	if err != nil {
		t.Fatalf("GetRef() error = %v", err)
	}
	if loadedRef.ID != ref.ID || loadedRef.FileID != object.ID {
		t.Fatalf("GetRef() = %+v, want ref %d file %d", loadedRef, ref.ID, object.ID)
	}

	refs, total, err := repository.ListRefs(ctx, repo.RefFilter{
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "order",
		SubjectID:   "SO-1",
		Field:       "invoice",
		OwnerType:   "user",
		OwnerID:     "7",
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListRefs() error = %v", err)
	}
	if total != 1 || len(refs) != 1 {
		t.Fatalf("ListRefs total=%d len=%d, want 1", total, len(refs))
	}

	objects, total, err := repository.ListObjects(ctx, repo.ObjectFilter{
		SceneCode: model.DefaultSceneCode,
		OwnerType: "user",
		OwnerID:   "7",
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if total != 1 || len(objects) != 1 || objects[0].ID != object.ID {
		t.Fatalf("ListObjects total=%d items=%v, want object %d", total, objects, object.ID)
	}
	sceneRefs, err := repository.CountRefsByScene(ctx, model.DefaultSceneCode)
	if err != nil {
		t.Fatalf("CountRefsByScene() error = %v", err)
	}
	if sceneRefs != 1 {
		t.Fatalf("CountRefsByScene() = %d, want 1", sceneRefs)
	}
	storageObjects, err := repository.CountObjectsByStorage(ctx, model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("CountObjectsByStorage() error = %v", err)
	}
	if storageObjects != 1 {
		t.Fatalf("CountObjectsByStorage() = %d, want 1", storageObjects)
	}
	storageScenes, err := repository.CountScenesByStorage(ctx, model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("CountScenesByStorage() error = %v", err)
	}
	if storageScenes != int64(len(model.SeedScenes())) {
		t.Fatalf("CountScenesByStorage() = %d, want seed scene count", storageScenes)
	}

	tempObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "avatar-1",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/avatar/avatar-1.png",
		OriginalName: "avatar.png",
		Ext:          "png",
		Mime:         "image/png",
		Size:         10,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(temp) error = %v", err)
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    tempObject.ID,
		SceneCode: model.SceneAvatar,
		Status:    model.RefStatusTemp,
	}); err != nil {
		t.Fatalf("CreateRef(temp) error = %v", err)
	}
	if err := repository.BindRefs(ctx, repo.BindRefsParam{
		FileIDs:     []int{tempObject.ID},
		SceneCode:   model.SceneAvatar,
		SubjectType: "profile",
		SubjectID:   "7",
		Field:       "avatar",
		OwnerType:   strPtrGORM("user"),
		OwnerID:     strPtrGORM("7"),
	}); err != nil {
		t.Fatalf("BindRefs() error = %v", err)
	}
	bound, total, err := repository.ListRefs(ctx, repo.RefFilter{FileID: &tempObject.ID, Status: model.RefStatusActive}, 1, 20)
	if err != nil {
		t.Fatalf("ListRefs(bound) error = %v", err)
	}
	if total != 1 || len(bound) != 1 || ptrValue(bound[0].SubjectType) != "profile" || ptrValue(bound[0].OwnerID) != "7" {
		t.Fatalf("bound refs total=%d items=%v, want active profile owner 7", total, bound)
	}

	share, err := repository.CreateShare(ctx, repo.CreateShareParam{
		FileID:       object.ID,
		Token:        "share-token",
		PasswordHash: strPtrGORM("hash"),
		Status:       model.ShareStatusActive,
		CreatedBy:    intPtrGORM(7),
	})
	if err != nil {
		t.Fatalf("CreateShare() error = %v", err)
	}
	if err := repository.IncrementShareDownload(ctx, share.ID); err != nil {
		t.Fatalf("IncrementShareDownload() error = %v", err)
	}
	share, err = repository.GetShareByToken(ctx, "share-token")
	if err != nil {
		t.Fatalf("GetShareByToken() error = %v", err)
	}
	if share.DownloadCount != 1 {
		t.Fatalf("DownloadCount = %d, want 1", share.DownloadCount)
	}
}

func TestGORMRepositoryListObjectsIgnoresDeletedRefsForOwnerFilters(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := uploadmigration.InitialData(provider).Up(ctx); err != nil {
		t.Fatalf("InitialData() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)
	object, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "deleted-ref-object",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/deleted-ref-object.txt",
		OriginalName: "deleted-ref-object.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         1,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    object.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusDeleted,
		OwnerType: strPtrGORM("user"),
		OwnerID:   strPtrGORM("7"),
	}); err != nil {
		t.Fatalf("CreateRef() error = %v", err)
	}

	objects, total, err := repository.ListObjects(ctx, repo.ObjectFilter{
		SceneCode: model.DefaultSceneCode,
		OwnerType: "user",
		OwnerID:   "7",
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if total != 0 || len(objects) != 0 {
		t.Fatalf("ListObjects() total=%d items=%+v, want empty for deleted ref", total, objects)
	}
}

func TestGORMRepositoryUploadUsageCountsLiveObjectsAndDistinctOwnerRefs(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := uploadmigration.InitialData(provider).Up(ctx); err != nil {
		t.Fatalf("InitialData() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)
	active, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "usage-active",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/usage-active.txt",
		OriginalName: "usage-active.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         5,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(active) error = %v", err)
	}
	pending, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "usage-pending",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/usage-pending.txt",
		OriginalName: "usage-pending.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         7,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusPending,
	})
	if err != nil {
		t.Fatalf("CreateObject(pending) error = %v", err)
	}
	if _, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "usage-deleted",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/usage-deleted.txt",
		OriginalName: "usage-deleted.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         11,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusDeleted,
	}); err != nil {
		t.Fatalf("CreateObject(deleted) error = %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
			FileID:    active.ID,
			SceneCode: model.DefaultSceneCode,
			Status:    model.RefStatusActive,
			OwnerType: strPtrGORM("user"),
			OwnerID:   strPtrGORM("7"),
		}); err != nil {
			t.Fatalf("CreateRef(active duplicate %d) error = %v", i, err)
		}
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    pending.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusActive,
		OwnerType: strPtrGORM("user"),
		OwnerID:   strPtrGORM("8"),
	}); err != nil {
		t.Fatalf("CreateRef(pending owner 8) error = %v", err)
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    pending.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusDeleted,
		OwnerType: strPtrGORM("user"),
		OwnerID:   strPtrGORM("7"),
	}); err != nil {
		t.Fatalf("CreateRef(deleted owner 7) error = %v", err)
	}

	global, err := repository.UploadUsage(ctx, repo.UsageFilter{})
	if err != nil {
		t.Fatalf("UploadUsage(global) error = %v", err)
	}
	if global.Files != 2 || global.Bytes != 12 {
		t.Fatalf("global usage = %+v, want 2 files and 12 bytes", global)
	}
	owner, err := repository.UploadUsage(ctx, repo.UsageFilter{OwnerType: "user", OwnerID: "7"})
	if err != nil {
		t.Fatalf("UploadUsage(owner) error = %v", err)
	}
	if owner.Files != 1 || owner.Bytes != 5 {
		t.Fatalf("owner usage = %+v, want distinct active object only", owner)
	}
}

func TestGORMRepositoryUploadUsageFilters(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := uploadmigration.InitialData(provider).Up(ctx); err != nil {
		t.Fatalf("InitialData() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)
	defaultObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "stats-default",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/stats-default.txt",
		OriginalName: "stats-default.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         5,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(default) error = %v", err)
	}
	archiveObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "stats-archive",
		StorageCode:  "archive",
		Provider:     model.ProviderS3,
		ObjectKey:    "uploads/avatar/stats-archive.png",
		OriginalName: "stats-archive.png",
		Ext:          "png",
		Mime:         "image/png",
		Size:         3,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(archive) error = %v", err)
	}
	pendingObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "stats-pending",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/stats-pending.txt",
		OriginalName: "stats-pending.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         7,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusPending,
	})
	if err != nil {
		t.Fatalf("CreateObject(pending) error = %v", err)
	}
	deletedRefObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "stats-deleted-ref",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/stats-deleted-ref.txt",
		OriginalName: "stats-deleted-ref.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         11,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(deleted ref) error = %v", err)
	}
	deletedObject, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "stats-deleted-object",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/stats-deleted-object.txt",
		OriginalName: "stats-deleted-object.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         13,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusDeleted,
	})
	if err != nil {
		t.Fatalf("CreateObject(deleted object) error = %v", err)
	}
	refs := []repo.CreateRefParam{
		{FileID: defaultObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtrGORM("user"), OwnerID: strPtrGORM("7")},
		{FileID: archiveObject.ID, SceneCode: model.SceneAvatar, Status: model.RefStatusActive, OwnerType: strPtrGORM("user"), OwnerID: strPtrGORM("7")},
		{FileID: pendingObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtrGORM("user"), OwnerID: strPtrGORM("8")},
		{FileID: deletedRefObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusDeleted, OwnerType: strPtrGORM("user"), OwnerID: strPtrGORM("7")},
		{FileID: deletedObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtrGORM("user"), OwnerID: strPtrGORM("7")},
	}
	for i, ref := range refs {
		if _, err := repository.CreateRef(ctx, ref); err != nil {
			t.Fatalf("CreateRef(%d) error = %v", i, err)
		}
	}

	assertUsage(t, repository, repo.UsageFilter{SceneCode: model.DefaultSceneCode}, 2, 12)
	assertUsage(t, repository, repo.UsageFilter{SceneCode: model.DefaultSceneCode, OwnerType: "user", OwnerID: "7"}, 1, 5)
	assertUsage(t, repository, repo.UsageFilter{StorageCode: model.DefaultStorageCode}, 3, 23)
	assertUsage(t, repository, repo.UsageFilter{Provider: model.ProviderS3}, 1, 3)
	assertUsage(t, repository, repo.UsageFilter{Status: model.StatusPending}, 1, 7)
}

func TestGORMRepositoryManagesScenes(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)

	scene, err := repository.CreateScene(ctx, repo.SaveSceneParam{
		Code:               "contract",
		Name:               "Contract",
		MaxSize:            1024,
		AllowedExts:        strPtrGORM(`["txt"]`),
		DefaultStorageCode: strPtrGORM(model.DefaultStorageCode),
		DefaultVisibility:  model.VisibilityPrivate,
		TempTTLSeconds:     120,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}
	if scene.ID == 0 || scene.Code != "contract" {
		t.Fatalf("created scene = %+v, want id and code contract", scene)
	}

	scene, err = repository.UpdateScene(ctx, "contract", repo.SaveSceneParam{
		Code:               "contract",
		Name:               "Contract Files",
		MaxSize:            2048,
		AllowedExts:        strPtrGORM(`["txt","pdf"]`),
		DefaultStorageCode: strPtrGORM(model.DefaultStorageCode),
		DefaultVisibility:  model.VisibilityPrivate,
		TempTTLSeconds:     300,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("UpdateScene() error = %v", err)
	}
	if scene.Name != "Contract Files" || scene.MaxSize != 2048 {
		t.Fatalf("updated scene = %+v, want Contract Files max 2048", scene)
	}

	if err := repository.DeleteScene(ctx, "contract"); err != nil {
		t.Fatalf("DeleteScene() error = %v", err)
	}
	if _, err := repository.GetScene(ctx, "contract"); err == nil {
		t.Fatal("GetScene() found deleted scene")
	}
}

func TestInitialDataUsesConfiguredSeed(t *testing.T) {
	ctx := context.Background()
	provider := newSQLiteProvider(t)
	if err := uploadmigration.AutoMigrate(provider).Up(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	seed := repo.SeedData()
	bucket := "configured-bucket"
	region := "ap-southeast-1"
	baseURL := "https://cdn.example.test/files"
	config := `{"force_path_style":true}`
	seed.Storages[0].Provider = model.ProviderS3
	seed.Storages[0].Bucket = &bucket
	seed.Storages[0].Region = &region
	seed.Storages[0].BaseURL = &baseURL
	seed.Storages[0].Config = &config
	seed.Scenes[0].MaxSize = 12345
	seed.Scenes[0].TempTTLSeconds = 600

	if err := uploadmigration.InitialData(provider, seed).Up(ctx); err != nil {
		t.Fatalf("InitialData() error = %v", err)
	}
	repository := repo.NewGORMRepository(provider)
	storageConfig, err := repository.GetStorage(ctx, model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("GetStorage() error = %v", err)
	}
	if storageConfig.Provider != model.ProviderS3 || ptrValue(storageConfig.Bucket) != bucket || ptrValue(storageConfig.Region) != region {
		t.Fatalf("storage = %+v, want configured S3 seed", storageConfig)
	}
	if ptrValue(storageConfig.BaseURL) != baseURL || ptrValue(storageConfig.Config) != config {
		t.Fatalf("storage url/config = %v / %v", storageConfig.BaseURL, storageConfig.Config)
	}
	scene, err := repository.GetScene(ctx, model.DefaultSceneCode)
	if err != nil {
		t.Fatalf("GetScene() error = %v", err)
	}
	if scene.MaxSize != 12345 || scene.TempTTLSeconds != 600 {
		t.Fatalf("scene = %+v, want configured limits", scene)
	}
}

func newSQLiteProvider(t *testing.T) db.Provider {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	gormDB, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db.NewGORMProvider(gormDB, nil)
}

func strPtrGORM(value string) *string {
	return &value
}

func intPtrGORM(value int) *int {
	return &value
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
