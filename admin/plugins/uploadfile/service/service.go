package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/dto"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
	"github.com/yuWorm/fba-go/core/pagination"
)

type Clock func() time.Time

const defaultFileAccessTokenMaxTTL = time.Hour

type Options struct {
	TokenSecret            []byte
	Now                    Clock
	DownloadTokenTTL       time.Duration
	FileAccessTokenMaxTTL  time.Duration
	DirectUploadPresignTTL time.Duration
	MaxTotalBytes          int64
	MaxOwnerBytes          int64
	MaxTotalFiles          int64
	MaxOwnerFiles          int64
}

type Service struct {
	repo              repo.Repository
	storage           *storage.Registry
	secret            []byte
	now               Clock
	tokenTTL          time.Duration
	accessTokenMaxTTL time.Duration
	directUploadTTL   time.Duration
	maxTotalBytes     int64
	maxOwnerBytes     int64
	maxTotalFiles     int64
	maxOwnerFiles     int64
}

type UploadInput struct {
	Filename    string
	ContentType string
	Size        int64
	Reader      io.Reader
	SceneCode   string
	Field       string
	SubjectType string
	SubjectID   string
	OwnerType   *string
	OwnerID     *string
	Temp        *bool
	Actor       Actor
}

type PresignUploadInput struct {
	Filename    string
	ContentType string
	Size        int64
	SceneCode   string
	Field       string
	SubjectType string
	SubjectID   string
	OwnerType   *string
	OwnerID     *string
	Temp        *bool
	TTL         time.Duration
	Actor       Actor
}

type BindInput struct {
	FileIDs     []int
	SceneCode   string
	SubjectType string
	SubjectID   string
	Field       string
	OwnerType   *string
	OwnerID     *string
	Actor       Actor
}

type ShareInput struct {
	FileID       int
	RefID        *int
	Password     *string
	ExpiresAt    *time.Time
	MaxDownloads *int
	Actor        Actor
}

type CleanupOptions struct {
	DryRun     bool
	PendingTTL time.Duration
}

type FileAccessTokenInput struct {
	TTL   time.Duration
	Actor Actor
}

