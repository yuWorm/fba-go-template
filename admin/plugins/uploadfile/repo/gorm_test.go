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
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:      object.ID,
		SceneCode:   model.DefaultSceneCode,
		SubjectType: strPtrGORM("order"),
		SubjectID:   strPtrGORM("SO-1"),
		Field:       strPtrGORM("invoice"),
		Status:      model.RefStatusActive,
		OwnerType:   strPtrGORM("user"),
		OwnerID:     strPtrGORM("7"),
		CreatedBy:   intPtrGORM(7),
	}); err != nil {
		t.Fatalf("CreateRef() error = %v", err)
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
