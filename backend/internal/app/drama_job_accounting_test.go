package app

import (
	"errors"
	"testing"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestDramaJobCostUsesActualSettlement(t *testing.T) {
	svc, fake := newDramaTestService(t)
	run := dramaRun{s: svc}
	for _, test := range []struct {
		name   string
		status model.BillingStatus
		actual int64
		want   int64
	}{
		{"settled", model.BillingStatusSettled, 45, 45},
		{"settled_zero", model.BillingStatusSettled, 0, 0},
		{"refunded", model.BillingStatusRefunded, 45, 0},
		{"reserved", model.BillingStatusReserved, 45, 0},
		{"uncertain", model.BillingStatusUncertain, 45, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			order := model.BillingOrder{ID: test.name, UserID: "usr-drama", IdempotencyKey: test.name, Status: test.status, AmountMicrocredits: 100, ActualAmountMicrocredits: test.actual}
			if err := fake.db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			cost, err := run.taskCost(&model.Task{BillingOrderID: order.ID})
			if err != nil || cost != test.want {
				t.Fatalf("cost=%d, want actual settlement %d; err=%v", cost, test.want, err)
			}
		})
	}
}

func TestDramaJobReadFailuresRemainRunning(t *testing.T) {
	for _, table := range []string{"tasks", "billing_orders"} {
		t.Run(table, func(t *testing.T) {
			svc, fake := newDramaTestService(t)
			task := model.Task{ID: "task", UserID: "usr-drama", Status: model.TaskStatusSucceeded, BillingOrderID: "order"}
			if err := fake.db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
			wantErr := errors.New("injected read outage")
			if err := fake.db.Callback().Query().Before("gorm:query").Register("drama-read-outage", func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					tx.AddError(wantErr)
				}
			}); err != nil {
				t.Fatal(err)
			}
			defer fake.db.Callback().Query().Remove("drama-read-outage")
			run := dramaRun{s: svc, production: &model.DramaProduction{SpentMicrocredits: 25}}
			job := model.DramaJob{TaskID: task.ID, Status: model.DramaJobRunning, CostMicrocredits: 10}
			err := run.syncRunningJob(&job)
			var deferred *dramaSyncDeferred
			if !errors.As(err, &deferred) || !errors.Is(deferred.err, wantErr) {
				t.Fatalf("read failure must defer synchronization: %v", err)
			}
			if job.Status != model.DramaJobRunning || job.Error != "" || job.CostMicrocredits != 10 || run.production.SpentMicrocredits != 25 {
				t.Fatalf("read failure changed job or accounting: %+v %+v", job, run.production)
			}
		})
	}
}
