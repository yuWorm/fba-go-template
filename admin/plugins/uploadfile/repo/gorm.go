package repo

import (
	"context"
	"errors"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go/core/db"
	"gorm.io/gorm"
)

type GORMRepository struct {
	provider db.Provider
}

func NewGORMRepository(provider db.Provider) *GORMRepository {
	return &GORMRepository{provider: provider}
}

func (r *GORMRepository) GetScene(ctx context.Context, code string) (model.Scene, error) {
	var item model.Scene
	err := r.provider.Read().WithContext(ctx).Where("code = ?", code).First(&item).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) ListScenes(ctx context.Context) ([]model.Scene, error) {
	var items []model.Scene
	err := r.provider.Read().WithContext(ctx).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *GORMRepository) GetStorage(ctx context.Context, code string) (model.Storage, error) {
	var item model.Storage
	err := r.provider.Read().WithContext(ctx).Where("code = ?", code).First(&item).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) GetDefaultStorage(ctx context.Context) (model.Storage, error) {
	var item model.Storage
	err := r.provider.Read().WithContext(ctx).Where("is_default = ? AND enabled = ?", true, true).First(&item).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) ListStorages(ctx context.Context) ([]model.Storage, error) {
	var items []model.Storage
	err := r.provider.Read().WithContext(ctx).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *GORMRepository) CreateObject(ctx context.Context, param CreateObjectParam) (model.FileObject, error) {
	item := model.FileObject{
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
	}
	if err := r.provider.Write().WithContext(ctx).Create(&item).Error; err != nil {
		return model.FileObject{}, err
	}
	return item, nil
}

