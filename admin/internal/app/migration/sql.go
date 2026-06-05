package migration

import (
	"context"
	"fmt"
	"strings"

	"github.com/yuWorm/fba-go/core/db"
	coremigration "github.com/yuWorm/fba-go/core/migration"
	"gorm.io/gorm"
)

type SQLScripts struct {
	MySQL      string
	PostgreSQL string
	SQLite     string
}

type SQLMigrationOptions struct {
	Scope    string
	Version  string
	Name     string
	Checksum string
	Scripts  SQLScripts
}

func SQLMigration(provider db.Provider, opts SQLMigrationOptions) coremigration.Migration {
	return coremigration.Migration{
		Scope:    opts.Scope,
		Version:  opts.Version,
		Name:     opts.Name,
		Checksum: opts.Checksum,
		Up: func(ctx context.Context) error {
			return ExecuteSQL(ctx, provider, opts.Scripts)
		},
	}
}

func ExecuteSQL(ctx context.Context, provider db.Provider, scripts SQLScripts) error {
	if provider == nil || provider.Write() == nil {
		return fmt.Errorf("database provider is required for SQL migration")
	}
	sql, err := scriptForDialect(provider.Write(), scripts)
	if err != nil {
		return err
	}
	statements := splitStatements(sql)
	if len(statements) == 0 {
		return nil
	}
	return provider.Write().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func scriptForDialect(database *gorm.DB, scripts SQLScripts) (string, error) {
	switch strings.ToLower(database.Dialector.Name()) {
	case "mysql":
		return requireScript("mysql", scripts.MySQL)
	case "postgres", "postgresql":
		return requireScript("postgresql", scripts.PostgreSQL)
	case "sqlite":
		return requireScript("sqlite", scripts.SQLite)
	default:
		return "", fmt.Errorf("unsupported database dialect %q for SQL migration", database.Dialector.Name())
	}
}

func requireScript(dialect string, script string) (string, error) {
	if strings.TrimSpace(script) == "" {
		return "", fmt.Errorf("missing %s SQL migration script", dialect)
	}
	return script, nil
}

func splitStatements(sql string) []string {
	statements := make([]string, 0)
	remaining := strings.TrimSpace(sql)
	for remaining != "" {
		// PostgreSQL init scripts use DO $$ blocks that contain internal semicolons,
		// so each block must stay intact before regular statement splitting.
		if strings.HasPrefix(strings.ToLower(remaining), "do $$") {
			end := strings.Index(strings.ToLower(remaining), "end $$;")
			if end < 0 {
				statements = append(statements, remaining)
				break
			}
			end += len("end $$;")
			statements = append(statements, strings.TrimSpace(remaining[:end]))
			remaining = strings.TrimSpace(remaining[end:])
			continue
		}
		next := strings.Index(remaining, ";")
		if next < 0 {
			statements = appendNonEmpty(statements, remaining)
			break
		}
		statements = appendNonEmpty(statements, remaining[:next])
		remaining = strings.TrimSpace(remaining[next+1:])
	}
	return statements
}

func appendNonEmpty(statements []string, statement string) []string {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return statements
	}
	return append(statements, statement)
}
