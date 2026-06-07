package repo

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

type MemoryRepository struct {
	mu           sync.Mutex
	storages     []model.Storage
	scenes       []model.Scene
	objects      []model.FileObject
	refs         []model.FileRef
	shares       []model.Share
	nextObjectID int
	nextRefID    int
	nextShareID  int
}

func NewMemoryRepository(seed Seed) *MemoryRepository {
	repo := &MemoryRepository{
		storages:     append([]model.Storage(nil), seed.Storages...),
		scenes:       append([]model.Scene(nil), seed.Scenes...),
		nextObjectID: 1,
		nextRefID:    1,
		nextShareID:  1,
	}
	return repo
}

func (r *MemoryRepository) GetScene(_ context.Context, code string) (model.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.scenes {
		if item.Code == code {
			return item, nil
		}
	}
	return model.Scene{}, ErrNotFound
}

func (r *MemoryRepository) ListScenes(context.Context) ([]model.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.Scene(nil), r.scenes...), nil
}

func (r *MemoryRepository) CreateScene(_ context.Context, param SaveSceneParam) (model.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := sceneFromParam(r.nextSceneIDLocked(), param)
	r.scenes = append(r.scenes, item)
	return item, nil
}

func (r *MemoryRepository) UpdateScene(_ context.Context, code string, param SaveSceneParam) (model.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.scenes {
		if r.scenes[i].Code != code {
			continue
		}
		item := sceneFromParam(r.scenes[i].ID, param)
		now := time.Now()
		item.CreatedTime = r.scenes[i].CreatedTime
		item.UpdatedTime = &now
		r.scenes[i] = item
		return item, nil
	}
	return model.Scene{}, ErrNotFound
}

