package migration_test

import (
	"os"
	"strings"
	"testing"
)

func TestMySQLInitialDataConvertsNoticeTableToUTF8MB4(t *testing.T) {
	data, err := os.ReadFile("sql/mysql/0002_initial_data.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sql := strings.ToLower(string(data))
	if !strings.Contains(sql, "alter table sys_notice convert to character set utf8mb4") {
		t.Fatalf("mysql notice init SQL does not convert sys_notice to utf8mb4")
	}
}
