package service

import (
	"strconv"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
)

const ownerTypeUser = "user"

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
		if a.allowsOwner(ref.OwnerType, ref.OwnerID) {
			return true
		}
	}
	return false
}

func ownerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
