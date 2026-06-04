package migration

import (
	"context"

	"github.com/yuWorm/fba-go-template/admin/plugins/oauth2/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:   "plugin:oauth2",
		Version: "2026060301",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.UserSocial{})
		},
	}
}