func New(repository repo.Repository, registry *storage.Registry, opts Options) *Service {
	if repository == nil {
		repository = repo.NewMemoryRepository(repo.SeedData())
	}
	if registry == nil {
		registry = storage.NewRegistry()
		registry.Add(model.DefaultStorageCode, storage.NewLocal(storage.LocalOptions{}))
	}
	if len(opts.TokenSecret) == 0 {
		opts.TokenSecret = []byte("uploadfile-local-development-secret")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.DownloadTokenTTL <= 0 {
		opts.DownloadTokenTTL = 10 * time.Minute
	}
	if opts.FileAccessTokenMaxTTL <= 0 {
		opts.FileAccessTokenMaxTTL = defaultFileAccessTokenMaxTTL
	}
	if opts.DirectUploadPresignTTL <= 0 {
		opts.DirectUploadPresignTTL = 15 * time.Minute
	}
	return &Service{
		repo:              repository,
		storage:           registry,
		secret:            opts.TokenSecret,
		now:               opts.Now,
		tokenTTL:          opts.DownloadTokenTTL,
		accessTokenMaxTTL: opts.FileAccessTokenMaxTTL,
		directUploadTTL:   opts.DirectUploadPresignTTL,
		maxTotalBytes:     opts.MaxTotalBytes,
		maxOwnerBytes:     opts.MaxOwnerBytes,
		maxTotalFiles:     opts.MaxTotalFiles,
		maxOwnerFiles:     opts.MaxOwnerFiles,
	}
}

func resolveUploadOwner(actor Actor, inputOwnerType *string, inputOwnerID *string) (*string, *string, error) {
	ownerType := cleanOptional(inputOwnerType)
	ownerID := cleanOptional(inputOwnerID)
	if ownerType == nil && ownerID == nil {
		ownerType, ownerID = actor.defaultOwner()
	}
	if !actor.allowsOwner(ownerType, ownerID) {
		return nil, nil, forbidden("file owner is not allowed")
	}
	return ownerType, ownerID, nil
}

func (s *Service) enforceUploadQuota(ctx context.Context, size int64, ownerType *string, ownerID *string) error {
	if size < 0 {
		return badRequest("file size is invalid", nil)
	}
	if s.maxTotalBytes > 0 || s.maxTotalFiles > 0 {
		usage, err := s.repo.UploadUsage(ctx, repo.UsageFilter{})
		if err != nil {
			return err
		}
		if exceedsByteQuota(usage.Bytes, size, s.maxTotalBytes) {
			return badRequest("upload total byte quota exceeded", nil)
		}
		if exceedsFileQuota(usage.Files, s.maxTotalFiles) {
			return badRequest("upload total file quota exceeded", nil)
		}
	}
	if (s.maxOwnerBytes > 0 || s.maxOwnerFiles > 0) && ownerValue(ownerType) != "" && ownerValue(ownerID) != "" {
		usage, err := s.repo.UploadUsage(ctx, repo.UsageFilter{
			OwnerType: ownerValue(ownerType),
			OwnerID:   ownerValue(ownerID),
		})
		if err != nil {
			return err
		}
		if exceedsByteQuota(usage.Bytes, size, s.maxOwnerBytes) {
			return badRequest("upload owner byte quota exceeded", nil)
		}
		if exceedsFileQuota(usage.Files, s.maxOwnerFiles) {
			return badRequest("upload owner file quota exceeded", nil)
		}
	}
	return nil
}

func exceedsByteQuota(current int64, incoming int64, limit int64) bool {
	if limit <= 0 {
		return false
	}
	return current > limit || incoming > limit-current
}

func exceedsFileQuota(current int64, limit int64) bool {
	if limit <= 0 {
		return false
	}
	return current+1 > limit
}

func (s *Service) Upload(ctx context.Context, input UploadInput) (dto.UploadResult, error) {
	if input.Reader == nil {
		return dto.UploadResult{}, badRequest("file is required", nil)
	}
	sceneCode := strings.TrimSpace(input.SceneCode)
	if sceneCode == "" {
		sceneCode = model.DefaultSceneCode
	}
	scene, err := s.repo.GetScene(ctx, sceneCode)
	if err != nil {
		return dto.UploadResult{}, notFound("upload scene not found", err)
	}
	if !scene.Enabled {
		return dto.UploadResult{}, badRequest("upload scene disabled", nil)
	}
	if input.Size > scene.MaxSize {
		return dto.UploadResult{}, badRequest("file size exceeds scene limit", nil)
	}
	name := sanitizeFilename(input.Filename)
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if ext == "" {
		return dto.UploadResult{}, badRequest("file extension is required", nil)
	}
	if !allowed(ext, scene.AllowedExts) {
		return dto.UploadResult{}, badRequest("file extension is not allowed", nil)
	}
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType([]byte{})
	}
	if scene.AllowedMimes != nil && !allowed(contentType, scene.AllowedMimes) {
		return dto.UploadResult{}, badRequest("file MIME type is not allowed", nil)
	}
	ownerType, ownerID, err := resolveUploadOwner(input.Actor, input.OwnerType, input.OwnerID)
	if err != nil {
		return dto.UploadResult{}, err
	}
	if err := s.enforceUploadQuota(ctx, input.Size, ownerType, ownerID); err != nil {
		return dto.UploadResult{}, err
	}

	storageConfig, backend, err := s.backend(ctx, scene)
	if err != nil {
		return dto.UploadResult{}, err
	}
	uuid := randomHex(16)
	key := objectKey(storageConfig.Prefix, scene.Code, uuid, ext, s.now())
	hash := sha256.New()
	reader := io.TeeReader(input.Reader, hash)
	info, err := backend.Put(ctx, key, reader, storage.PutOptions{ContentType: contentType})
	if err != nil {
		return dto.UploadResult{}, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	object, err := s.repo.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         uuid,
		StorageCode:  storageConfig.Code,
		Provider:     storageConfig.Provider,
		Bucket:       storageConfig.Bucket,
		ObjectKey:    info.Key,
		OriginalName: name,
		Ext:          ext,
		Mime:         contentType,
		Size:         info.Size,
		SHA256:       &sum,
		ETag:         info.ETag,
		Visibility:   defaultString(scene.DefaultVisibility, model.VisibilityPrivate),
		Status:       model.StatusActive,
		UploadedBy:   input.Actor.UserID,
	})
	if err != nil {
		return dto.UploadResult{}, err
	}

	temp := input.SubjectType == "" || input.SubjectID == ""
	if input.Temp != nil {
		temp = *input.Temp
	}
	refStatus := model.RefStatusActive
	var expiresAt *time.Time
	if temp {
		refStatus = model.RefStatusTemp
		expire := s.now().Add(time.Duration(scene.TempTTLSeconds) * time.Second)
		expiresAt = &expire
	}
	ref, err := s.repo.CreateRef(ctx, repo.CreateRefParam{
		FileID:      object.ID,
		SceneCode:   scene.Code,
		SubjectType: optionalString(input.SubjectType),
		SubjectID:   optionalString(input.SubjectID),
		Field:       optionalString(input.Field),
		Status:      refStatus,
		ExpiresAt:   expiresAt,
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		CreatedBy:   input.Actor.UserID,
	})
	if err != nil {
		return dto.UploadResult{}, err
	}
	return dto.UploadResult{File: dto.FileDetailFromModel(object, s.fileURL(object)), Ref: dto.RefDetailFromModel(ref, object, s.fileURL(object))}, nil
}

