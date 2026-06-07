package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/dto"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	fbaerrors "github.com/yuWorm/fba-go/core/errors"
	"github.com/yuWorm/fba-go/core/fiberx"
	"github.com/yuWorm/fba-go/core/plugin"
	"github.com/yuWorm/fba-go/core/rbac"
	"github.com/yuWorm/fba-go/core/response"
)

const defaultCurrentUserID = 1

type Handler struct {
	service *service.Service
}

func NewHandler(svc *service.Service) Handler {
	if svc == nil {
		svc = service.New(nil, nil, service.Options{})
	}
	return Handler{service: svc}
}

func (h Handler) UploadFile(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return fiberx.ValidationMissingField("file")
	}
	opened, err := file.Open()
	if err != nil {
		return err
	}
	defer opened.Close()

	temp, err := optionalBool(c.FormValue("temp"))
	if err != nil {
		return err
	}
	result, err := h.service.Upload(c.RequestCtx(), service.UploadInput{
		Filename:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
		Size:        file.Size,
		Reader:      opened,
		SceneCode:   c.FormValue("scene_code", model.DefaultSceneCode),
		Field:       c.FormValue("field"),
		SubjectType: c.FormValue("subject_type"),
		SubjectID:   c.FormValue("subject_id"),
		OwnerType:   optionalForm(c, "owner_type"),
		OwnerID:     optionalForm(c, "owner_id"),
		Temp:        temp,
		Actor:       actor(c),
	})
	if err != nil {
		return err
	}
	return c.JSON(response.Success(result))
}

func (h Handler) ListFiles(c fiber.Ctx) error {
	page, size := pageParams(c)
	data, err := h.service.ListFiles(c.RequestCtx(), repo.ObjectFilter{
		Keyword:     c.Query("keyword"),
		SceneCode:   c.Query("scene_code"),
		Provider:    c.Query("provider"),
		StorageCode: c.Query("storage_code"),
		Status:      c.Query("status"),
		UploadedBy:  intPtrQuery(c, "uploaded_by"),
		OwnerType:   c.Query("owner_type"),
		OwnerID:     c.Query("owner_id"),
	}, page, size)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(data))
}

func (h Handler) GetFile(c fiber.Ctx) error {
	id, err := parseID(c.Params("pk"))
	if err != nil {
		return err
	}
	item, err := h.service.GetFile(c.RequestCtx(), id)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(item))
}

func (h Handler) DeleteFiles(c fiber.Ctx) error {
	var param dto.DeleteParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	if err := h.service.DeleteFiles(c.RequestCtx(), param.PKs); err != nil {
		return err
	}
	return c.JSON(response.Success[any](nil))
}

func (h Handler) BindRefs(c fiber.Ctx) error {
	var param dto.BindParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	if err := h.service.Bind(c.RequestCtx(), service.BindInput{
		FileIDs:     param.FileIDs,
		SceneCode:   param.SceneCode,
		SubjectType: param.SubjectType,
		SubjectID:   param.SubjectID,
		Field:       param.Field,
		OwnerType:   param.OwnerType,
		OwnerID:     param.OwnerID,
		Actor:       actor(c),
	}); err != nil {
		return err
	}
	return c.JSON(response.Success[any](nil))
}

func (h Handler) ListRefs(c fiber.Ctx) error {
	page, size := pageParams(c)
	data, err := h.service.ListRefs(c.RequestCtx(), repo.RefFilter{
		FileID:      intPtrQuery(c, "file_id"),
		SceneCode:   c.Query("scene_code"),
		SubjectType: c.Query("subject_type"),
		SubjectID:   c.Query("subject_id"),
		Field:       c.Query("field"),
		Status:      c.Query("status"),
		OwnerType:   c.Query("owner_type"),
		OwnerID:     c.Query("owner_id"),
	}, page, size)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(data))
}

func (h Handler) ListScenes(c fiber.Ctx) error {
	items, err := h.service.ListScenes(c.RequestCtx())
	if err != nil {
		return err
	}
	return c.JSON(response.Success(items))
}

func (h Handler) CreateScene(c fiber.Ctx) error {
	var param dto.SceneParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	item, err := h.service.CreateScene(c.RequestCtx(), param)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(item))
}

func (h Handler) UpdateScene(c fiber.Ctx) error {
	var param dto.SceneParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	item, err := h.service.UpdateScene(c.RequestCtx(), c.Params("code"), param)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(item))
}

func (h Handler) DeleteScene(c fiber.Ctx) error {
	if err := h.service.DeleteScene(c.RequestCtx(), c.Params("code")); err != nil {
		return err
	}
	return c.JSON(response.Success[any](nil))
}

func (h Handler) ListStorages(c fiber.Ctx) error {
	items, err := h.service.ListStorages(c.RequestCtx())
	if err != nil {
		return err
	}
	return c.JSON(response.Success(items))
}

