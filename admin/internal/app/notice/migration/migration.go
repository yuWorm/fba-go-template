package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/internal/app/notice/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:notice",
		Version:  "0001",
		Name:     "notice tables",
		Checksum: "go:auto-migrate:notice:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.Notice{})
		},
	}
}
