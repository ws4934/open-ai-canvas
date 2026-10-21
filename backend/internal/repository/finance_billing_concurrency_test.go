package repository

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"yingce/backend/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBillingConcurrencyRepository(t *testing.T) (*Repository, *gorm.DB, string) {
	t.Helper()
	dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set CANVAS_TEST_POSTGRES_DSN to an isolated test database")
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("billing_concurrency_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		connection, _ := admin.DB()
		_ = connection.Close()
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	query.Set("application_name", schema)
	query.Set("statement_timeout", "10000")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	connection.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.CreditLedgerEntry{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	return New(db), db, schema
}

func TestBillingTerminalConcurrencyPostgres(t *testing.T) {
	for _, actions := range [][2]string{{"refund", "refund"}, {"refund", "settle"}, {"settle", "settle"}, {"auto", "refund"}, {"auto", "settle"}} {
		t.Run(actions[0]+"_"+actions[1], func(t *testing.T) {
			repo, db, application := newBillingConcurrencyRepository(t)
			if err := db.Create(&model.CreditAccount{UserID: "user", AvailableMicrocredits: 800, ReservedMicrocredits: 200}).Error; err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"a", "b"} {
				if err := db.Create(&model.BillingOrder{ID: id, UserID: "user", IdempotencyKey: id, Status: model.BillingStatusRunning,
					BillingMode: "fixed", AmountMicrocredits: 100, ReservedAmountMicrocredits: 100, UpdatedAt: time.Now().Add(-time.Hour)}).Error; err != nil {
					t.Fatal(err)
				}
			}

			// Hold only the account row: both transactions can reach the disputed
			// order before either changes money. With order locking, one instead
			// waits on the order; without it, both spend the aggregate reservation.
			gate := db.Begin()
			if gate.Error != nil {
				t.Fatal(gate.Error)
			}
			defer gate.Rollback()
			if err := gate.Exec("SELECT user_id FROM credit_accounts WHERE user_id = ? FOR UPDATE", "user").Error; err != nil {
				t.Fatal(err)
			}
			results := make(chan error, 2)
			for _, action := range actions {
				go func(action string) {
					if action == "auto" {
						_, err := repo.RefundReviewBillingOrder("a", "fixture auto refund")
						results <- err
					} else if action == "refund" {
						results <- repo.RefundBillingOrder("a", "fixture refund")
					} else {
						results <- repo.SettleBillingOrder("a", "")
					}
				}(action)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				var blocked int64
				if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND wait_event_type = 'Lock'", application).Scan(&blocked).Error; err != nil {
					t.Fatal(err)
				}
				if blocked == 2 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("both transactions did not reach the lock barrier: %d", blocked)
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := gate.Commit().Error; err != nil {
				t.Fatal(err)
			}
			errors := 0
			for range 2 {
				if err := <-results; err != nil {
					errors++
				}
			}
			if actions[0] != "auto" && (actions[0] == actions[1] && errors != 0 || actions[0] != actions[1] && errors != 1) {
				t.Fatalf("terminal errors = %d for %v", errors, actions)
			}
			if actions[0] == "auto" && errors > 1 {
				t.Fatalf("neither competing operation completed: %v", actions)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", "user").Error; err != nil {
				t.Fatal(err)
			}
			order, err := repo.BillingOrder("a")
			if err != nil {
				t.Fatal(err)
			}
			wantAvailable := int64(800)
			if order.Status == model.BillingStatusRefunded {
				wantAvailable = 900
			} else if order.Status != model.BillingStatusSettled {
				t.Fatalf("unexpected terminal state: %s", order.Status)
			}
			if account.AvailableMicrocredits != wantAvailable || account.ReservedMicrocredits != 100 {
				t.Fatalf("other order's reservation was consumed: available=%d reserved=%d, want %d/100", account.AvailableMicrocredits, account.ReservedMicrocredits, wantAvailable)
			}
			other, err := repo.BillingOrder("b")
			if err != nil || other.Status != model.BillingStatusRunning {
				t.Fatalf("other order changed: %+v, %v", other, err)
			}
			var count int64
			if err := db.Model(&model.CreditLedgerEntry{}).Where("billing_order_id = ?", "a").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("terminal ledger entries = %d, want 1", count)
			}
		})
	}
}
