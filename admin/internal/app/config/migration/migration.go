package migration

import (
	"context"
	_ "embed"

	"github.com/yuWorm/fba-go-template/admin/internal/app/config/model"
	appmigration "github.com/yuWorm/fba-go-template/admin/internal/app/migration"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

//go:embed sql/mysql/0002_initial_data.sql
var mysqlInitialDataSQL string

//go:embed sql/postgresql/0002_initial_data.sql
var postgresqlInitialDataSQL string

//go:embed sql/sqlite/0002_initial_data.sql
var sqliteInitialDataSQL string

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

func InitialData(provider db.Provider) coremigration.Migration {
	return appmigration.SQLMigration(provider, appmigration.SQLMigrationOptions{
		Scope:    "plugin:config",
		Version:  "0002",
		Name:     "config initial data",
		Checksum: "sql:init-data:config:0002",
		Scripts: appmigration.SQLScripts{
			MySQL:      mysqlInitialDataSQL,
			PostgreSQL: postgresqlInitialDataSQL,
			SQLite:     sqliteInitialDataSQL,
		},
	})
}
