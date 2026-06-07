package service

import (
	"strconv"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

const ownerTypeUser = "user"
const ownerNoMatch = "__uploadfile_no_owner_match__"

type Actor struct {
	UserID       *int
	IsSuperAdmin bool
}

func (a Actor) defaultOwner() (*string, *string) {
	if a.UserID == nil || *a.UserID <= 0 {
		return nil, nil
	}
	ownerType := ownerTypeUser
	ownerID := strconv.Itoa(*a.UserID)
	return &ownerType, &ownerID
}

func (a Actor) allowsOwner(ownerType *string, ownerID *string) bool {
	if a.IsSuperAdmin {
		return true
	}
	defaultType, defaultID := a.defaultOwner()
	if defaultType == nil || defaultID == nil {
		return ownerType == nil && ownerID == nil
	}
	return ownerValue(ownerType) == *defaultType && ownerValue(ownerID) == *defaultID
}

func (a Actor) ownsObject(object model.FileObject, refs []model.FileRef) bool {
	if a.IsSuperAdmin {
		return true
	}
	if a.UserID != nil && object.UploadedBy != nil && *object.UploadedBy == *a.UserID {
		return true
	}
	for _, ref := range refs {
		if ref.Status == model.RefStatusDeleted {
			continue
		}
		if a.allowsOwner(ref.OwnerType, ref.OwnerID) {
			return true
		}
	}
	return false
}

func (a Actor) scopedOwnerFilter(ownerType string, ownerID string) (string, string) {
	if a.IsSuperAdmin {
		return ownerType, ownerID
	}
	defaultType, defaultID := a.defaultOwner()
	if defaultType == nil || defaultID == nil {
		return ownerNoMatch, ownerNoMatch
	}
	if ownerType != "" && ownerType != *defaultType {
		return ownerNoMatch, ownerNoMatch
	}
	if ownerID != "" && ownerID != *defaultID {
		return ownerNoMatch, ownerNoMatch
	}
	return *defaultType, *defaultID
}

func ownerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
