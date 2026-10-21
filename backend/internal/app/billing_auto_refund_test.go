package app

import (
	"path/filepath"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBillingAutoRefundDefaultsDisabled(t *testing.T) {
	svc := newResourceTestService(t)
	setting, err := svc.billingAutoRefundSetting()
	if err != nil {
		t.Fatal(err)
	}
	if setting.Enabled {
		t.Fatal("自动退回默认必须关闭")
	}
	// 关闭时在读取订单之前就返回；测试库未迁移 billing_orders，若误查询会记录告警但仍返回 0。
	if refunded := svc.autoRefundReviewBilling(t.Context()); refunded != 0 {
		t.Fatalf("关闭时不应退回订单，实际退回 %d", refunded)
	}
}

func TestBillingAutoRefundRequiresAdmin(t *testing.T) {
	svc := newResourceTestService(t)
	for _, actor := range []*model.User{nil, {ID: "user-1", Role: model.UserRoleUser}} {
		if _, err := svc.UpdateBillingAutoRefundSetting(actor, BillingAutoRefundSetting{Enabled: true}); err == nil {
			t.Fatal("未登录或非管理员不能修改自动退回")
		}
		if _, err := svc.AdminBillingAutoRefundSetting(actor); err == nil {
			t.Fatal("未登录或非管理员不能读取自动退回设置")
		}
	}
	setting, err := svc.billingAutoRefundSetting()
	if err != nil {
		t.Fatal(err)
	}
	if setting.Enabled {
		t.Fatal("被拒绝的请求不能写入开关")
	}
}

func TestBillingAutoRefundSettingAuditFailureRollsBack(t *testing.T) {
	svc := newResourceTestService(t) // No audit table: fail after the setting write.
	_, err := svc.UpdateBillingAutoRefundSetting(&model.User{ID: "admin", Role: model.UserRoleAdmin}, BillingAutoRefundSetting{Enabled: true})
	if err == nil {
		t.Fatal("expected the missing audit table to reject the update")
	}
	setting, err := svc.billingAutoRefundSetting()
	if err != nil || setting.Enabled {
		t.Fatalf("failed audited update must leave refunds disabled: %+v, %v", setting, err)
	}
}

func TestBillingAutoRefundReviewCandidatesAndIdempotency(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "billing.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.AdminAuditEvent{}, &model.BillingOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	if err := db.Create(&model.CreditAccount{UserID: "user", AvailableMicrocredits: 400, ReservedMicrocredits: 600}).Error; err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		id     string
		status model.BillingStatus
		task   model.TaskStatus
		age    time.Duration
	}{
		{"uncertain", model.BillingStatusUncertain, model.TaskStatusRunning, 0},
		{"stale", model.BillingStatusRunning, model.TaskStatusRunning, 41 * time.Minute},
		{"fresh", model.BillingStatusRunning, model.TaskStatusRunning, 39 * time.Minute},
		{"failed", model.BillingStatusReserved, model.TaskStatusFailed, 0},
		{"cancelled", model.BillingStatusReserved, model.TaskStatusCancelled, 0},
		{"queued", model.BillingStatusReserved, model.TaskStatusQueued, 41 * time.Minute},
	} {
		if err := db.Create(&model.Task{ID: fixture.id, UserID: "user", Status: fixture.task}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.BillingOrder{ID: fixture.id, UserID: "user", TaskID: fixture.id,
			IdempotencyKey: fixture.id, Status: fixture.status, AmountMicrocredits: 100, ReservedAmountMicrocredits: 100,
			UpdatedAt: time.Now().Add(-fixture.age)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if count := svc.autoRefundReviewBilling(t.Context()); count != 0 {
		t.Fatalf("disabled scan refunded %d orders", count)
	}
	setting, err := svc.UpdateBillingAutoRefundSetting(&model.User{ID: "admin", Role: model.UserRoleAdmin}, BillingAutoRefundSetting{Enabled: true})
	if err != nil || !setting.Enabled || setting.UpdatedAt == nil {
		t.Fatalf("enable = %+v, %v", setting, err)
	}
	if count := svc.autoRefundReviewBilling(t.Context()); count != 4 {
		t.Fatalf("refunded %d orders, want 4", count)
	}
	if count := svc.autoRefundReviewBilling(t.Context()); count != 0 {
		t.Fatalf("duplicate scan reported %d new refunds", count)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 800 || account.ReservedMicrocredits != 200 {
		t.Fatalf("account = %+v, want available=800 reserved=200", account)
	}
	for _, id := range []string{"uncertain", "stale", "failed", "cancelled"} {
		order, err := svc.repo.BillingOrder(id)
		if err != nil || order.Status != model.BillingStatusRefunded || order.ResolutionNote != billingAutoRefundNote {
			t.Fatalf("refund %s = %+v, %v", id, order, err)
		}
	}
	var ledgerCount, auditCount int64
	if err := db.Model(&model.CreditLedgerEntry{}).Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AdminAuditEvent{}).Where("action = ?", "billing_auto_refund.update").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 4 || auditCount != 1 {
		t.Fatalf("ledger/audit counts = %d/%d, want 4/1", ledgerCount, auditCount)
	}
}
