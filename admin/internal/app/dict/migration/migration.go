package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/internal/app/dict/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:dict",
		Version:  "0001",
		Name:     "dict tables",
		Checksum: "go:auto-migrate:dict:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.DictType{}, &model.DictData{})
		},
	}
}
