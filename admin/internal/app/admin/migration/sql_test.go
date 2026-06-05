package migration_test

import (
	"os"
	"strings"
	"testing"
)

func TestInitialDataSQLIncludesUserDeletedColumn(t *testing.T) {
	for _, path := range []string{
		"sql/mysql/0003_initial_data.sql",
		"sql/postgresql/0003_initial_data.sql",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		sql := strings.ToLower(string(data))
		if !strings.Contains(sql, "insert into sys_user") || !strings.Contains(sql, "deleted, join_time") {
			t.Fatalf("%s sys_user insert does not include deleted before join_time", path)
		}
	}
}
