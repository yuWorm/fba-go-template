package dto

import (
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

const TimeLayout = "2006-01-02 15:04:05"

type UploadResult struct {
	File FileDetail `json:"file"`
	Ref  RefDetail  `json:"ref"`
}

type FileDetail struct {
	ID           int     `json:"id"`
	UUID         string  `json:"uuid"`
	StorageCode  string  `json:"storage_code"`
	Provider     string  `json:"provider"`
	OriginalName string  `json:"original_name"`
	Ext          string  `json:"ext"`
	Mime         string  `json:"mime"`
	Size         int64   `json:"size"`
	SHA256       *string `json:"sha256"`
	ETag         *string `json:"etag"`
	Visibility   string  `json:"visibility"`
	Status       string  `json:"status"`
	URL          string  `json:"url"`
	UploadedBy   *int    `json:"uploaded_by"`
	CreatedTime  string  `json:"created_time"`
	UpdatedTime  *string `json:"updated_time"`
	ObjectKey    string  `json:"-"`
}

type RefDetail struct {
	ID          int        `json:"id"`
	FileID      int        `json:"file_id"`
	SceneCode   string     `json:"scene_code"`
	SubjectType *string    `json:"subject_type"`
	SubjectID   *string    `json:"subject_id"`
	Field       *string    `json:"field"`
	DisplayName *string    `json:"display_name"`
	Sort        int        `json:"sort"`
	Status      string     `json:"status"`
	ExpiresAt   *string    `json:"expires_at"`
	OwnerType   *string    `json:"owner_type"`
	OwnerID     *string    `json:"owner_id"`
	CreatedBy   *int       `json:"created_by"`
	Metadata    *string    `json:"metadata"`
	CreatedTime string     `json:"created_time"`
	UpdatedTime *string    `json:"updated_time"`
	File        FileDetail `json:"file"`
}

type SceneDetail struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	MaxSize            int64   `json:"max_size"`
	AllowedExts        *string `json:"allowed_exts"`
	AllowedMimes       *string `json:"allowed_mimes"`
	DefaultStorageCode *string `json:"default_storage_code"`
	DefaultVisibility  string  `json:"default_visibility"`
	TempTTLSeconds     int     `json:"temp_ttl_seconds"`
	Enabled            bool    `json:"enabled"`
}

type StorageDetail struct {
	Code      string  `json:"code"`
	Provider  string  `json:"provider"`
	Bucket    *string `json:"bucket"`
	Region    *string `json:"region"`
	Endpoint  *string `json:"endpoint"`
	BaseURL   *string `json:"base_url"`
	Prefix    string  `json:"prefix"`
	IsDefault bool    `json:"is_default"`
	Enabled   bool    `json:"enabled"`
}

type ShareDetail struct {
	ID            int     `json:"id"`
	FileID        int     `json:"file_id"`
	RefID         *int    `json:"ref_id"`
	Token         string  `json:"token"`
	ExpiresAt     *string `json:"expires_at"`
	MaxDownloads  *int    `json:"max_downloads"`
	DownloadCount int     `json:"download_count"`
	Status        string  `json:"status"`
	CreatedBy     *int    `json:"created_by"`
	CreatedTime   string  `json:"created_time"`
}

type DeleteParam struct {
	PKs []int `json:"pks"`
}

type BindParam struct {
	FileIDs     []int   `json:"file_ids"`
	SceneCode   string  `json:"scene_code"`
	SubjectType string  `json:"subject_type"`
	SubjectID   string  `json:"subject_id"`
	Field       string  `json:"field"`
	OwnerType   *string `json:"owner_type"`
	OwnerID     *string `json:"owner_id"`
}

type ShareCreateParam struct {
	FileID       int     `json:"file_id"`
	RefID        *int    `json:"ref_id"`
	Password     *string `json:"password"`
	ExpiresAt    *string `json:"expires_at"`
	MaxDownloads *int    `json:"max_downloads"`
}

type ShareVerifyParam struct {
	Password string `json:"password"`
}

func FileDetailFromModel(item model.FileObject, url string) FileDetail {
	return FileDetail{
		ID:           item.ID,
		UUID:         item.UUID,
		StorageCode:  item.StorageCode,
		Provider:     item.Provider,
		OriginalName: item.OriginalName,
		Ext:          item.Ext,
		Mime:         item.Mime,
		Size:         item.Size,
		SHA256:       item.SHA256,
		ETag:         item.ETag,
		Visibility:   item.Visibility,
		Status:       item.Status,
		URL:          url,
		UploadedBy:   item.UploadedBy,
		CreatedTime:  formatTime(item.CreatedTime),
		UpdatedTime:  formatTimePtr(item.UpdatedTime),
	}
}

func RefDetailFromModel(ref model.FileRef, file model.FileObject, url string) RefDetail {
	return RefDetail{
		ID:          ref.ID,
		FileID:      ref.FileID,
		SceneCode:   ref.SceneCode,
		SubjectType: ref.SubjectType,
		SubjectID:   ref.SubjectID,
		Field:       ref.Field,
		DisplayName: ref.DisplayName,
		Sort:        ref.Sort,
		Status:      ref.Status,
		ExpiresAt:   formatTimePtr(ref.ExpiresAt),
		OwnerType:   ref.OwnerType,
		OwnerID:     ref.OwnerID,
		CreatedBy:   ref.CreatedBy,
		Metadata:    ref.Metadata,
		CreatedTime: formatTime(ref.CreatedTime),
		UpdatedTime: formatTimePtr(ref.UpdatedTime),
		File:        FileDetailFromModel(file, url),
	}
}

func SceneDetailFromModel(item model.Scene) SceneDetail {
	return SceneDetail{
		Code:               item.Code,
		Name:               item.Name,
		MaxSize:            item.MaxSize,
		AllowedExts:        item.AllowedExts,
		AllowedMimes:       item.AllowedMimes,
		DefaultStorageCode: item.DefaultStorageCode,
		DefaultVisibility:  item.DefaultVisibility,
		TempTTLSeconds:     item.TempTTLSeconds,
		Enabled:            item.Enabled,
	}
}

func StorageDetailFromModel(item model.Storage) StorageDetail {
	return StorageDetail{
		Code:      item.Code,
		Provider:  item.Provider,
		Bucket:    item.Bucket,
		Region:    item.Region,
		Endpoint:  item.Endpoint,
		BaseURL:   item.BaseURL,
		Prefix:    item.Prefix,
		IsDefault: item.IsDefault,
		Enabled:   item.Enabled,
	}
}

func ShareDetailFromModel(item model.Share) ShareDetail {
	return ShareDetail{
		ID:            item.ID,
		FileID:        item.FileID,
		RefID:         item.RefID,
		Token:         item.Token,
		ExpiresAt:     formatTimePtr(item.ExpiresAt),
		MaxDownloads:  item.MaxDownloads,
		DownloadCount: item.DownloadCount,
		Status:        item.Status,
		CreatedBy:     item.CreatedBy,
		CreatedTime:   formatTime(item.CreatedTime),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(TimeLayout)
}

func formatTimePtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.Format(TimeLayout)
	return &formatted
}
