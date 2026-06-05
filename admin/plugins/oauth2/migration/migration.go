package migration

import (
	"context"
	_ "embed"

	appmigration "github.com/yuWorm/fba-go-template/admin/internal/app/migration"
	"github.com/yuWorm/fba-go-template/admin/plugins/oauth2/model"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

//go:embed sql/mysql/2026060302_initial_data.sql
var mysqlInitialDataSQL string

//go:embed sql/postgresql/2026060302_initial_data.sql
var postgresqlInitialDataSQL string

//go:embed sql/sqlite/2026060302_initial_data.sql
var sqliteInitialDataSQL string

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:   "plugin:oauth2",
		Version: "2026060301",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(&model.UserSocial{})
		},
	}
}

func InitialData(provider db.Provider) coremigration.Migration {
	return appmigration.SQLMigration(provider, appmigration.SQLMigrationOptions{
		Scope:    "plugin:oauth2",
		Version:  "2026060302",
		Name:     "oauth2 initial data",
		Checksum: "sql:init-data:oauth2:2026060302",
		Scripts: appmigration.SQLScripts{
			MySQL:      mysqlInitialDataSQL,
			PostgreSQL: postgresqlInitialDataSQL,
			SQLite:     sqliteInitialDataSQL,
		},
	})
}
