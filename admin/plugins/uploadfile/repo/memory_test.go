package repo_test

import (
	"context"
	"testing"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
)

func TestMemoryRepositoryStoresObjectsRefsAndShares(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())

	scene, err := repository.GetScene(ctx, model.DefaultSceneCode)
	if err != nil || scene.Code != model.DefaultSceneCode {
		t.Fatalf("GetScene() = %+v, %v", scene, err)
	}
	storageConfig, err := repository.GetDefaultStorage(ctx)
	if err != nil || storageConfig.Code != model.DefaultStorageCode {
		t.Fatalf("GetDefaultStorage() = %+v, %v", storageConfig, err)
	}

	object, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "file-uuid",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/file.txt",
		OriginalName: "file.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         5,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	ref, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:      object.ID,
		SceneCode:   model.DefaultSceneCode,
		SubjectType: strPtr("notice"),
		SubjectID:   strPtr("1001"),
		Field:       strPtr("attachments"),
		Status:      model.RefStatusTemp,
		CreatedBy:   intPtr(1),
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

	if err := repository.BindRefs(ctx, repo.BindRefsParam{
		FileIDs:     []int{object.ID},
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "notice",
		SubjectID:   "1001",
		Field:       "attachments",
		OwnerType:   strPtr("user"),
		OwnerID:     strPtr("1"),
	}); err != nil {
		t.Fatalf("BindRefs() error = %v", err)
	}

	refs, _, err := repository.ListRefs(ctx, repo.RefFilter{
		SceneCode:   model.DefaultSceneCode,
		SubjectType: "notice",
		SubjectID:   "1001",
		Field:       "attachments",
		Status:      model.RefStatusActive,
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListRefs() error = %v", err)
	}
	if len(refs) != 1 || refs[0].ID != ref.ID || refs[0].Status != model.RefStatusActive {
		t.Fatalf("ListRefs() = %+v", refs)
	}
	other, err := repository.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         "other-file-uuid",
		StorageCode:  model.DefaultStorageCode,
		Provider:     model.ProviderLocal,
		ObjectKey:    "uploads/default/other.txt",
		OriginalName: "other.txt",
		Ext:          "txt",
		Mime:         "text/plain",
		Size:         5,
		Visibility:   model.VisibilityPrivate,
		Status:       model.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateObject(other) error = %v", err)
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    other.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusActive,
		OwnerType: strPtr("user"),
		OwnerID:   strPtr("2"),
	}); err != nil {
		t.Fatalf("CreateRef(other) error = %v", err)
	}
	objects, total, err := repository.ListObjects(ctx, repo.ObjectFilter{
		SceneCode: model.DefaultSceneCode,
		OwnerType: "user",
		OwnerID:   "1",
	}, 1, 20)
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if total != 1 || len(objects) != 1 || objects[0].ID != object.ID {
		t.Fatalf("ListObjects() total=%d items=%+v, want object %d", total, objects, object.ID)
	}
	sceneRefs, err := repository.CountRefsByScene(ctx, model.DefaultSceneCode)
	if err != nil {
		t.Fatalf("CountRefsByScene() error = %v", err)
	}
	if sceneRefs != 2 {
		t.Fatalf("CountRefsByScene() = %d, want 2", sceneRefs)
	}
	storageObjects, err := repository.CountObjectsByStorage(ctx, model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("CountObjectsByStorage() error = %v", err)
	}
	if storageObjects != 2 {
		t.Fatalf("CountObjectsByStorage() = %d, want 2", storageObjects)
	}
	storageScenes, err := repository.CountScenesByStorage(ctx, model.DefaultStorageCode)
	if err != nil {
		t.Fatalf("CountScenesByStorage() error = %v", err)
	}
	if storageScenes != int64(len(model.SeedScenes())) {
		t.Fatalf("CountScenesByStorage() = %d, want seed scene count", storageScenes)
	}

	share, err := repository.CreateShare(ctx, repo.CreateShareParam{
		FileID:       object.ID,
		RefID:        &ref.ID,
		Token:        "share-token",
		PasswordHash: strPtr("hash"),
		Status:       model.ShareStatusActive,
		CreatedBy:    intPtr(1),
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
	if err := repository.DisableShare(ctx, share.ID); err != nil {
		t.Fatalf("DisableShare() error = %v", err)
	}
	share, _ = repository.GetShareByToken(ctx, "share-token")
	if share.Status != model.ShareStatusDisabled {
		t.Fatalf("share status = %q, want disabled", share.Status)
	}
}

func strPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
