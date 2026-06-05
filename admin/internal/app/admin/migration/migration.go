package migration

import (
	"context"
	_ "embed"

	"github.com/yuWorm/fba-go-template/admin/internal/app/admin/model"
	"github.com/yuWorm/fba-go-template/admin/internal/app/admin/repo"
	appmigration "github.com/yuWorm/fba-go-template/admin/internal/app/migration"
	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
)

//go:embed sql/mysql/0003_initial_data.sql
var mysqlInitialDataSQL string

//go:embed sql/postgresql/0003_initial_data.sql
var postgresqlInitialDataSQL string

//go:embed sql/sqlite/0003_initial_data.sql
var sqliteInitialDataSQL string

//go:embed sql/mysql/0004_user_deleted_default.sql
var mysqlUserDeletedDefaultSQL string

//go:embed sql/postgresql/0004_user_deleted_default.sql
var postgresqlUserDeletedDefaultSQL string

//go:embed sql/sqlite/0004_user_deleted_default.sql
var sqliteUserDeletedDefaultSQL string

func AutoMigrate(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:admin",
		Version:  "0001",
		Name:     "admin core tables",
		Checksum: "go:auto-migrate:admin:0001",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(
				&model.User{},
				&model.UserPasswordHistory{},
				&model.Role{},
				&model.Menu{},
				&model.Dept{},
				&model.DataRule{},
				&model.DataScope{},
				&model.Plugin{},
				&model.LoginLog{},
				&model.OperaLog{},
				&model.Session{},
				&repo.UserRole{},
				&repo.RoleMenu{},
				&repo.RoleDataScope{},
				&repo.DataScopeRule{},
				&repo.PluginState{},
			)
		},
	}
}

func InitialData(provider db.Provider) coremigration.Migration {
	return appmigration.SQLMigration(provider, appmigration.SQLMigrationOptions{
		Scope:    "plugin:admin",
		Version:  "0003",
		Name:     "admin initial data",
		Checksum: "sql:init-data:admin:0003",
		Scripts: appmigration.SQLScripts{
			MySQL:      mysqlInitialDataSQL,
			PostgreSQL: postgresqlInitialDataSQL,
			SQLite:     sqliteInitialDataSQL,
		},
	})
}

func UserDeletedDefaultMigration(provider db.Provider) coremigration.Migration {
	return appmigration.SQLMigration(provider, appmigration.SQLMigrationOptions{
		Scope:    "plugin:admin",
		Version:  "0004",
		Name:     "admin user deleted default",
		Checksum: "sql:user-deleted-default:admin:0004",
		Scripts: appmigration.SQLScripts{
			MySQL:      mysqlUserDeletedDefaultSQL,
			PostgreSQL: postgresqlUserDeletedDefaultSQL,
			SQLite:     sqliteUserDeletedDefaultSQL,
		},
	})
}

func PasswordSecurityMigration(provider db.Provider) coremigration.Migration {
	return coremigration.Migration{
		Scope:    "plugin:admin",
		Version:  "0002",
		Name:     "admin password security tables",
		Checksum: "go:auto-migrate:admin:0002-password-security",
		Up: func(ctx context.Context) error {
			return provider.Write().WithContext(ctx).AutoMigrate(
				&model.User{},
				&model.UserPasswordHistory{},
			)
		},
	}
}
