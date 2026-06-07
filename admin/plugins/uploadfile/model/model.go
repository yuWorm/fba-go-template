package model

import "time"

const (
	ProviderLocal = "local"
	ProviderS3    = "s3"
	ProviderOSS   = "oss"

	DefaultStorageCode = "local"

	DefaultSceneCode = "default"
	SceneAvatar      = "avatar"
	SceneAttachment  = "attachment"

	VisibilityPrivate = "private"
	VisibilityPublic  = "public"

	StatusPending = "pending"
	StatusActive  = "active"
	StatusDeleted = "deleted"

	RefStatusTemp    = "temp"
	RefStatusActive  = "active"
	RefStatusDeleted = "deleted"

	ShareStatusActive   = "active"
	ShareStatusDisabled = "disabled"
	ShareStatusExpired  = "expired"
)

type FileObject struct {
	ID           int        `gorm:"column:id;primaryKey"`
	UUID         string     `gorm:"column:uuid;size:64;uniqueIndex"`
	StorageCode  string     `gorm:"column:storage_code;size:64;uniqueIndex:uk_upload_file_object_storage_key"`
	Provider     string     `gorm:"column:provider;size:32;index"`
	Bucket       *string    `gorm:"column:bucket;size:128"`
	ObjectKey    string     `gorm:"column:object_key;size:512;uniqueIndex:uk_upload_file_object_storage_key"`
	OriginalName string     `gorm:"column:original_name;size:255"`
	Ext          string     `gorm:"column:ext;size:32;index"`
	Mime         string     `gorm:"column:mime;size:128;index"`
	Size         int64      `gorm:"column:size"`
	SHA256       *string    `gorm:"column:sha256;size:64;index"`
	ETag         *string    `gorm:"column:etag;size:128"`
	Visibility   string     `gorm:"column:visibility;size:32"`
	Status       string     `gorm:"column:status;size:32;index"`
	UploadedBy   *int       `gorm:"column:uploaded_by;index"`
	CreatedTime  time.Time  `gorm:"column:created_time;autoCreateTime;index"`
	UpdatedTime  *time.Time `gorm:"column:updated_time;autoUpdateTime"`
	DeletedTime  *time.Time `gorm:"column:deleted_time;index"`
}

func (FileObject) TableName() string {
	return "upload_file_object"
}

type FileRef struct {
	ID          int        `gorm:"column:id;primaryKey"`
	FileID      int        `gorm:"column:file_id;index"`
	SceneCode   string     `gorm:"column:scene_code;size:64;index:idx_upload_file_ref_subject"`
	SubjectType *string    `gorm:"column:subject_type;size:64;index:idx_upload_file_ref_subject"`
	SubjectID   *string    `gorm:"column:subject_id;size:64;index:idx_upload_file_ref_subject"`
	Field       *string    `gorm:"column:field;size:64;index:idx_upload_file_ref_subject"`
	DisplayName *string    `gorm:"column:display_name;size:255"`
	Sort        int        `gorm:"column:sort"`
	Status      string     `gorm:"column:status;size:32;index:idx_upload_file_ref_subject;index"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;index"`
	OwnerType   *string    `gorm:"column:owner_type;size:32;index:idx_upload_file_ref_owner"`
	OwnerID     *string    `gorm:"column:owner_id;size:64;index:idx_upload_file_ref_owner"`
	CreatedBy   *int       `gorm:"column:created_by;index"`
	Metadata    *string    `gorm:"column:metadata;type:json"`
	CreatedTime time.Time  `gorm:"column:created_time;autoCreateTime"`
	UpdatedTime *time.Time `gorm:"column:updated_time;autoUpdateTime"`
	DeletedTime *time.Time `gorm:"column:deleted_time;index"`
}

func (FileRef) TableName() string {
	return "upload_file_ref"
}

type Scene struct {
	ID                 int        `gorm:"column:id;primaryKey"`
	Code               string     `gorm:"column:code;size:64;uniqueIndex"`
	Name               string     `gorm:"column:name;size:128"`
	MaxSize            int64      `gorm:"column:max_size"`
	AllowedExts        *string    `gorm:"column:allowed_exts;type:json"`
	AllowedMimes       *string    `gorm:"column:allowed_mimes;type:json"`
	DefaultStorageCode *string    `gorm:"column:default_storage_code;size:64"`
	DefaultVisibility  string     `gorm:"column:default_visibility;size:32"`
	TempTTLSeconds     int        `gorm:"column:temp_ttl_seconds"`
	PathTemplate       string     `gorm:"column:path_template;size:255"`
	Enabled            bool       `gorm:"column:enabled;index"`
	CreatedTime        time.Time  `gorm:"column:created_time;autoCreateTime"`
	UpdatedTime        *time.Time `gorm:"column:updated_time;autoUpdateTime"`
}

