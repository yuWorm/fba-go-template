package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/plugins/task/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:task",
		Version:  "0001",
		Name:     "task tables",
		Checksum: "go:auto-migrate:task:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.TaskScheduler{}, &model.TaskResult{})
		},
	}
}