func (s *Service) CreatePresignedUpload(ctx context.Context, input PresignUploadInput) (dto.PresignedUploadResult, error) {
	sceneCode := strings.TrimSpace(input.SceneCode)
	if sceneCode == "" {
		sceneCode = model.DefaultSceneCode
	}
	scene, err := s.repo.GetScene(ctx, sceneCode)
	if err != nil {
		return dto.PresignedUploadResult{}, notFound("upload scene not found", err)
	}
	if !scene.Enabled {
		return dto.PresignedUploadResult{}, badRequest("upload scene disabled", nil)
	}
	if input.Size > scene.MaxSize {
		return dto.PresignedUploadResult{}, badRequest("file size exceeds scene limit", nil)
	}
	name := sanitizeFilename(input.Filename)
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if ext == "" {
		return dto.PresignedUploadResult{}, badRequest("file extension is required", nil)
	}
	if !allowed(ext, scene.AllowedExts) {
		return dto.PresignedUploadResult{}, badRequest("file extension is not allowed", nil)
	}
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType([]byte{})
	}
	if scene.AllowedMimes != nil && !allowed(contentType, scene.AllowedMimes) {
		return dto.PresignedUploadResult{}, badRequest("file MIME type is not allowed", nil)
	}
	ownerType, ownerID, err := resolveUploadOwner(input.Actor, input.OwnerType, input.OwnerID)
	if err != nil {
		return dto.PresignedUploadResult{}, err
	}
	if err := s.enforceUploadQuota(ctx, input.Size, ownerType, ownerID); err != nil {
		return dto.PresignedUploadResult{}, err
	}

	storageConfig, backend, err := s.backend(ctx, scene)
	if err != nil {
		return dto.PresignedUploadResult{}, err
	}
	uuid := randomHex(16)
	key := objectKey(storageConfig.Prefix, scene.Code, uuid, ext, s.now())
	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.directUploadTTL
	}
	presigned, err := backend.PresignPut(ctx, key, ttl, storage.PutOptions{ContentType: contentType})
	if err != nil {
		return dto.PresignedUploadResult{}, err
	}
	object, err := s.repo.CreateObject(ctx, repo.CreateObjectParam{
		UUID:         uuid,
		StorageCode:  storageConfig.Code,
		Provider:     storageConfig.Provider,
		Bucket:       storageConfig.Bucket,
		ObjectKey:    key,
		OriginalName: name,
		Ext:          ext,
		Mime:         contentType,
		Size:         input.Size,
		Visibility:   defaultString(scene.DefaultVisibility, model.VisibilityPrivate),
		Status:       model.StatusPending,
		UploadedBy:   input.Actor.UserID,
	})
	if err != nil {
		return dto.PresignedUploadResult{}, err
	}

	temp := input.SubjectType == "" || input.SubjectID == ""
	if input.Temp != nil {
		temp = *input.Temp
	}
	refStatus := model.RefStatusActive
	var expiresAt *time.Time
	if temp {
		refStatus = model.RefStatusTemp
		expire := s.now().Add(time.Duration(scene.TempTTLSeconds) * time.Second)
		expiresAt = &expire
	}
	ref, err := s.repo.CreateRef(ctx, repo.CreateRefParam{
		FileID:      object.ID,
		SceneCode:   scene.Code,
		SubjectType: optionalString(input.SubjectType),
		SubjectID:   optionalString(input.SubjectID),
		Field:       optionalString(input.Field),
		Status:      refStatus,
		ExpiresAt:   expiresAt,
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		CreatedBy:   input.Actor.UserID,
	})
	if err != nil {
		return dto.PresignedUploadResult{}, err
	}
	fileURL := s.fileURL(object)
	return dto.PresignedUploadResult{
		File: dto.FileDetailFromModel(object, fileURL),
		Ref:  dto.RefDetailFromModel(ref, object, fileURL),
		Presigned: dto.PresignedUploadURLFromStorage(
			presigned.Method,
			presigned.URL,
			presigned.ExpiresAt,
			presigned.Headers,
		),
	}, nil
}

func (s *Service) CompletePresignedUpload(ctx context.Context, id int, actor Actor) (dto.FileDetail, error) {
	object, _, err := s.ensureObjectAccess(ctx, id, actor)
	if err != nil {
		return dto.FileDetail{}, err
	}
	if object.Status == model.StatusDeleted {
		return dto.FileDetail{}, badRequest("file is deleted", nil)
	}
	if object.Status != model.StatusActive {
		if err := s.validatePresignedUploadObject(ctx, object); err != nil {
			return dto.FileDetail{}, err
		}
		if err := s.repo.UpdateObjectStatus(ctx, id, model.StatusActive); err != nil {
			return dto.FileDetail{}, err
		}
		object.Status = model.StatusActive
	}
	return dto.FileDetailFromModel(object, s.fileURL(object)), nil
}

func (s *Service) Bind(ctx context.Context, input BindInput) error {
	sceneCode := strings.TrimSpace(input.SceneCode)
	if sceneCode == "" {
		sceneCode = model.DefaultSceneCode
	}
	if len(input.FileIDs) == 0 {
		return badRequest("file_ids is required", nil)
	}
	if strings.TrimSpace(input.SubjectType) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return badRequest("subject is required", nil)
	}
	for _, fileID := range input.FileIDs {
		if _, _, err := s.ensureObjectAccess(ctx, fileID, input.Actor); err != nil {
			return err
		}
	}
	ownerType := cleanOptional(input.OwnerType)
	ownerID := cleanOptional(input.OwnerID)
	if ownerType == nil && ownerID == nil {
		ownerType, ownerID = input.Actor.defaultOwner()
	}
	if !input.Actor.allowsOwner(ownerType, ownerID) {
		return forbidden("file owner is not allowed")
	}
	return s.repo.BindRefs(ctx, repo.BindRefsParam{
		FileIDs:     input.FileIDs,
		SceneCode:   sceneCode,
		SubjectType: strings.TrimSpace(input.SubjectType),
		SubjectID:   strings.TrimSpace(input.SubjectID),
		Field:       strings.TrimSpace(input.Field),
		OwnerType:   ownerType,
		OwnerID:     ownerID,
	})
}

func (s *Service) ListRefs(ctx context.Context, filter repo.RefFilter, page int, size int, actor Actor) (pagination.PageData[dto.RefDetail], error) {
	filter = scopeRefFilter(filter, actor)
	items, total, err := s.repo.ListRefs(ctx, filter, page, size)
	if err != nil {
		return pagination.PageData[dto.RefDetail]{}, err
	}
	result := make([]dto.RefDetail, 0, len(items))
	for _, item := range items {
		object, err := s.repo.GetObject(ctx, item.FileID)
		if err != nil {
			return pagination.PageData[dto.RefDetail]{}, err
		}
		result = append(result, dto.RefDetailFromModel(item, object, s.fileURL(object)))
	}
	return pagination.NewPageData(result, total, page, size, "/api/v1/upload/refs"), nil
}

func (s *Service) ListFiles(ctx context.Context, filter repo.ObjectFilter, page int, size int, actor Actor) (pagination.PageData[dto.FileDetail], error) {
	filter = scopeObjectFilter(filter, actor)
	items, total, err := s.repo.ListObjects(ctx, filter, page, size)
	if err != nil {
		return pagination.PageData[dto.FileDetail]{}, err
	}
	result := make([]dto.FileDetail, 0, len(items))
	for _, item := range items {
		result = append(result, dto.FileDetailFromModel(item, s.fileURL(item)))
	}
	return pagination.NewPageData(result, total, page, size, "/api/v1/upload/files"), nil
}

