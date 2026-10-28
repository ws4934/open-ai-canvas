package repository

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"yingce/backend/internal/model"
)

func TestDramaBudgetConcurrentReservationPostgres(t *testing.T) {
	for _, canvas := range []bool{false, true} {
		t.Run(fmt.Sprintf("canvas=%t", canvas), func(t *testing.T) {
			testDramaBudgetConcurrentReservationPostgres(t, canvas)
		})
	}
}

func testDramaBudgetConcurrentReservationPostgres(t *testing.T, canvas bool) {
	t.Helper()
	repo, db, application := newBillingConcurrencyRepository(t)
	for _, record := range []any{
		&model.DramaProduction{ProjectID: "drama", UserID: "user", BudgetMicrocredits: 100, SpentMicrocredits: 999},
		&model.CanvasProject{ID: "canvas", UserID: "user", ProjectID: "drama"},
		&model.CreditAccount{UserID: "user", AvailableMicrocredits: 1000, ReservedMicrocredits: 400},
		&model.Task{ID: "past", UserID: "user", ProjectID: "drama", Status: model.TaskStatusSucceeded},
		&model.BillingOrder{ID: "past-order", UserID: "user", TaskID: "past", IdempotencyKey: "past", Status: model.BillingStatusSettled, AmountMicrocredits: 80, ReservedAmountMicrocredits: 80, ActualAmountMicrocredits: 15},
		&model.Task{ID: "canvas-past", UserID: "user", ProjectID: "canvas", BillingOrderID: "canvas-past-2", Status: model.TaskStatusSucceeded},
		&model.BillingOrder{ID: "canvas-past-1", UserID: "user", TaskID: "canvas-past", IdempotencyKey: "canvas-past-1", Status: model.BillingStatusSettled, AmountMicrocredits: 70, ActualAmountMicrocredits: 5},
		&model.BillingOrder{ID: "canvas-past-2", UserID: "user", TaskID: "canvas-past", IdempotencyKey: "canvas-past-2", Status: model.BillingStatusSettled, AmountMicrocredits: 70, ActualAmountMicrocredits: 5},
		&model.Task{ID: "other-project", UserID: "user", ProjectID: "other", Status: model.TaskStatusRunning},
		&model.BillingOrder{ID: "other-order", UserID: "user", TaskID: "other-project", IdempotencyKey: "other", Status: model.BillingStatusRunning, AmountMicrocredits: 400, ReservedAmountMicrocredits: 400},
	} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	gate := db.Begin()
	if gate.Error != nil {
		t.Fatal(gate.Error)
	}
	defer gate.Rollback()
	if err := gate.Exec("SELECT user_id FROM credit_accounts WHERE user_id = ? FOR UPDATE", "user").Error; err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for i := range 2 {
		go func(i int) {
			id := fmt.Sprintf("new-%d", i)
			projectID := "drama"
			if canvas && i == 1 {
				projectID = "canvas"
			}
			order := &model.BillingOrder{ID: id, UserID: "user", TaskID: id, IdempotencyKey: id, BillingMode: "fixed_request", Status: model.BillingStatusReserved, AmountMicrocredits: 60}
			task := &model.Task{ID: id, UserID: "user", ProjectID: projectID, BillingOrderID: id, Status: model.TaskStatusQueued, InputJSON: `{}`}
			results <- repo.CreateTaskWithCreditReservation(task, order, 100)
		}(i)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-results:
			t.Fatalf("admission completed before lock barrier: %v", err)
		default:
		}
		var blocked int64
		if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND wait_event_type = 'Lock'", application).Scan(&blocked).Error; err != nil {
			t.Fatal(err)
		}
		if blocked == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("competing admissions did not reach the lock barrier: %d", blocked)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := gate.Commit().Error; err != nil {
		t.Fatal(err)
	}
	accepted, refused := 0, 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			accepted++
		case errors.Is(err, ErrDramaBudgetExceeded):
			refused++
		default:
			t.Fatalf("unexpected admission failure: %v", err)
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("project budget admitted conflicting reservations: accepted=%d refused=%d", accepted, refused)
	}
	spent, reserved, err := repo.DramaBudgetUsage("user", "drama")
	if err != nil || spent != 15 {
		t.Fatalf("budget must use actual settlement and own in-flight reservation: spent=%d reserved=%d err=%v", spent, reserved, err)
	}
	canvasSpends, err := repo.DramaCanvasSpends("user", []string{"drama"})
	if err != nil || reserved+canvasSpends["drama"] != 70 {
		t.Fatalf("pipeline and canvas must share reservations and all settled attempts: reserved=%d canvas=%v err=%v", reserved, canvasSpends, err)
	}
	var admitted []model.BillingOrder
	if err := db.Where("id IN ?", []string{"new-0", "new-1"}).Find(&admitted).Error; err != nil || len(admitted) != 1 || !admitted[0].ChargeLimitSet || admitted[0].ChargeLimitMicrocredits != 60 {
		t.Fatalf("admitted quote lost its settlement cap: %#v, %v", admitted, err)
	}
	account, err := repo.CreditAccount("user")
	if err != nil || account.AvailableMicrocredits != 940 || account.ReservedMicrocredits != 460 {
		t.Fatalf("rejected project admission changed funds: %#v, %v", account, err)
	}
}
