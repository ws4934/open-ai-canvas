package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yingce/backend/internal/buildinfo"
	"yingce/backend/internal/database"
)

func TestVersionCommandDoesNotInitializeApplication(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	t.Setenv("CANVAS_BACKEND_DATA_DIR", dir)
	var output bytes.Buffer
	handled, err := runCommand(context.Background(), []string{"--version"}, &output)
	if err != nil || !handled {
		t.Fatalf("command: %v, %v", handled, err)
	}
	var info buildinfo.Info
	if err := json.Unmarshal(output.Bytes(), &info); err != nil || info.Version == "" {
		t.Fatalf("invalid version JSON %s: %v", output.String(), err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("version command created the data directory")
	}
}

func TestReadOnlySchemaCheckDoesNotCreateDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CANVAS_BACKEND_DATA_DIR", dir)
	t.Setenv("CANVAS_DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", "")
	if err := checkDatabaseSchema(context.Background()); err == nil {
		t.Fatal("missing database must fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "open_ai_canvas.db")); !os.IsNotExist(err) {
		t.Fatal("read-only check created a database")
	}
}

func TestReadOnlySQLiteDSN(t *testing.T) {
	dsn, err := readOnlySQLiteDSN(database.Config{DSN: "file:/tmp/existing.db?mode=rwc&_journal_mode=WAL&_busy_timeout=5000"})
	if err != nil || !strings.Contains(dsn, "mode=ro") || !strings.Contains(dsn, "_query_only=1") || strings.Contains(dsn, "journal_mode") {
		t.Fatalf("unexpected read-only DSN %q, %v", dsn, err)
	}
	if _, err := readOnlySQLiteDSN(database.Config{DSN: ":memory:"}); err == nil {
		t.Fatal("memory database is not a rollback target")
	}
}

func TestSchemaCheckAcceptsCurrentAndRejectsNewerDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CANVAS_BACKEND_DATA_DIR", dir)
	t.Setenv("CANVAS_DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", "")
	db, err := database.Open(database.Config{Driver: "sqlite", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := checkDatabaseSchema(context.Background()); err != nil {
		t.Fatalf("compatible database rejected: %v", err)
	}
	if err := db.Table("schema_migrations").Where("version = ?", database.CurrentSchemaVersion).Update("version", database.CurrentSchemaVersion+1).Error; err != nil {
		t.Fatal(err)
	}
	if err := checkDatabaseSchema(context.Background()); err == nil {
		t.Fatal("older program accepted a newer database schema")
	}
	var latest int64
	if err := db.Table("schema_migrations").Select("MAX(version)").Scan(&latest).Error; err != nil || latest != database.CurrentSchemaVersion+1 {
		t.Fatalf("check changed the schema: version=%d error=%v", latest, err)
	}
}