func (s *Service) UploadStats(ctx context.Context, filter repo.UsageFilter, actor Actor) (dto.UploadStats, error) {
	filter = scopeUsageFilter(filter, actor)
	stats, err := s.repo.UploadUsage(ctx, filter)
	if err != nil {
		return dto.UploadStats{}, err
	}
	return dto.UploadStats{Files: stats.Files, Bytes: stats.Bytes}, nil
}

func (s *Service) GetFile(ctx context.Context, id int, actor Actor) (dto.FileDetail, error) {
	object, _, err := s.ensureObjectAccess(ctx, id, actor)
	if err != nil {
		return dto.FileDetail{}, err
	}
	return dto.FileDetailFromModel(object, s.fileURL(object)), nil
}

func (s *Service) OpenFile(ctx context.Context, id int, actor Actor) (io.ReadCloser, dto.FileDetail, error) {
	object, _, err := s.ensureObjectAccess(ctx, id, actor)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	if object.Status == model.StatusDeleted {
		return nil, dto.FileDetail{}, badRequest("file is deleted", nil)
	}
	backend, err := s.backendForObject(ctx, object)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	reader, _, err := backend.Open(ctx, object.ObjectKey)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	return reader, dto.FileDetailFromModel(object, s.fileURL(object)), nil
}

func (s *Service) CreateFileAccessToken(ctx context.Context, id int, input FileAccessTokenInput) (dto.FileAccessToken, error) {
	object, _, err := s.ensureObjectAccess(ctx, id, input.Actor)
	if err != nil {
		return dto.FileAccessToken{}, err
	}
	if object.Status == model.StatusDeleted {
		return dto.FileAccessToken{}, badRequest("file is deleted", nil)
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.tokenTTL
		if ttl > s.accessTokenMaxTTL {
			ttl = s.accessTokenMaxTTL
		}
	}
	if input.TTL > s.accessTokenMaxTTL {
		return dto.FileAccessToken{}, badRequest("file access token ttl exceeds limit", nil)
	}
	expiresAt := s.now().Add(ttl)
	token := signFileAccessToken(s.secret, object.UUID, expiresAt)
	downloadURL := s.fileURL(object) + "?download_token=" + url.QueryEscape(token)
	return dto.FileAccessToken{
		DownloadURL:   downloadURL,
		DownloadToken: token,
		ExpiresAt:     expiresAt.Format(dto.TimeLayout),
	}, nil
}

func (s *Service) ListScenes(ctx context.Context) ([]dto.SceneDetail, error) {
	items, err := s.repo.ListScenes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SceneDetail, 0, len(items))
	for _, item := range items {
		result = append(result, dto.SceneDetailFromModel(item))
	}
	return result, nil
}

func (s *Service) CreateScene(ctx context.Context, param dto.SceneParam) (dto.SceneDetail, error) {
	save, err := s.sceneParamForCreate(ctx, param)
	if err != nil {
		return dto.SceneDetail{}, err
	}
	item, err := s.repo.CreateScene(ctx, save)
	if err != nil {
		return dto.SceneDetail{}, err
	}
	return dto.SceneDetailFromModel(item), nil
}

func (s *Service) UpdateScene(ctx context.Context, code string, param dto.SceneParam) (dto.SceneDetail, error) {
	current, err := s.repo.GetScene(ctx, strings.TrimSpace(code))
	if err != nil {
		return dto.SceneDetail{}, notFound("upload scene not found", err)
	}
	save, err := s.sceneParamForUpdate(ctx, current, param)
	if err != nil {
		return dto.SceneDetail{}, err
	}
	item, err := s.repo.UpdateScene(ctx, current.Code, save)
	if err != nil {
		return dto.SceneDetail{}, err
	}
	return dto.SceneDetailFromModel(item), nil
}

func (s *Service) DeleteScene(ctx context.Context, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return badRequest("scene code is required", nil)
	}
	switch code {
	case model.DefaultSceneCode, model.SceneAvatar, model.SceneAttachment:
		return badRequest("seed upload scene cannot be deleted", nil)
	}
	refCount, err := s.repo.CountRefsByScene(ctx, code)
	if err != nil {
		return err
	}
	if refCount > 0 {
		return badRequest("upload scene is in use", nil)
	}
	return s.repo.DeleteScene(ctx, code)
}

func (s *Service) ListStorages(ctx context.Context) ([]dto.StorageDetail, error) {
	items, err := s.repo.ListStorages(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.StorageDetail, 0, len(items))
	for _, item := range items {
		result = append(result, dto.StorageDetailFromModel(item))
	}
	return result, nil
}

func (s *Service) CreateStorage(ctx context.Context, param dto.StorageParam) (dto.StorageDetail, error) {
	save, err := storageParamForCreate(param)
	if err != nil {
		return dto.StorageDetail{}, err
	}
	item, err := s.repo.CreateStorage(ctx, save)
	if err != nil {
		return dto.StorageDetail{}, err
	}
	return dto.StorageDetailFromModel(item), nil
}

func (s *Service) UpdateStorage(ctx context.Context, code string, param dto.StorageParam) (dto.StorageDetail, error) {
	current, err := s.repo.GetStorage(ctx, strings.TrimSpace(code))
	if err != nil {
		return dto.StorageDetail{}, notFound("storage not found", err)
	}
	save, err := storageParamForUpdate(current, param)
	if err != nil {
		return dto.StorageDetail{}, err
	}
	item, err := s.repo.UpdateStorage(ctx, current.Code, save)
	if err != nil {
		return dto.StorageDetail{}, err
	}
	return dto.StorageDetailFromModel(item), nil
}

