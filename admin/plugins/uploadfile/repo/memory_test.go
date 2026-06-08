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

func TestMemoryRepositoryListObjectsIgnoresDeletedRefsForOwnerFilters(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
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
		OwnerType: strPtr("user"),
		OwnerID:   strPtr("7"),
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

func TestMemoryRepositoryUploadUsageCountsLiveObjectsAndDistinctOwnerRefs(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
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
			OwnerType: strPtr("user"),
			OwnerID:   strPtr("7"),
		}); err != nil {
			t.Fatalf("CreateRef(active duplicate %d) error = %v", i, err)
		}
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    pending.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusActive,
		OwnerType: strPtr("user"),
		OwnerID:   strPtr("8"),
	}); err != nil {
		t.Fatalf("CreateRef(pending owner 8) error = %v", err)
	}
	if _, err := repository.CreateRef(ctx, repo.CreateRefParam{
		FileID:    pending.ID,
		SceneCode: model.DefaultSceneCode,
		Status:    model.RefStatusDeleted,
		OwnerType: strPtr("user"),
		OwnerID:   strPtr("7"),
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

func TestMemoryRepositoryUploadUsageFilters(t *testing.T) {
	ctx := context.Background()
	repository := repo.NewMemoryRepository(repo.SeedData())
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
		{FileID: defaultObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtr("user"), OwnerID: strPtr("7")},
		{FileID: archiveObject.ID, SceneCode: model.SceneAvatar, Status: model.RefStatusActive, OwnerType: strPtr("user"), OwnerID: strPtr("7")},
		{FileID: pendingObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtr("user"), OwnerID: strPtr("8")},
		{FileID: deletedRefObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusDeleted, OwnerType: strPtr("user"), OwnerID: strPtr("7")},
		{FileID: deletedObject.ID, SceneCode: model.DefaultSceneCode, Status: model.RefStatusActive, OwnerType: strPtr("user"), OwnerID: strPtr("7")},
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

func assertUsage(t *testing.T, repository repo.Repository, filter repo.UsageFilter, wantFiles int64, wantBytes int64) {
	t.Helper()
	stats, err := repository.UploadUsage(context.Background(), filter)
	if err != nil {
		t.Fatalf("UploadUsage(%+v) error = %v", filter, err)
	}
	if stats.Files != wantFiles || stats.Bytes != wantBytes {
		t.Fatalf("UploadUsage(%+v) = %+v, want files=%d bytes=%d", filter, stats, wantFiles, wantBytes)
	}
}

func strPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
