package database

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestPostgresUpgrade57To60(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CANVAS_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	base, err := Open(Config{Driver: "postgres", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	baseSQL, _ := base.DB()
	t.Cleanup(func() { _ = baseSQL.Close() })
	schema := fmt.Sprintf("schema_v60_%d", time.Now().UnixNano())
	if err := base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Error(err)
		}
	})
	testDSN, err := postgresDSNWithSearchPath(dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(Config{Driver: "postgres", DSN: testDSN})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range schemaMigrations {
		if item.version > 57 {
			break
		}
		if err := item.apply(db); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The baseline uses current models. Restore the actual v57 boundary before upgrading.
	if err := db.Migrator().DropTable(&model.AgentMemoryProposal{}, &model.AgentMemoryRevision{}, &model.UserFeedback{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		table  any
		column string
	}{
		{&model.AgentLesson{}, "OriginRunID"}, {&model.AgentLesson{}, "EvidenceJSON"},
		{&model.AgentMemorySetting{}, "CompactProposalID"}, {&model.AgentMemorySetting{}, "CompactCursor"},
	} {
		if err := db.Migrator().DropColumn(item.table, item.column); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("ALTER TABLE channel_models ALTER COLUMN protocol TYPE varchar(32)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO agent_lessons (id, author_user_id, topic, source, status) VALUES ('existing', 'owner', 'preserved', 'original', 'approved')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ChannelModel{ID: "existing", ChannelID: "channel", ModelKey: "existing", Protocol: "openai-chat"}).Error; err != nil {
		t.Fatal(err)
	}
	var before []schemaMigration
	if err := db.Order("version").Find(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
			return err
		}
		if err := RequireSchemaVersion(tx); err == nil || !strings.Contains(err.Error(), "当前 57") {
			t.Fatalf("expected read-only old schema rejection, got %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var after []schemaMigration
	if err := db.Where("version <= 57").Order("version").Find(&after).Error; err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("historical migration records changed: %v", err)
	}
	for _, table := range []any{&model.AgentMemoryProposal{}, &model.AgentMemoryRevision{}, &model.UserFeedback{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing %T", table)
		}
	}
	for _, item := range []struct {
		table  any
		column string
	}{
		{&model.AgentLesson{}, "OriginRunID"}, {&model.AgentLesson{}, "EvidenceJSON"},
		{&model.AgentMemorySetting{}, "CompactProposalID"}, {&model.AgentMemorySetting{}, "CompactCursor"},
	} {
		if !db.Migrator().HasColumn(item.table, item.column) {
			t.Fatalf("missing %T.%s", item.table, item.column)
		}
	}
	var lesson model.AgentLesson
	if err := db.First(&lesson, "id = ?", "existing").Error; err != nil || lesson.Source != "original" || lesson.Status != "approved" {
		t.Fatalf("existing lesson changed: %+v %v", lesson, err)
	}
	var length int
	if err := db.Raw("SELECT character_maximum_length FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'channel_models' AND column_name = 'protocol'").Scan(&length).Error; err != nil || length != 64 {
		t.Fatalf("protocol length = %d: %v", length, err)
	}
	if err := db.Model(&model.ChannelModel{}).Where("id = ?", "existing").Update("protocol", strings.Repeat("p", 64)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
			return err
		}
		return RequireSchemaVersion(tx)
	}); err != nil {
		t.Fatalf("v60 read-only schema check: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: CurrentSchemaVersion + 1, Name: "future", Checksum: "test", AppliedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
			return err
		}
		if err := RequireSchemaVersion(tx); err == nil || !strings.Contains(err.Error(), "拒绝使用旧程序") {
			t.Fatalf("expected future schema rejection, got %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