func (s *Service) DeleteStorage(ctx context.Context, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return badRequest("storage code is required", nil)
	}
	if code == model.DefaultStorageCode {
		return badRequest("default local storage cannot be deleted", nil)
	}
	objectCount, err := s.repo.CountObjectsByStorage(ctx, code)
	if err != nil {
		return err
	}
	if objectCount > 0 {
		return badRequest("storage is in use", nil)
	}
	sceneCount, err := s.repo.CountScenesByStorage(ctx, code)
	if err != nil {
		return err
	}
	if sceneCount > 0 {
		return badRequest("storage is used by upload scenes", nil)
	}
	return s.repo.DeleteStorage(ctx, code)
}

func (s *Service) DeleteFiles(ctx context.Context, ids []int, actor Actor) error {
	if len(ids) == 0 {
		return badRequest("pks is required", nil)
	}
	for _, id := range ids {
		if _, _, err := s.ensureObjectAccess(ctx, id, actor); err != nil {
			return err
		}
		if err := s.repo.UpdateObjectStatus(ctx, id, model.StatusDeleted); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CleanupExpiredTemps(ctx context.Context, opts CleanupOptions) (dto.CleanupResult, error) {
	expired, err := s.repo.ListExpiredTempRefs(ctx, s.now())
	if err != nil {
		return dto.CleanupResult{}, err
	}
	result := dto.CleanupResult{ExpiredRefs: len(expired)}
	deletedFileIDs := map[int]bool{}
	if len(expired) > 0 {
		refIDs := make([]int, 0, len(expired))
		expiredRefIDsByFile := make(map[int]map[int]bool, len(expired))
		for _, ref := range expired {
			refIDs = append(refIDs, ref.ID)
			if expiredRefIDsByFile[ref.FileID] == nil {
				expiredRefIDsByFile[ref.FileID] = map[int]bool{}
			}
			expiredRefIDsByFile[ref.FileID][ref.ID] = true
		}
		if !opts.DryRun && len(refIDs) > 0 {
			if err := s.repo.UpdateRefsStatus(ctx, refIDs, model.RefStatusDeleted); err != nil {
				return dto.CleanupResult{}, err
			}
		}
		for fileID, expiredRefIDs := range expiredRefIDsByFile {
			remaining, err := s.countRemainingLiveRefsAfterCleanup(ctx, fileID, expiredRefIDs, opts.DryRun)
			if err != nil {
				return dto.CleanupResult{}, err
			}
			if remaining > 0 {
				continue
			}
			object, err := s.repo.GetObject(ctx, fileID)
			if err != nil {
				return dto.CleanupResult{}, err
			}
			if object.Status == model.StatusDeleted {
				continue
			}
			if opts.DryRun {
				result.DeletedFiles++
				deletedFileIDs[object.ID] = true
				continue
			}
			backend, err := s.backendForObject(ctx, object)
			if err != nil {
				return dto.CleanupResult{}, err
			}
			if err := backend.Delete(ctx, object.ObjectKey); err != nil {
				return dto.CleanupResult{}, err
			}
			if err := s.repo.UpdateObjectStatus(ctx, object.ID, model.StatusDeleted); err != nil {
				return dto.CleanupResult{}, err
			}
			result.DeletedFiles++
			deletedFileIDs[object.ID] = true
		}
	}
	if err := s.cleanupPendingUploads(ctx, opts, deletedFileIDs, &result); err != nil {
		return dto.CleanupResult{}, err
	}
	return result, nil
}

func (s *Service) validatePresignedUploadObject(ctx context.Context, object model.FileObject) error {
	backend, err := s.backendForObject(ctx, object)
	if err != nil {
		return err
	}
	info, err := backend.Head(ctx, object.ObjectKey)
	if err != nil {
		return badRequest("uploaded object is not available", err)
	}
	if info.Size != object.Size {
		return badRequest("uploaded object size mismatch", nil)
	}
	if strings.TrimSpace(info.ContentType) != "" && strings.TrimSpace(object.Mime) != "" && strings.TrimSpace(info.ContentType) != strings.TrimSpace(object.Mime) {
		return badRequest("uploaded object MIME type mismatch", nil)
	}
	return nil
}

func (s *Service) cleanupPendingUploads(ctx context.Context, opts CleanupOptions, deletedFileIDs map[int]bool, result *dto.CleanupResult) error {
	pendingTTL := opts.PendingTTL
	if pendingTTL <= 0 {
		pendingTTL = s.directUploadTTL
	}
	pending, err := s.repo.ListPendingObjectsBefore(ctx, s.now().Add(-pendingTTL))
	if err != nil {
		return err
	}
	for _, object := range pending {
		if object.Status == model.StatusDeleted {
			continue
		}
		result.PendingFiles++
		if !deletedFileIDs[object.ID] {
			result.DeletedFiles++
			deletedFileIDs[object.ID] = true
		}
		if opts.DryRun {
			continue
		}
		backend, err := s.backendForObject(ctx, object)
		if err != nil {
			return err
		}
		if err := backend.Delete(ctx, object.ObjectKey); err != nil {
			return err
		}
		refs, err := s.refsForFile(ctx, object.ID)
		if err != nil {
			return err
		}
		refIDs := make([]int, 0, len(refs))
		for _, ref := range refs {
			if ref.Status != model.RefStatusDeleted {
				refIDs = append(refIDs, ref.ID)
			}
		}
		if len(refIDs) > 0 {
			if err := s.repo.UpdateRefsStatus(ctx, refIDs, model.RefStatusDeleted); err != nil {
				return err
			}
		}
		if err := s.repo.UpdateObjectStatus(ctx, object.ID, model.StatusDeleted); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) countRemainingLiveRefsAfterCleanup(ctx context.Context, fileID int, expiredRefIDs map[int]bool, dryRun bool) (int64, error) {
	if !dryRun {
		return s.repo.CountRefsByFileStatus(ctx, fileID, []string{model.RefStatusTemp, model.RefStatusActive})
	}
	refs, err := s.refsForFile(ctx, fileID)
	if err != nil {
		return 0, err
	}
	var remaining int64
	for _, ref := range refs {
		if expiredRefIDs[ref.ID] {
			continue
		}
		if ref.Status == model.RefStatusTemp || ref.Status == model.RefStatusActive {
			remaining++
		}
	}
	return remaining, nil
}

func (s *Service) OpenPublicFile(ctx context.Context, uuid string, downloadToken string) (io.ReadCloser, dto.FileDetail, error) {
	object, err := s.repo.GetObjectByUUID(ctx, strings.TrimSpace(uuid))
	if err != nil {
		return nil, dto.FileDetail{}, notFound("file not found", err)
	}
	if object.Status == model.StatusDeleted {
		return nil, dto.FileDetail{}, badRequest("file is deleted", nil)
	}
	if object.Visibility != model.VisibilityPublic && !verifyFileAccessToken(s.secret, downloadToken, object.UUID, s.now()) {
		return nil, dto.FileDetail{}, forbidden("file is not public")
	}
	backend, err := s.backendForObject(ctx, object)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	reader, _, err := backend.Open(ctx, object.ObjectKey)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	return reader, dto.FileDetailFromModel(object, s.fileURL(object)), nil
}

func (s *Service) CreateShare(ctx context.Context, input ShareInput) (dto.ShareDetail, error) {
	if input.FileID <= 0 {
		return dto.ShareDetail{}, badRequest("file_id is required", nil)
	}
	if _, _, err := s.ensureObjectAccess(ctx, input.FileID, input.Actor); err != nil {
		return dto.ShareDetail{}, err
	}
	var passwordHash *string
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" {
		hash := hashPassword(*input.Password)
		passwordHash = &hash
	}
	share, err := s.repo.CreateShare(ctx, repo.CreateShareParam{
		FileID:       input.FileID,
		RefID:        input.RefID,
		Token:        randomHex(18),
		PasswordHash: passwordHash,
		ExpiresAt:    input.ExpiresAt,
		MaxDownloads: input.MaxDownloads,
		Status:       model.ShareStatusActive,
		CreatedBy:    input.Actor.UserID,
	})
	if err != nil {
		return dto.ShareDetail{}, err
	}
	return dto.ShareDetailFromModel(share), nil
}

func (s *Service) ListShares(ctx context.Context, filter repo.ShareFilter, page int, size int, actor Actor) (pagination.PageData[dto.ShareDetail], error) {
	if !actor.IsSuperAdmin && actor.UserID != nil {
		filter.CreatedBy = actor.UserID
	}
	items, total, err := s.repo.ListShares(ctx, filter, page, size)
	if err != nil {
		return pagination.PageData[dto.ShareDetail]{}, err
	}
	result := make([]dto.ShareDetail, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ShareDetailFromModel(item))
	}
	return pagination.NewPageData(result, total, page, size, "/api/v1/upload/shares"), nil
}

func (s *Service) DisableShare(ctx context.Context, id int, actor Actor) error {
	share, err := s.repo.GetShare(ctx, id)
	if err != nil {
		return notFound("share not found", err)
	}
	if _, _, err := s.ensureObjectAccess(ctx, share.FileID, actor); err != nil {
		return err
	}
	return s.repo.DisableShare(ctx, id)
}

func (s *Service) ShareMetadata(ctx context.Context, token string) (dto.ShareDetail, error) {
	share, err := s.validShare(ctx, token)
	if err != nil {
		return dto.ShareDetail{}, err
	}
	return dto.ShareDetailFromModel(share), nil
}

func (s *Service) VerifySharePassword(ctx context.Context, token string, password string) (string, error) {
	share, err := s.validShare(ctx, token)
	if err != nil {
		return "", err
	}
	if share.PasswordHash == nil {
		return "", badRequest("share password is not required", nil)
	}
	if !verifyPassword(*share.PasswordHash, password) {
		return "", forbidden("share password invalid")
	}
	return signDownloadToken(s.secret, share.Token, *share.PasswordHash, s.now().Add(s.tokenTTL)), nil
}

func (s *Service) OpenShare(ctx context.Context, token string, downloadToken string) (io.ReadCloser, dto.FileDetail, error) {
	share, err := s.validShare(ctx, token)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	if share.PasswordHash != nil && !verifyDownloadToken(s.secret, downloadToken, share.Token, *share.PasswordHash, s.now()) {
		return nil, dto.FileDetail{}, forbidden("share password required")
	}
	object, err := s.repo.GetObject(ctx, share.FileID)
	if err != nil {
		return nil, dto.FileDetail{}, notFound("file not found", err)
	}
	backend, ok := s.storage.Get(object.StorageCode)
	if !ok {
		return nil, dto.FileDetail{}, notFound("storage backend not found", nil)
	}
	reader, _, err := backend.Open(ctx, object.ObjectKey)
	if err != nil {
		return nil, dto.FileDetail{}, err
	}
	if err := s.repo.IncrementShareDownload(ctx, share.ID); err != nil {
		_ = reader.Close()
		return nil, dto.FileDetail{}, err
	}
	return reader, dto.FileDetailFromModel(object, s.fileURL(object)), nil
}

func (s *Service) validShare(ctx context.Context, token string) (model.Share, error) {
	share, err := s.repo.GetShareByToken(ctx, token)
	if err != nil {
		return model.Share{}, notFound("share not found", err)
	}
	if share.Status != model.ShareStatusActive {
		return model.Share{}, forbidden("share disabled")
	}
	if share.ExpiresAt != nil && s.now().After(*share.ExpiresAt) {
		return model.Share{}, forbidden("share expired")
	}
	if share.MaxDownloads != nil && share.DownloadCount >= *share.MaxDownloads {
		return model.Share{}, forbidden("share download limit reached")
	}
	return share, nil
}

func (s *Service) ensureObjectAccess(ctx context.Context, id int, actor Actor) (model.FileObject, []model.FileRef, error) {
	object, err := s.repo.GetObject(ctx, id)
	if err != nil {
		return model.FileObject{}, nil, notFound("file not found", err)
	}
	refs, err := s.refsForFile(ctx, id)
	if err != nil {
		return model.FileObject{}, nil, err
	}
	if !actor.ownsObject(object, refs) {
		return model.FileObject{}, nil, forbidden("file owner is not allowed")
	}
	return object, refs, nil
}

func (s *Service) refsForFile(ctx context.Context, fileID int) ([]model.FileRef, error) {
	refs, _, err := s.repo.ListRefs(ctx, repo.RefFilter{FileID: &fileID}, 1, 1000)
	return refs, err
}

func scopeObjectFilter(filter repo.ObjectFilter, actor Actor) repo.ObjectFilter {
	if actor.IsSuperAdmin {
		return filter
	}
	filter.OwnerType, filter.OwnerID = actor.scopedOwnerFilter(filter.OwnerType, filter.OwnerID)
	return filter
}

func scopeRefFilter(filter repo.RefFilter, actor Actor) repo.RefFilter {
	if actor.IsSuperAdmin {
		return filter
	}
	filter.OwnerType, filter.OwnerID = actor.scopedOwnerFilter(filter.OwnerType, filter.OwnerID)
	return filter
}

func scopeUsageFilter(filter repo.UsageFilter, actor Actor) repo.UsageFilter {
	if actor.IsSuperAdmin {
		return filter
	}
	filter.OwnerType, filter.OwnerID = actor.scopedOwnerFilter(filter.OwnerType, filter.OwnerID)
	return filter
}

func (s *Service) backend(ctx context.Context, scene model.Scene) (model.Storage, storage.Backend, error) {
	var storageConfig model.Storage
	var err error
	if scene.DefaultStorageCode != nil && *scene.DefaultStorageCode != "" {
		storageConfig, err = s.repo.GetStorage(ctx, *scene.DefaultStorageCode)
	} else {
		storageConfig, err = s.repo.GetDefaultStorage(ctx)
	}
	if err != nil {
		return model.Storage{}, nil, notFound("storage not found", err)
	}
	if !storageConfig.Enabled {
		return model.Storage{}, nil, badRequest("storage disabled", nil)
	}
	backend, ok := s.storage.Get(storageConfig.Code)
	if !ok {
		var err error
		backend, ok, err = s.storage.Resolve(storageBackendConfig(storageConfig))
		if err != nil {
			return model.Storage{}, nil, badRequest("storage config invalid", err)
		}
		if !ok {
			return model.Storage{}, nil, notFound("storage backend not found", nil)
		}
	}
	return storageConfig, backend, nil
}

func (s *Service) backendForObject(ctx context.Context, object model.FileObject) (storage.Backend, error) {
	storageConfig, err := s.repo.GetStorage(ctx, object.StorageCode)
	if err != nil {
		return nil, notFound("storage not found", err)
	}
	backend, ok := s.storage.Get(storageConfig.Code)
	if ok {
		return backend, nil
	}
	backend, ok, err = s.storage.Resolve(storageBackendConfig(storageConfig))
	if err != nil {
		return nil, badRequest("storage config invalid", err)
	}
	if !ok {
		return nil, notFound("storage backend not found", nil)
	}
	return backend, nil
}

func storageBackendConfig(item model.Storage) storage.BackendConfig {
	return storage.BackendConfig{
		Code:     item.Code,
		Provider: item.Provider,
		Bucket:   item.Bucket,
		Region:   item.Region,
		Endpoint: item.Endpoint,
		BaseURL:  item.BaseURL,
		Prefix:   item.Prefix,
		Config:   item.Config,
	}
}

func (s *Service) sceneParamForCreate(ctx context.Context, param dto.SceneParam) (repo.SaveSceneParam, error) {
	code := strings.TrimSpace(param.Code)
	if code == "" {
		return repo.SaveSceneParam{}, badRequest("scene code is required", nil)
	}
	name := strings.TrimSpace(param.Name)
	if name == "" {
		name = code
	}
	maxSize := param.MaxSize
	if maxSize <= 0 {
		return repo.SaveSceneParam{}, badRequest("scene max_size must be positive", nil)
	}
	tempTTL := param.TempTTLSeconds
	if tempTTL <= 0 {
		tempTTL = 24 * 60 * 60
	}
	visibility := defaultString(strings.TrimSpace(param.DefaultVisibility), model.VisibilityPrivate)
	if visibility != model.VisibilityPrivate && visibility != model.VisibilityPublic {
		return repo.SaveSceneParam{}, badRequest("scene visibility is not supported", nil)
	}
	if err := validateJSONArray(param.AllowedExts, "allowed_exts"); err != nil {
		return repo.SaveSceneParam{}, err
	}
	if err := validateJSONArray(param.AllowedMimes, "allowed_mimes"); err != nil {
		return repo.SaveSceneParam{}, err
	}
	defaultStorageCode := cleanOptional(param.DefaultStorageCode)
	if defaultStorageCode != nil {
		if _, err := s.repo.GetStorage(ctx, *defaultStorageCode); err != nil {
			return repo.SaveSceneParam{}, notFound("storage not found", err)
		}
	}
	return repo.SaveSceneParam{
		Code:               code,
		Name:               name,
		MaxSize:            maxSize,
		AllowedExts:        cleanOptional(param.AllowedExts),
		AllowedMimes:       cleanOptional(param.AllowedMimes),
		DefaultStorageCode: defaultStorageCode,
		DefaultVisibility:  visibility,
		TempTTLSeconds:     tempTTL,
		Enabled:            boolValue(param.Enabled, true),
	}, nil
}

func (s *Service) sceneParamForUpdate(ctx context.Context, current model.Scene, param dto.SceneParam) (repo.SaveSceneParam, error) {
	if strings.TrimSpace(param.Code) == "" {
		param.Code = current.Code
	}
	if strings.TrimSpace(param.Name) == "" {
		param.Name = current.Name
	}
	if param.MaxSize <= 0 {
		param.MaxSize = current.MaxSize
	}
	if param.AllowedExts == nil {
		param.AllowedExts = current.AllowedExts
	}
	if param.AllowedMimes == nil {
		param.AllowedMimes = current.AllowedMimes
	}
	if param.DefaultStorageCode == nil {
		param.DefaultStorageCode = current.DefaultStorageCode
	}
	if strings.TrimSpace(param.DefaultVisibility) == "" {
		param.DefaultVisibility = current.DefaultVisibility
	}
	if param.TempTTLSeconds <= 0 {
		param.TempTTLSeconds = current.TempTTLSeconds
	}
	if param.Enabled == nil {
		param.Enabled = &current.Enabled
	}
	return s.sceneParamForCreate(ctx, param)
}

func validateJSONArray(value *string, field string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(*value), &items); err != nil {
		return badRequest(field+" must be a JSON string array", err)
	}
	return nil
}

