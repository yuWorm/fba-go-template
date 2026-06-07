package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
	"gorm.io/gorm/clause"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:uploadfile",
		Version:  "0001",
		Name:     "uploadfile tables",
		Checksum: "go:auto-migrate:uploadfile:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(
				&model.Storage{},
				&model.Scene{},
				&model.FileObject{},
				&model.FileRef{},
				&model.Share{},
			)
		},
	}
}

func InitialData(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:uploadfile",
		Version:  "0002",
		Name:     "uploadfile initial data",
		Checksum: "go:init-data:uploadfile:0002",
		Up: func(ctx context.Context) error {
			storages := model.SeedStorages()
			if err := provider.Write().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&storages).Error; err != nil {
				return err
			}
			scenes := model.SeedScenes()
			return provider.Write().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&scenes).Error
		},
	}
}