func (Scene) TableName() string {
	return "upload_scene"
}

type Storage struct {
	ID          int        `gorm:"column:id;primaryKey"`
	Code        string     `gorm:"column:code;size:64;uniqueIndex"`
	Provider    string     `gorm:"column:provider;size:32;index"`
	Bucket      *string    `gorm:"column:bucket;size:128"`
	Region      *string    `gorm:"column:region;size:64"`
	Endpoint    *string    `gorm:"column:endpoint;size:255"`
	BaseURL     *string    `gorm:"column:base_url;size:255"`
	Prefix      string     `gorm:"column:prefix;size:128"`
	IsDefault   bool       `gorm:"column:is_default;index"`
	Enabled     bool       `gorm:"column:enabled;index"`
	Config      *string    `gorm:"column:config;type:json"`
	CreatedTime time.Time  `gorm:"column:created_time;autoCreateTime"`
	UpdatedTime *time.Time `gorm:"column:updated_time;autoUpdateTime"`
}

func (Storage) TableName() string {
	return "upload_storage"
}

type Share struct {
	ID            int        `gorm:"column:id;primaryKey"`
	FileID        int        `gorm:"column:file_id;index"`
	RefID         *int       `gorm:"column:ref_id;index"`
	Token         string     `gorm:"column:token;size:128;uniqueIndex"`
	PasswordHash  *string    `gorm:"column:password_hash;size:255"`
	ExpiresAt     *time.Time `gorm:"column:expires_at;index"`
	MaxDownloads  *int       `gorm:"column:max_downloads"`
	DownloadCount int        `gorm:"column:download_count"`
	Status        string     `gorm:"column:status;size:32;index"`
	CreatedBy     *int       `gorm:"column:created_by;index"`
	CreatedTime   time.Time  `gorm:"column:created_time;autoCreateTime"`
	UpdatedTime   *time.Time `gorm:"column:updated_time;autoUpdateTime"`
}

func (Share) TableName() string {
	return "upload_share"
}

func SeedStorages() []Storage {
	return []Storage{
		{
			ID:          1,
			Code:        DefaultStorageCode,
			Provider:    ProviderLocal,
			Prefix:      "uploads",
			IsDefault:   true,
			Enabled:     true,
			CreatedTime: seedTime(),
		},
	}
}

func SeedScenes() []Scene {
	local := DefaultStorageCode
	return []Scene{
		{
			ID:                 1,
			Code:               DefaultSceneCode,
			Name:               "Default",
			MaxSize:            20 * 1024 * 1024,
			AllowedExts:        strPtr(`["jpg","jpeg","png","gif","webp","pdf","txt","doc","docx","xls","xlsx","mp4","mov"]`),
			DefaultStorageCode: &local,
			DefaultVisibility:  VisibilityPrivate,
			TempTTLSeconds:     24 * 60 * 60,
			PathTemplate:       "{prefix}/{scene_code}/{yyyy}/{mm}/{dd}/{uuid}.{ext}",
			Enabled:            true,
			CreatedTime:        seedTime(),
		},
		{
			ID:                 2,
			Code:               SceneAvatar,
			Name:               "Avatar",
			MaxSize:            5 * 1024 * 1024,
			AllowedExts:        strPtr(`["jpg","jpeg","png","gif","webp"]`),
			AllowedMimes:       strPtr(`["image/jpeg","image/png","image/gif","image/webp"]`),
			DefaultStorageCode: &local,
			DefaultVisibility:  VisibilityPrivate,
			TempTTLSeconds:     24 * 60 * 60,
			PathTemplate:       "{prefix}/{scene_code}/{yyyy}/{mm}/{dd}/{uuid}.{ext}",
			Enabled:            true,
			CreatedTime:        seedTime(),
		},
		{
			ID:                 3,
			Code:               SceneAttachment,
			Name:               "Attachment",
			MaxSize:            50 * 1024 * 1024,
			AllowedExts:        strPtr(`["jpg","jpeg","png","gif","webp","pdf","txt","doc","docx","xls","xlsx","mp4","mov"]`),
			DefaultStorageCode: &local,
			DefaultVisibility:  VisibilityPrivate,
			TempTTLSeconds:     24 * 60 * 60,
			PathTemplate:       "{prefix}/{scene_code}/{yyyy}/{mm}/{dd}/{uuid}.{ext}",
			Enabled:            true,
			CreatedTime:        seedTime(),
		},
	}
}

func seedTime() time.Time {
	return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
}

func strPtr(value string) *string {
	return &value
}
