package database

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestPostgresSchema71FreshAndUpgrade60(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(fmt.Sprintf("upgrade60=%v", upgrade), func(t *testing.T) {
			dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
			if dsn == "" {
				t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
			}
			base, err := Open(Config{Driver: "postgres", DSN: dsn})
			if err != nil {
				t.Fatal(err)
			}
			baseSQL, _ := base.DB()
			t.Cleanup(func() { _ = baseSQL.Close() })
			schema := fmt.Sprintf("schema_v71_%d", time.Now().UnixNano())
			if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
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
			var before []schemaMigration
			if upgrade {
				if err := db.AutoMigrate(&schemaMigration{}); err != nil {
					t.Fatal(err)
				}
				for _, item := range schemaMigrations {
					if item.version > 60 {
						break
					}
					if err := item.apply(db); err != nil {
						t.Fatal(err)
					}
					if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
						t.Fatal(err)
					}
				}
				// Restore the actual v60 boundary; the baseline uses today's models.
				if err := db.Migrator().DropTable(&model.DramaProduction{}, &model.DramaCast{}, &model.DramaEpisode{}, &model.DramaShot{}, &model.DramaTake{}, &model.DramaSegment{}, &model.DramaJob{}, &model.UserStorageGrant{}, &model.UserStorageState{}, &model.ResourceOffloadJob{}); err != nil {
					t.Fatal(err)
				}
				for _, column := range []string{"Kind", "StorageBytes", "StorageDurationDays"} {
					if err := db.Migrator().DropColumn(&model.TopupProduct{}, column); err != nil {
						t.Fatal(err)
					}
				}
				for _, column := range []string{"ProductKind", "StorageBytes", "StorageDurationDays"} {
					if err := db.Migrator().DropColumn(&model.PaymentOrder{}, column); err != nil {
						t.Fatal(err)
					}
				}
				if err := db.Exec("INSERT INTO topup_products (id,name,amount_fen,credits_microcredits) VALUES ('old-product','credits',100,1000)").Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Exec("INSERT INTO payment_orders (id,user_id,merchant_order_no,amount_fen,credits_microcredits,status) VALUES ('old-order','owner','old-merchant',100,1000,'pending')").Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&model.UserFeedback{ID: "retained-feedback", UserID: "owner", Content: "preserve", Attachments: "[]"}).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Order("version").Find(&before).Error; err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := MigrateSchema(db); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
					return err
				}
				return RequireSchemaVersion(tx)
			}); err != nil {
				t.Fatal(err)
			}
			for _, table := range []any{&model.DramaProduction{}, &model.DramaCast{}, &model.DramaEpisode{}, &model.DramaShot{}, &model.DramaTake{}, &model.DramaSegment{}, &model.DramaJob{}, &model.UserStorageGrant{}, &model.UserStorageState{}, &model.ResourceOffloadJob{}} {
				if !db.Migrator().HasTable(table) {
					t.Fatalf("missing %T", table)
				}
			}
			if upgrade {
				var after []schemaMigration
				if err := db.Where("version <= 60").Order("version").Find(&after).Error; err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("historical ledger changed: %v", err)
				}
				var product model.TopupProduct
				if err := db.First(&product, "id = ?", "old-product").Error; err != nil || product.Kind != model.TopupProductKindCredits || product.CreditsMicrocredits != 1000 {
					t.Fatalf("product changed: %+v %v", product, err)
				}
				var order model.PaymentOrder
				if err := db.First(&order, "id = ?", "old-order").Error; err != nil || order.ProductKind != model.TopupProductKindCredits || order.CreditsMicrocredits != 1000 || order.Status != model.PaymentOrderPending {
					t.Fatalf("order changed: %+v %v", order, err)
				}
				var feedback model.UserFeedback
				if err := db.First(&feedback, "id = ?", "retained-feedback").Error; err != nil || feedback.Content != "preserve" {
					t.Fatalf("feedback changed: %+v %v", feedback, err)
				}
			}
		})
	}
}
