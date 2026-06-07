package repo

import (
	"context"
	"errors"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

var ErrNotFound = errors.New("not found")

type ObjectFilter struct {
	Keyword     string
	SceneCode   string
	Provider    string
	StorageCode string
	Status      string
	UploadedBy  *int
	OwnerType   string
	OwnerID     string
}

type RefFilter struct {
	FileID      *int
	SceneCode   string
	SubjectType string
	SubjectID   string
	Field       string
	Status      string
	OwnerType   string
	OwnerID     string
}

type ShareFilter struct {
	FileID    *int
	Status    string
	CreatedBy *int
}

type CreateObjectParam struct {
	UUID         string
	StorageCode  string
	Provider     string
	Bucket       *string
	ObjectKey    string
	OriginalName string
	Ext          string
	Mime         string
	Size         int64
	SHA256       *string
	ETag         *string
	Visibility   string
	Status       string
	UploadedBy   *int
}

type CreateRefParam struct {
	FileID      int
	SceneCode   string
	SubjectType *string
	SubjectID   *string
	Field       *string
	DisplayName *string
	Sort        int
	Status      string
	ExpiresAt   *time.Time
	OwnerType   *string
	OwnerID     *string
	CreatedBy   *int
	Metadata    *string
}

type BindRefsParam struct {
	FileIDs     []int
	SceneCode   string
	SubjectType string
	SubjectID   string
	Field       string
	OwnerType   *string
	OwnerID     *string
}

type CreateShareParam struct {
	FileID       int
	RefID        *int
	Token        string
	PasswordHash *string
	ExpiresAt    *time.Time
	MaxDownloads *int
	Status       string
	CreatedBy    *int
}

type Repository interface {
	GetScene(ctx context.Context, code string) (model.Scene, error)
	ListScenes(ctx context.Context) ([]model.Scene, error)
	GetStorage(ctx context.Context, code string) (model.Storage, error)
	GetDefaultStorage(ctx context.Context) (model.Storage, error)
	ListStorages(ctx context.Context) ([]model.Storage, error)
	CreateObject(ctx context.Context, param CreateObjectParam) (model.FileObject, error)
	GetObject(ctx context.Context, id int) (model.FileObject, error)
	GetObjectByUUID(ctx context.Context, uuid string) (model.FileObject, error)
	ListObjects(ctx context.Context, filter ObjectFilter, page int, size int) ([]model.FileObject, int64, error)
	UpdateObjectStatus(ctx context.Context, id int, status string) error
	CreateRef(ctx context.Context, param CreateRefParam) (model.FileRef, error)
	ListRefs(ctx context.Context, filter RefFilter, page int, size int) ([]model.FileRef, int64, error)
	BindRefs(ctx context.Context, param BindRefsParam) error
	UpdateRefsStatus(ctx context.Context, ids []int, status string) error
	CreateShare(ctx context.Context, param CreateShareParam) (model.Share, error)
	GetShare(ctx context.Context, id int) (model.Share, error)
	GetShareByToken(ctx context.Context, token string) (model.Share, error)
	ListShares(ctx context.Context, filter ShareFilter, page int, size int) ([]model.Share, int64, error)
	DisableShare(ctx context.Context, id int) error
	IncrementShareDownload(ctx context.Context, id int) error
}

type Seed struct {
	Storages []model.Storage
	Scenes   []model.Scene
}

func SeedData() Seed {
	return Seed{Storages: model.SeedStorages(), Scenes: model.SeedScenes()}
}
