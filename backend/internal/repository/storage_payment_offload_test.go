package repository

import (
	"errors"
	"sync"
	"testing"
	"time"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestStoragePaymentConcurrentCompletionPostgres(t *testing.T) {
	repo, db, _ := newBillingConcurrencyRepository(t)
	if err := db.AutoMigrate(&model.PaymentOrder{}, &model.UserStorageGrant{}); err != nil {
		t.Fatal(err)
	}
	order := model.PaymentOrder{ID: "storage-order", UserID: "user", IdempotencyKey: "storage", MerchantOrderNo: "merchant", ProviderID: "test", ProductKind: model.TopupProductKindStorage, AmountFen: 100, Currency: "CNY", StorageBytes: 1 << 30, StorageDurationDays: 30, Status: model.PaymentOrderPending}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user", AvailableMicrocredits: 500}).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, granted, err := repo.CompletePaymentOrder("test", "merchant", PaymentEvidence{ProviderTradeNo: "trade", AmountFen: 100, Currency: "CNY"})
			if err != nil {
				t.Error(err)
			}
			results <- granted
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	granted := 0
	for result := range results {
		if result {
			granted++
		}
	}
	var grants []model.UserStorageGrant
	if err := db.Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if granted != 1 || len(grants) != 1 || grants[0].Bytes != 1<<30 || grants[0].ExpiresAt == nil || grants[0].ExpiresAt.Sub(grants[0].StartsAt) != 30*24*time.Hour {
		t.Fatalf("duplicate or wrong grant: %d %+v", granted, grants)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user").Error; err != nil || account.AvailableMicrocredits != 500 {
		t.Fatalf("storage purchase changed credits: %+v %v", account, err)
	}
	bad := order
	bad.ID = "bad"
	bad.MerchantOrderNo = "bad"
	bad.IdempotencyKey = "bad"
	if err := db.Create(&bad).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.UserStorageGrant{}); err != nil {
		t.Fatal(err)
	}
	if _, granted, err := repo.CompletePaymentOrder("test", "bad", PaymentEvidence{ProviderTradeNo: "bad-trade", AmountFen: 100, Currency: "CNY"}); err == nil || granted {
		t.Fatal("failed grant was reported successful")
	}
	if err := db.First(&bad, "id = ?", "bad").Error; err != nil || bad.Status != model.PaymentOrderPending {
		t.Fatalf("failed grant changed order: %+v %v", bad, err)
	}
}

func TestResourceOffloadSwitchIsAtomicAndFencedPostgres(t *testing.T) {
	repo, db, _ := newBillingConcurrencyRepository(t)
	if err := db.AutoMigrate(&model.Resource{}, &model.ResourceOffloadJob{}); err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "resource", UserID: "user", Provider: "local", ObjectKey: "local.png", Status: model.ResourceStatusReady}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	job := model.ResourceOffloadJob{ID: "job", ResourceID: resource.ID, UserID: "user", LocalObjectKey: "local.png", TargetProvider: "s3", TargetBucket: "bucket", TargetObjectKey: "remote.png", Status: model.ResourceOffloadStatusPending, NextAttemptAt: time.Now().Add(-time.Minute)}
	if err := repo.UpsertResourceOffloadJob(&job); err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimNextResourceOffloadJob("owner", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if changed, err := repo.SwitchResourceToObjectStorage(claimed, "stale", "etag", "", time.Now()); !errors.Is(err, ErrResourceOffloadLeaseLost) || changed {
		t.Fatalf("stale writer: %v %v", changed, err)
	}
	const callback = "test:fail-offload-finish"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "resource_offload_jobs" {
			tx.AddError(errors.New("forced finish failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.SwitchResourceToObjectStorage(claimed, "owner", "etag", "", time.Now()); err == nil || changed {
		t.Fatal("partial switch succeeded")
	}
	_ = db.Callback().Update().Remove(callback)
	stored, err := repo.Resource(resource.ID)
	if err != nil || stored.Provider != "local" {
		t.Fatalf("resource switch escaped rollback: %+v %v", stored, err)
	}
	deadline := time.Now().Add(15 * time.Minute)
	if changed, err := repo.SwitchResourceToObjectStorage(claimed, "owner", "etag", "playback.mp4", deadline); err != nil || !changed {
		t.Fatalf("switch: %v %v", changed, err)
	}
	if err := db.First(&job, "id = ?", job.ID).Error; err != nil || job.Status != model.ResourceOffloadStatusDone || job.LocalDeleteAfter == nil || job.LeaseOwner != "" {
		t.Fatalf("finish: %+v %v", job, err)
	}
	stored, err = repo.Resource(resource.ID)
	if err != nil || stored.Provider != "s3" || stored.ObjectKey != "remote.png" {
		t.Fatalf("switch resource: %+v %v", stored, err)
	}
}