func (h Handler) CreateStorage(c fiber.Ctx) error {
	var param dto.StorageParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	item, err := h.service.CreateStorage(c.RequestCtx(), param)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(item))
}

func (h Handler) UpdateStorage(c fiber.Ctx) error {
	var param dto.StorageParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	item, err := h.service.UpdateStorage(c.RequestCtx(), c.Params("code"), param)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(item))
}

func (h Handler) DeleteStorage(c fiber.Ctx) error {
	if err := h.service.DeleteStorage(c.RequestCtx(), c.Params("code")); err != nil {
		return err
	}
	return c.JSON(response.Success[any](nil))
}

func (h Handler) CreateShare(c fiber.Ctx) error {
	var param dto.ShareCreateParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	expiresAt, err := parseOptionalTime(param.ExpiresAt)
	if err != nil {
		return err
	}
	share, err := h.service.CreateShare(c.RequestCtx(), service.ShareInput{
		FileID:       param.FileID,
		RefID:        param.RefID,
		Password:     param.Password,
		ExpiresAt:    expiresAt,
		MaxDownloads: param.MaxDownloads,
		Actor:        actor(c),
	})
	if err != nil {
		return err
	}
	return c.JSON(response.Success(share))
}

func (h Handler) ListShares(c fiber.Ctx) error {
	page, size := pageParams(c)
	data, err := h.service.ListShares(c.RequestCtx(), repo.ShareFilter{
		FileID:    intPtrQuery(c, "file_id"),
		Status:    c.Query("status"),
		CreatedBy: intPtrQuery(c, "created_by"),
	}, page, size)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(data))
}

func (h Handler) DisableShare(c fiber.Ctx) error {
	id, err := parseID(c.Params("pk"))
	if err != nil {
		return err
	}
	if err := h.service.DisableShare(c.RequestCtx(), id); err != nil {
		return err
	}
	return c.JSON(response.Success[any](nil))
}

func (h Handler) ShareMetadata(c fiber.Ctx) error {
	share, err := h.service.ShareMetadata(c.RequestCtx(), c.Params("token"))
	if err != nil {
		return err
	}
	return c.JSON(response.Success(share))
}

func (h Handler) VerifySharePassword(c fiber.Ctx) error {
	var param dto.ShareVerifyParam
	if err := c.Bind().Body(&param); err != nil {
		return err
	}
	token, err := h.service.VerifySharePassword(c.RequestCtx(), c.Params("token"), param.Password)
	if err != nil {
		return err
	}
	return c.JSON(response.Success(map[string]string{"download_token": token}))
}

func (h Handler) DownloadShare(c fiber.Ctx) error {
	reader, file, err := h.service.OpenShare(c.RequestCtx(), c.Params("token"), c.Query("download_token"))
	if err != nil {
		return err
	}
	setDownloadHeaders(c, file)
	return c.SendStream(reader)
}

func (h Handler) OpenPublicFile(c fiber.Ctx) error {
	reader, file, err := h.service.OpenPublicFile(c.RequestCtx(), c.Params("uuid"))
	if err != nil {
		return err
	}
	setDownloadHeaders(c, file)
	return c.SendStream(reader)
}

func parseID(raw string) (int, error) {
	return fiberx.ParseIntParam("pk", raw)
}

func pageParams(c fiber.Ctx) (int, int) {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(c.Query("size", "20"))
	if err != nil || size < 1 {
		size = 20
	}
	return page, size
}

func intPtrQuery(c fiber.Ctx, name string) *int {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func optionalForm(c fiber.Ctx, name string) *string {
	value := strings.TrimSpace(c.FormValue(name))
	if value == "" {
		return nil
	}
	return &value
}

func optionalBool(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, badRequest("invalid boolean value: temp")
	}
	return &value, nil
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.ParseInLocation(dto.TimeLayout, value, time.Local)
	if err != nil {
		return nil, badRequest("invalid time value: expires_at")
	}
	return &parsed, nil
}

func actor(c fiber.Ctx) service.Actor {
	id := currentUserID(c)
	return service.Actor{UserID: &id}
}

func currentUserID(c fiber.Ctx) int {
	user, ok := c.Locals(plugin.CurrentUserLocalKey).(*rbac.CurrentUser)
	if !ok || user == nil || user.ID <= 0 {
		return defaultCurrentUserID
	}
	return int(user.ID)
}

func setDownloadHeaders(c fiber.Ctx, file dto.FileDetail) {
	if file.Mime != "" {
		c.Set(fiber.HeaderContentType, file.Mime)
	}
	if file.OriginalName != "" {
		c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+strings.ReplaceAll(file.OriginalName, `"`, "")+`"`)
	}
}

func badRequest(message string) error {
	return fbaerrors.New(http.StatusBadRequest, http.StatusBadRequest, message, nil)
}
