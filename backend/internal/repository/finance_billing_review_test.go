package repository

import (
	"testing"
	"time"

	"yingce/backend/internal/model"
)

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
