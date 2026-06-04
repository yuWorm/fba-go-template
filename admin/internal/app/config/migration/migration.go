package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/internal/app/config/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:config",
		Version:  "0001",
		Name:     "config tables",
		Checksum: "go:auto-migrate:config:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.Config{})
		},
	}
}