func storageParamForCreate(param dto.StorageParam) (repo.SaveStorageParam, error) {
	code := strings.TrimSpace(param.Code)
	if code == "" {
		return repo.SaveStorageParam{}, badRequest("storage code is required", nil)
	}
	provider := strings.TrimSpace(param.Provider)
	if provider == "" {
		provider = model.ProviderLocal
	}
	if err := validateStorageProvider(provider); err != nil {
		return repo.SaveStorageParam{}, err
	}
	if err := validateStorageConfig(param.Config); err != nil {
		return repo.SaveStorageParam{}, err
	}
	isDefault := boolValue(param.IsDefault, false)
	enabled := boolValue(param.Enabled, true)
	return repo.SaveStorageParam{
		Code:      code,
		Provider:  provider,
		Bucket:    cleanOptional(param.Bucket),
		Region:    cleanOptional(param.Region),
		Endpoint:  cleanOptional(param.Endpoint),
		BaseURL:   cleanOptional(param.BaseURL),
		Prefix:    defaultString(strings.Trim(strings.TrimSpace(param.Prefix), "/"), "uploads"),
		IsDefault: isDefault,
		Enabled:   enabled,
		Config:    cleanOptional(param.Config),
	}, nil
}

func storageParamForUpdate(current model.Storage, param dto.StorageParam) (repo.SaveStorageParam, error) {
	if strings.TrimSpace(param.Code) == "" {
		param.Code = current.Code
	}
	if strings.TrimSpace(param.Provider) == "" {
		param.Provider = current.Provider
	}
	if strings.TrimSpace(param.Prefix) == "" {
		param.Prefix = current.Prefix
	}
	if param.Bucket == nil {
		param.Bucket = current.Bucket
	}
	if param.Region == nil {
		param.Region = current.Region
	}
	if param.Endpoint == nil {
		param.Endpoint = current.Endpoint
	}
	if param.BaseURL == nil {
		param.BaseURL = current.BaseURL
	}
	if param.Config == nil {
		param.Config = current.Config
	}
	if param.IsDefault == nil {
		param.IsDefault = &current.IsDefault
	}
	if param.Enabled == nil {
		param.Enabled = &current.Enabled
	}
	return storageParamForCreate(param)
}