func (r *GORMRepository) GetObject(ctx context.Context, id int) (model.FileObject, error) {
	var item model.FileObject
	err := r.provider.Read().WithContext(ctx).First(&item, id).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) GetObjectByUUID(ctx context.Context, uuid string) (model.FileObject, error) {
	var item model.FileObject
	err := r.provider.Read().WithContext(ctx).Where("uuid = ?", uuid).First(&item).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) ListObjects(ctx context.Context, filter ObjectFilter, page int, size int) ([]model.FileObject, int64, error) {
	query := r.provider.Read().WithContext(ctx).Model(&model.FileObject{})
	if filter.Keyword != "" {
		query = query.Where("original_name LIKE ?", "%"+filter.Keyword+"%")
	}
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.StorageCode != "" {
		query = query.Where("storage_code = ?", filter.StorageCode)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UploadedBy != nil {
		query = query.Where("uploaded_by = ?", *filter.UploadedBy)
	}
	if filter.SceneCode != "" || filter.OwnerType != "" || filter.OwnerID != "" {
		subquery := r.provider.Read().WithContext(ctx).Model(&model.FileRef{}).Select("file_id")
		if filter.SceneCode != "" {
			subquery = subquery.Where("scene_code = ?", filter.SceneCode)
		}
		if filter.OwnerType != "" {
			subquery = subquery.Where("owner_type = ?", filter.OwnerType)
		}
		if filter.OwnerID != "" {
			subquery = subquery.Where("owner_id = ?", filter.OwnerID)
		}
		query = query.Where("id IN (?)", subquery)
	}
	return paginateGORM[model.FileObject](query.Order("id ASC"), page, size)
}

func (r *GORMRepository) UpdateObjectStatus(ctx context.Context, id int, status string) error {
	updates := map[string]any{
		"status":       status,
		"updated_time": time.Now(),
	}
	if status == model.StatusDeleted {
		updates["deleted_time"] = time.Now()
	}
	result := r.provider.Write().WithContext(ctx).Model(&model.FileObject{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMRepository) CreateRef(ctx context.Context, param CreateRefParam) (model.FileRef, error) {
	item := model.FileRef{
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
	}
	if err := r.provider.Write().WithContext(ctx).Create(&item).Error; err != nil {
		return model.FileRef{}, err
	}
	return item, nil
}

func (r *GORMRepository) ListRefs(ctx context.Context, filter RefFilter, page int, size int) ([]model.FileRef, int64, error) {
	query := r.provider.Read().WithContext(ctx).Model(&model.FileRef{})
	if filter.FileID != nil {
		query = query.Where("file_id = ?", *filter.FileID)
	}
	if filter.SceneCode != "" {
		query = query.Where("scene_code = ?", filter.SceneCode)
	}
	if filter.SubjectType != "" {
		query = query.Where("subject_type = ?", filter.SubjectType)
	}
	if filter.SubjectID != "" {
		query = query.Where("subject_id = ?", filter.SubjectID)
	}
	if filter.Field != "" {
		query = query.Where("field = ?", filter.Field)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.OwnerType != "" {
		query = query.Where("owner_type = ?", filter.OwnerType)
	}
	if filter.OwnerID != "" {
		query = query.Where("owner_id = ?", filter.OwnerID)
	}
	return paginateGORM[model.FileRef](query.Order("id ASC"), page, size)
}

func (r *GORMRepository) BindRefs(ctx context.Context, param BindRefsParam) error {
	return r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		for _, fileID := range param.FileIDs {
			updates := map[string]any{
				"subject_type": param.SubjectType,
				"subject_id":   param.SubjectID,
				"field":        param.Field,
				"status":       model.RefStatusActive,
				"expires_at":   nil,
				"owner_type":   param.OwnerType,
				"owner_id":     param.OwnerID,
				"updated_time": time.Now(),
			}
			result := tx.Model(&model.FileRef{}).
				Where("file_id = ? AND scene_code = ? AND status = ?", fileID, param.SceneCode, model.RefStatusTemp).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				continue
			}
			ref := model.FileRef{
				FileID:      fileID,
				SceneCode:   param.SceneCode,
				SubjectType: &param.SubjectType,
				SubjectID:   &param.SubjectID,
				Field:       &param.Field,
				Status:      model.RefStatusActive,
				OwnerType:   param.OwnerType,
				OwnerID:     param.OwnerID,
			}
			if err := tx.Create(&ref).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GORMRepository) UpdateRefsStatus(ctx context.Context, ids []int, status string) error {
	updates := map[string]any{
		"status":       status,
		"updated_time": time.Now(),
	}
	if status == model.RefStatusDeleted {
		updates["deleted_time"] = time.Now()
	}
	result := r.provider.Write().WithContext(ctx).Model(&model.FileRef{}).Where("id IN ?", ids).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMRepository) CreateShare(ctx context.Context, param CreateShareParam) (model.Share, error) {
	item := model.Share{
		FileID:       param.FileID,
		RefID:        param.RefID,
		Token:        param.Token,
		PasswordHash: param.PasswordHash,
		ExpiresAt:    param.ExpiresAt,
		MaxDownloads: param.MaxDownloads,
		Status:       param.Status,
		CreatedBy:    param.CreatedBy,
	}
	if err := r.provider.Write().WithContext(ctx).Create(&item).Error; err != nil {
		return model.Share{}, err
	}
	return item, nil
}

func (r *GORMRepository) GetShare(ctx context.Context, id int) (model.Share, error) {
	var item model.Share
	err := r.provider.Read().WithContext(ctx).First(&item, id).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) GetShareByToken(ctx context.Context, token string) (model.Share, error) {
	var item model.Share
	err := r.provider.Read().WithContext(ctx).Where("token = ?", token).First(&item).Error
	return item, mapGORMError(err)
}

func (r *GORMRepository) ListShares(ctx context.Context, filter ShareFilter, page int, size int) ([]model.Share, int64, error) {
	query := r.provider.Read().WithContext(ctx).Model(&model.Share{})
	if filter.FileID != nil {
		query = query.Where("file_id = ?", *filter.FileID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}
	return paginateGORM[model.Share](query.Order("id ASC"), page, size)
}

func (r *GORMRepository) DisableShare(ctx context.Context, id int) error {
	result := r.provider.Write().WithContext(ctx).Model(&model.Share{}).Where("id = ?", id).Updates(map[string]any{
		"status":       model.ShareStatusDisabled,
		"updated_time": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMRepository) IncrementShareDownload(ctx context.Context, id int) error {
	result := r.provider.Write().WithContext(ctx).Model(&model.Share{}).Where("id = ?", id).Updates(map[string]any{
		"download_count": gorm.Expr("download_count + ?", 1),
		"updated_time":   time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func paginateGORM[T any](query *gorm.DB, page int, size int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []T
	err := query.Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

func mapGORMError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
