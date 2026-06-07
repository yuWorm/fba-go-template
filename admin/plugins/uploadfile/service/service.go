package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
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

type Options struct {
	TokenSecret      []byte
	Now              Clock
	DownloadTokenTTL time.Duration
}

type Service struct {
	repo     repo.Repository
	storage  *storage.Registry
	secret   []byte
	now      Clock
	tokenTTL time.Duration
}

type Actor struct {
	UserID *int
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
	return &Service{repo: repository, storage: registry, secret: opts.TokenSecret, now: opts.Now, tokenTTL: opts.DownloadTokenTTL}
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
		OwnerType:   cleanOptional(input.OwnerType),
		OwnerID:     cleanOptional(input.OwnerID),
		CreatedBy:   input.Actor.UserID,
	})
	if err != nil {
		return dto.UploadResult{}, err
	}
	return dto.UploadResult{File: dto.FileDetailFromModel(object, s.fileURL(object)), Ref: dto.RefDetailFromModel(ref, object, s.fileURL(object))}, nil
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
	return s.repo.BindRefs(ctx, repo.BindRefsParam{
		FileIDs:     input.FileIDs,
		SceneCode:   sceneCode,
		SubjectType: strings.TrimSpace(input.SubjectType),
		SubjectID:   strings.TrimSpace(input.SubjectID),
		Field:       strings.TrimSpace(input.Field),
		OwnerType:   cleanOptional(input.OwnerType),
		OwnerID:     cleanOptional(input.OwnerID),
	})
}

func (s *Service) ListRefs(ctx context.Context, filter repo.RefFilter, page int, size int) (pagination.PageData[dto.RefDetail], error) {
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

func (s *Service) ListFiles(ctx context.Context, filter repo.ObjectFilter, page int, size int) (pagination.PageData[dto.FileDetail], error) {
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

func (s *Service) GetFile(ctx context.Context, id int) (dto.FileDetail, error) {
	object, err := s.repo.GetObject(ctx, id)
	if err != nil {
		return dto.FileDetail{}, notFound("file not found", err)
	}
	return dto.FileDetailFromModel(object, s.fileURL(object)), nil
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
	return s.repo.DeleteStorage(ctx, code)
}

func (s *Service) DeleteFiles(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return badRequest("pks is required", nil)
	}
	for _, id := range ids {
		if err := s.repo.UpdateObjectStatus(ctx, id, model.StatusDeleted); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CleanupExpiredTemps(ctx context.Context) (dto.CleanupResult, error) {
	expired, err := s.repo.ListExpiredTempRefs(ctx, s.now())
	if err != nil {
		return dto.CleanupResult{}, err
	}
	result := dto.CleanupResult{ExpiredRefs: len(expired)}
	if len(expired) == 0 {
		return result, nil
	}
	refIDs := make([]int, 0, len(expired))
	fileIDs := make(map[int]bool, len(expired))
	for _, ref := range expired {
		refIDs = append(refIDs, ref.ID)
		fileIDs[ref.FileID] = true
	}
	if err := s.repo.UpdateRefsStatus(ctx, refIDs, model.RefStatusDeleted); err != nil {
		return dto.CleanupResult{}, err
	}
	for fileID := range fileIDs {
		remaining, err := s.repo.CountRefsByFileStatus(ctx, fileID, []string{model.RefStatusTemp, model.RefStatusActive})
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
	}
	return result, nil
}

func (s *Service) OpenPublicFile(ctx context.Context, uuid string) (io.ReadCloser, dto.FileDetail, error) {
	object, err := s.repo.GetObjectByUUID(ctx, strings.TrimSpace(uuid))
	if err != nil {
		return nil, dto.FileDetail{}, notFound("file not found", err)
	}
	if object.Visibility != model.VisibilityPublic {
		return nil, dto.FileDetail{}, forbidden("file is not public")
	}
	backend, ok := s.storage.Get(object.StorageCode)
	if !ok {
		return nil, dto.FileDetail{}, notFound("storage backend not found", nil)
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
	if _, err := s.repo.GetObject(ctx, input.FileID); err != nil {
		return dto.ShareDetail{}, notFound("file not found", err)
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

func (s *Service) ListShares(ctx context.Context, filter repo.ShareFilter, page int, size int) (pagination.PageData[dto.ShareDetail], error) {
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

func (s *Service) DisableShare(ctx context.Context, id int) error {
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