func validateStorageProvider(provider string) error {
	switch provider {
	case model.ProviderLocal, model.ProviderS3, model.ProviderOSS:
		return nil
	default:
		return badRequest("storage provider is not supported", nil)
	}
}

func validateStorageConfig(config *string) error {
	if config == nil || strings.TrimSpace(*config) == "" {
		return nil
	}
	if !json.Valid([]byte(*config)) {
		return badRequest("storage config must be valid JSON", nil)
	}
	return nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *Service) fileURL(object model.FileObject) string {
	return "/api/v1/public/upload/files/" + object.UUID
}

func objectKey(prefix string, sceneCode string, uuid string, ext string, now time.Time) string {
	parts := []string{
		strings.Trim(strings.TrimSpace(prefix), "/"),
		strings.Trim(strings.TrimSpace(sceneCode), "/"),
		strconv.Itoa(now.Year()),
		fmt2(now.Month()),
		fmt2(time.Duration(now.Day())),
		uuid + "." + ext,
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
}

func fmt2(value any) string {
	switch v := value.(type) {
	case time.Month:
		return twoDigit(int(v))
	case time.Duration:
		return twoDigit(int(v))
	default:
		return ""
	}
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func sanitizeFilename(filename string) string {
	name := path.Base(strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/")))
	if name == "." || name == "/" || name == "" {
		return "upload.bin"
	}
	return name
}

func allowed(value string, raw *string) bool {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return true
	}
	var items []string
	if err := json.Unmarshal([]byte(*raw), &items); err != nil {
		return true
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range items {
		if value == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