func (r *MemoryRepository) DeleteScene(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.scenes {
		if r.scenes[i].Code == code {
			r.scenes = append(r.scenes[:i], r.scenes[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (r *MemoryRepository) nextSceneIDLocked() int {
	maxID := 0
	for _, item := range r.scenes {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	return maxID + 1
}

func (r *MemoryRepository) GetStorage(_ context.Context, code string) (model.Storage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.storages {
		if item.Code == code {
			return item, nil
		}
	}
	return model.Storage{}, ErrNotFound
}

func (r *MemoryRepository) GetDefaultStorage(context.Context) (model.Storage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.storages {
		if item.IsDefault && item.Enabled {
			return item, nil
		}
	}
	return model.Storage{}, ErrNotFound
}

func (r *MemoryRepository) ListStorages(context.Context) ([]model.Storage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.Storage(nil), r.storages...), nil
}

func (r *MemoryRepository) CreateStorage(_ context.Context, param SaveStorageParam) (model.Storage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if param.IsDefault {
		r.unsetDefaultStorageLocked()
	}
	item := storageFromParam(r.nextStorageIDLocked(), param)
	r.storages = append(r.storages, item)
	return item, nil
}

func (r *MemoryRepository) UpdateStorage(_ context.Context, code string, param SaveStorageParam) (model.Storage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.storages {
		if r.storages[i].Code != code {
			continue
		}
		if param.IsDefault {
			r.unsetDefaultStorageLocked()
		}
		item := storageFromParam(r.storages[i].ID, param)
		now := time.Now()
		item.CreatedTime = r.storages[i].CreatedTime
		item.UpdatedTime = &now
		r.storages[i] = item
		return item, nil
	}
	return model.Storage{}, ErrNotFound
}

func (r *MemoryRepository) DeleteStorage(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.storages {
		if r.storages[i].Code == code {
			r.storages = append(r.storages[:i], r.storages[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (r *MemoryRepository) nextStorageIDLocked() int {
	maxID := 0
	for _, item := range r.storages {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	return maxID + 1
}

func (r *MemoryRepository) unsetDefaultStorageLocked() {
	for i := range r.storages {
		r.storages[i].IsDefault = false
	}
}

func (r *MemoryRepository) CreateObject(_ context.Context, param CreateObjectParam) (model.FileObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := model.FileObject{
		ID:           r.nextObjectID,
		UUID:         param.UUID,
		StorageCode:  param.StorageCode,
		Provider:     param.Provider,
		Bucket:       param.Bucket,
		ObjectKey:    param.ObjectKey,
		OriginalName: param.OriginalName,
		Ext:          param.Ext,
		Mime:         param.Mime,
		Size:         param.Size,
		SHA256:       param.SHA256,
		ETag:         param.ETag,
		Visibility:   param.Visibility,
		Status:       param.Status,
		UploadedBy:   param.UploadedBy,
		CreatedTime:  time.Now(),
	}
	r.nextObjectID++
	r.objects = append(r.objects, item)
	return item, nil
}

func (r *MemoryRepository) GetObject(_ context.Context, id int) (model.FileObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.objects {
		if item.ID == id {
			return item, nil
		}
	}
	return model.FileObject{}, ErrNotFound
}

func (r *MemoryRepository) GetObjectByUUID(_ context.Context, uuid string) (model.FileObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.objects {
		if item.UUID == uuid {
			return item, nil
		}
	}
	return model.FileObject{}, ErrNotFound
}

func (r *MemoryRepository) ListObjects(_ context.Context, filter ObjectFilter, page int, size int) ([]model.FileObject, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.FileObject, 0, len(r.objects))
	for _, item := range r.objects {
		if filter.Keyword != "" && !strings.Contains(item.OriginalName, filter.Keyword) {
			continue
		}
		if filter.Provider != "" && item.Provider != filter.Provider {
			continue
		}
		if filter.StorageCode != "" && item.StorageCode != filter.StorageCode {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.UploadedBy != nil && (item.UploadedBy == nil || *item.UploadedBy != *filter.UploadedBy) {
			continue
		}
		if (filter.SceneCode != "" || filter.OwnerType != "" || filter.OwnerID != "") && !r.objectHasMatchingRef(item.ID, filter) {
			continue
		}
		items = append(items, item)
	}
	return paginate(items, page, size)
}

func (r *MemoryRepository) objectHasMatchingRef(fileID int, filter ObjectFilter) bool {
	for _, ref := range r.refs {
		if ref.FileID != fileID {
			continue
		}
		if filter.SceneCode != "" && ref.SceneCode != filter.SceneCode {
			continue
		}
		if filter.OwnerType != "" && stringPtrValue(ref.OwnerType) != filter.OwnerType {
			continue
		}
		if filter.OwnerID != "" && stringPtrValue(ref.OwnerID) != filter.OwnerID {
			continue
		}
		return true
	}
	return false
}

func (r *MemoryRepository) UpdateObjectStatus(_ context.Context, id int, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.objects {
		if r.objects[i].ID == id {
			r.objects[i].Status = status
			now := time.Now()
			r.objects[i].UpdatedTime = &now
			if status == model.StatusDeleted {
				r.objects[i].DeletedTime = &now
			}
			return nil
		}
	}
	return ErrNotFound
}

func (r *MemoryRepository) CreateRef(_ context.Context, param CreateRefParam) (model.FileRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := model.FileRef{
		ID:          r.nextRefID,
		FileID:      param.FileID,
		SceneCode:   param.SceneCode,
		SubjectType: param.SubjectType,
		SubjectID:   param.SubjectID,
		Field:       param.Field,
		DisplayName: param.DisplayName,
		Sort:        param.Sort,
		Status:      param.Status,
		ExpiresAt:   param.ExpiresAt,
		OwnerType:   param.OwnerType,
		OwnerID:     param.OwnerID,
		CreatedBy:   param.CreatedBy,
		Metadata:    param.Metadata,
		CreatedTime: time.Now(),
	}
	r.nextRefID++
	r.refs = append(r.refs, item)
	return item, nil
}

func (r *MemoryRepository) GetRef(_ context.Context, id int) (model.FileRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.refs {
		if item.ID == id {
			return item, nil
		}
	}
	return model.FileRef{}, ErrNotFound
}

func (r *MemoryRepository) ListRefs(_ context.Context, filter RefFilter, page int, size int) ([]model.FileRef, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.FileRef, 0, len(r.refs))
	for _, item := range r.refs {
		if filter.FileID != nil && item.FileID != *filter.FileID {
			continue
		}
		if filter.SceneCode != "" && item.SceneCode != filter.SceneCode {
			continue
		}
		if filter.SubjectType != "" && stringPtrValue(item.SubjectType) != filter.SubjectType {
			continue
		}
		if filter.SubjectID != "" && stringPtrValue(item.SubjectID) != filter.SubjectID {
			continue
		}
		if filter.Field != "" && stringPtrValue(item.Field) != filter.Field {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.OwnerType != "" && stringPtrValue(item.OwnerType) != filter.OwnerType {
			continue
		}
		if filter.OwnerID != "" && stringPtrValue(item.OwnerID) != filter.OwnerID {
			continue
		}
		items = append(items, item)
	}
	return paginate(items, page, size)
}

func (r *MemoryRepository) ListExpiredTempRefs(_ context.Context, now time.Time) ([]model.FileRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.FileRef, 0)
	for _, item := range r.refs {
		if item.Status != model.RefStatusTemp || item.ExpiresAt == nil {
			continue
		}
		if item.ExpiresAt.After(now) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *MemoryRepository) CountRefsByFileStatus(_ context.Context, fileID int, statuses []string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	statusSet := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		statusSet[status] = true
	}
	var count int64
	for _, item := range r.refs {
		if item.FileID == fileID && statusSet[item.Status] {
			count++
		}
	}
	return count, nil
}

func (r *MemoryRepository) CountRefsByScene(_ context.Context, sceneCode string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, item := range r.refs {
		if item.SceneCode == sceneCode && item.Status != model.RefStatusDeleted {
			count++
		}
	}
	return count, nil
}

func (r *MemoryRepository) CountObjectsByStorage(_ context.Context, storageCode string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, item := range r.objects {
		if item.StorageCode == storageCode && item.Status != model.StatusDeleted {
			count++
		}
	}
	return count, nil
}

func (r *MemoryRepository) CountScenesByStorage(_ context.Context, storageCode string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, item := range r.scenes {
		if stringPtrValue(item.DefaultStorageCode) == storageCode {
			count++
		}
	}
	return count, nil
}

func (r *MemoryRepository) BindRefs(_ context.Context, param BindRefsParam) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, fileID := range param.FileIDs {
		found := false
		for i := range r.refs {
			if r.refs[i].FileID == fileID && r.refs[i].SceneCode == param.SceneCode && r.refs[i].Status == model.RefStatusTemp {
				r.refs[i].Status = model.RefStatusActive
				r.refs[i].SubjectType = &param.SubjectType
				r.refs[i].SubjectID = &param.SubjectID
				r.refs[i].Field = &param.Field
				r.refs[i].OwnerType = param.OwnerType
				r.refs[i].OwnerID = param.OwnerID
				now := time.Now()
				r.refs[i].UpdatedTime = &now
				found = true
			}
		}
		if !found {
			item := model.FileRef{
				ID:          r.nextRefID,
				FileID:      fileID,
				SceneCode:   param.SceneCode,
				SubjectType: &param.SubjectType,
				SubjectID:   &param.SubjectID,
				Field:       &param.Field,
				Status:      model.RefStatusActive,
				OwnerType:   param.OwnerType,
				OwnerID:     param.OwnerID,
				CreatedTime: time.Now(),
			}
			r.nextRefID++
			r.refs = append(r.refs, item)
		}
	}
	return nil
}

func (r *MemoryRepository) UpdateRefsStatus(_ context.Context, ids []int, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	updated := 0
	for _, id := range ids {
		for i := range r.refs {
			if r.refs[i].ID == id {
				r.refs[i].Status = status
				now := time.Now()
				r.refs[i].UpdatedTime = &now
				if status == model.RefStatusDeleted {
					r.refs[i].DeletedTime = &now
				}
				updated++
			}
		}
	}
	if updated == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemoryRepository) CreateShare(_ context.Context, param CreateShareParam) (model.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := model.Share{
		ID:           r.nextShareID,
		FileID:       param.FileID,
		RefID:        param.RefID,
		Token:        param.Token,
		PasswordHash: param.PasswordHash,
		ExpiresAt:    param.ExpiresAt,
		MaxDownloads: param.MaxDownloads,
		Status:       param.Status,
		CreatedBy:    param.CreatedBy,
		CreatedTime:  time.Now(),
	}
	r.nextShareID++
	r.shares = append(r.shares, item)
	return item, nil
}

func (r *MemoryRepository) GetShare(_ context.Context, id int) (model.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.shares {
		if item.ID == id {
			return item, nil
		}
	}
	return model.Share{}, ErrNotFound
}

func (r *MemoryRepository) GetShareByToken(_ context.Context, token string) (model.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.shares {
		if item.Token == token {
			return item, nil
		}
	}
	return model.Share{}, ErrNotFound
}

func (r *MemoryRepository) ListShares(_ context.Context, filter ShareFilter, page int, size int) ([]model.Share, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.Share, 0, len(r.shares))
	for _, item := range r.shares {
		if filter.FileID != nil && item.FileID != *filter.FileID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.CreatedBy != nil && (item.CreatedBy == nil || *item.CreatedBy != *filter.CreatedBy) {
			continue
		}
		items = append(items, item)
	}
	return paginate(items, page, size)
}

func (r *MemoryRepository) DisableShare(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.shares {
		if r.shares[i].ID == id {
			r.shares[i].Status = model.ShareStatusDisabled
			now := time.Now()
			r.shares[i].UpdatedTime = &now
			return nil
		}
	}
	return ErrNotFound
}

func (r *MemoryRepository) IncrementShareDownload(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.shares {
		if r.shares[i].ID == id {
			r.shares[i].DownloadCount++
			now := time.Now()
			r.shares[i].UpdatedTime = &now
			return nil
		}
	}
	return ErrNotFound
}

func paginate[T any](items []T, page int, size int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	total := int64(len(items))
	start := (page - 1) * size
	if start >= len(items) {
		return []T{}, total, nil
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return append([]T(nil), items[start:end]...), total, nil
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func storageFromParam(id int, param SaveStorageParam) model.Storage {
	return model.Storage{
		ID:          id,
		Code:        param.Code,
		Provider:    param.Provider,
		Bucket:      param.Bucket,
		Region:      param.Region,
		Endpoint:    param.Endpoint,
		BaseURL:     param.BaseURL,
		Prefix:      param.Prefix,
		IsDefault:   param.IsDefault,
		Enabled:     param.Enabled,
		Config:      param.Config,
		CreatedTime: time.Now(),
	}
}

func sceneFromParam(id int, param SaveSceneParam) model.Scene {
	return model.Scene{
		ID:                 id,
		Code:               param.Code,
		Name:               param.Name,
		MaxSize:            param.MaxSize,
		AllowedExts:        param.AllowedExts,
		AllowedMimes:       param.AllowedMimes,
		DefaultStorageCode: param.DefaultStorageCode,
		DefaultVisibility:  param.DefaultVisibility,
		TempTTLSeconds:     param.TempTTLSeconds,
		Enabled:            param.Enabled,
		CreatedTime:        time.Now(),
	}
}
