package repository

import (
	"testing"
	"time"

	"yingce/backend/internal/model"
)

func TestReviewRefundRechecksNewMediaCheckpoint(t *testing.T) {
	for _, field := range []string{"media_recovery_json", "execution_diagnostic_json"} {
		t.Run(field, func(t *testing.T) {
			repo, db := newRefundedBillingRecoveryRepository(t)
			connection, _ := db.DB()
			t.Cleanup(func() { _ = connection.Close() })
			if err := db.AutoMigrate(&model.Task{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.Task{ID: "task", UserID: "user", Status: model.TaskStatusFailed}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.CreditAccount{UserID: "user", ReservedMicrocredits: 100}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.BillingOrder{ID: "order", UserID: "user", TaskID: "task", IdempotencyKey: "order", Status: model.BillingStatusUncertain, AmountMicrocredits: 100}).Error; err != nil {
				t.Fatal(err)
			}
			candidates, err := repo.AutoRefundableBillingOrders(100)
			if err != nil || len(candidates) != 1 {
				t.Fatalf("candidates: %+v %v", candidates, err)
			}
			value := "encrypted-work"
			if field == "execution_diagnostic_json" {
				value = `{"code": "result_persistence_failed"}`
			}
			if err := db.Model(&model.Task{}).Where("id = ?", "task").Update(field, value).Error; err != nil {
				t.Fatal(err)
			}
			if changed, err := repo.RefundReviewBillingOrder("order", "auto"); err != nil || changed {
				t.Fatalf("delivered work refunded: %v %v", changed, err)
			}
			candidates, err = repo.AutoRefundableBillingOrders(100)
			if err != nil || len(candidates) != 0 {
				t.Fatalf("protected order still scanned: %+v %v", candidates, err)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", "user").Error; err != nil || account.ReservedMicrocredits != 100 {
				t.Fatalf("balance changed: %+v %v", account, err)
			}
		})
	}
}

func TestReviewRefundRechecksCandidateAndPreservesManualResolution(t *testing.T) {
	repo, db := newRefundedBillingRecoveryRepository(t)
	connection, _ := db.DB()
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user", ReservedMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{ID: "order", UserID: "user", IdempotencyKey: "order", Status: model.BillingStatusRunning,
		AmountMicrocredits: 100, UpdatedAt: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	orders, _, err := repo.AdminBillingOrders("review", "", 100, 0)
	if err != nil || len(orders) != 1 {
		t.Fatalf("review candidates = %+v, %v", orders, err)
	}
	if err := repo.UpdateBillingProviderRequestID("order", "fresh-progress"); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.RefundReviewBillingOrder("order", "auto"); err != nil || changed {
		t.Fatalf("refreshed order was refunded: %v, %v", changed, err)
	}
	if err := repo.RefundBillingOrder("order", "manual"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordBillingResolution("order", "admin", "checked manually"); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.RefundReviewBillingOrder("order", "auto"); err != nil || changed {
		t.Fatalf("already refunded order changed: %v, %v", changed, err)
	}
	order, err := repo.BillingOrder("order")
	if err != nil || order.ResolvedBy != "admin" || order.ResolutionNote != "checked manually" {
		t.Fatalf("manual resolution overwritten: %+v, %v", order, err)
	}
}

func TestReviewRefundRollsBackMoneyAndResolutionOnLedgerFailure(t *testing.T) {
	repo, db := newRefundedBillingRecoveryRepository(t)
	connection, _ := db.DB()
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.Create(&model.CreditAccount{UserID: "user", ReservedMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{ID: "order", UserID: "user", IdempotencyKey: "order", Status: model.BillingStatusUncertain, AmountMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.RefundReviewBillingOrder("order", "auto"); err == nil || changed {
		t.Fatalf("failed ledger write must reject refund: %v, %v", changed, err)
	}
	order, err := repo.BillingOrder("order")
	if err != nil || order.Status != model.BillingStatusUncertain || order.ResolutionNote != "" {
		t.Fatalf("failed refund changed order: %+v, %v", order, err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 0 || account.ReservedMicrocredits != 100 {
		t.Fatalf("failed refund changed account: %+v", account)
	}
}
