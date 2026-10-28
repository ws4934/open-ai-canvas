package app

import (
	"context"
	"strings"
	"testing"

	"yingce/backend/internal/model"
)

func TestAgentMediaAdmissionPersistsOriginAndChargeAuthorization(t *testing.T) {
	s, _, args := agentMediaFixture(t)
	run, state := agentMediaRun(t, s, args, "auto")
	req, _, err := s.prepareCloudAgentMedia(run, &state, agentMediaCall(args))
	if err != nil {
		t.Fatal(err)
	}
	req.admission = &taskAdmission{ID: "generation-task", MaxCharge: 1000, AgentRunID: run.ID, GenerationID: "generation", ApprovalID: "approval"}
	req.creationPrepare = &creationTaskPreparation{}
	task, err := s.CreateTask(run.UserID, req)
	if err != nil {
		t.Fatal(err)
	}
	order := req.creationPrepare.Order
	if task.AgentRunID != run.ID || task.GenerationID != "generation" || task.ApprovalID != "approval" || order == nil || !order.ChargeLimitSet || task.AuthorizedChargeMicrocredits != order.AmountMicrocredits || order.ChargeLimitMicrocredits != order.AmountMicrocredits {
		t.Fatalf("incomplete generation authorization: task=%+v order=%+v", task, order)
	}
}

func TestAgentMediaRetryRequiresNewGenerationAuthorization(t *testing.T) {
	s, db, _ := agentMediaFixture(t)
	for _, task := range []model.Task{
		{ID: "agent-run-only", AgentRunID: "run"},
		{ID: "generation-only", GenerationID: "generation"},
		{ID: "operation-prefix", Operation: "cloud_agent_media"},
		{ID: "source", InputJSON: `{"metadata":{"source":"cloud_agent"}}`},
	} {
		task.UserID, task.Type, task.Status = "user", "canvas_video", model.TaskStatusFailed
		if task.Operation == "" {
			task.Operation = "image_to_video"
		}
		if task.InputJSON == "" {
			task.InputJSON = `{}`
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := s.RetryTask(task.UserID, task.ID); err == nil || !strings.Contains(err.Error(), "Agent 重试") {
			t.Fatalf("media retry retained old authorization: %v", err)
		}
		stored, err := s.repo.Task(task.ID)
		if err != nil || stored.Status != model.TaskStatusFailed || stored.BillingOrderID != "" {
			t.Fatalf("retry changed old execution: %+v %v", stored, err)
		}
	}
}

func TestOrdinaryFailedAndCancelledTasksKeepRetryIdentity(t *testing.T) {
	s, db, _ := agentMediaFixture(t)
	for _, status := range []model.TaskStatus{model.TaskStatusFailed, model.TaskStatusCancelled} {
		task := model.Task{
			ID:        "ordinary-" + string(status),
			UserID:    "user",
			Type:      "canvas_text",
			Operation: "text",
			Status:    status,
			InputJSON: `{}`,
			RouteRun:  3,
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}

		retried, err := s.RetryTask(task.UserID, task.ID)
		if err != nil {
			t.Fatalf("ordinary %s task should remain retryable: %v", status, err)
		}
		if retried.ID != task.ID || retried.Status != model.TaskStatusQueued || retried.RouteRun != 4 || retried.GenerationID != "" || retried.AgentRunID != "" {
			t.Fatalf("retry must reuse task execution identity without inventing an Agent generation: %+v", retried)
		}
		if _, err = s.RetryTask(task.UserID, task.ID); err == nil {
			t.Fatal("queued retry was accepted a second time")
		}
	}
}

func TestContentModerationTasksCanRetryUnchangedInput(t *testing.T) {
	for _, taskType := range []string{"canvas_image", "canvas_video", "canvas_audio", "canvas_text", "timeline_render", "timeline_transcription"} {
		t.Run(taskType, func(t *testing.T) {
			s, db, _, _ := creationTestService(t)
			task := model.Task{
				ID: "moderation-" + taskType, UserID: "user", Type: taskType,
				Status: model.TaskStatusFailed, Error: "content_policy_violation：内容未通过上游安全审核",
				Prompt: "原始提示词", InputJSON: `{"prompt":"原始提示词"}`, RouteRun: 1,
			}
			if err := db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
			retried, err := s.RetryTask(task.UserID, task.ID)
			if err != nil {
				t.Fatalf("moderation failure should allow manual retry: %v", err)
			}
			stored, err := s.repo.TaskForUser(task.UserID, task.ID)
			if err != nil || retried.Status != model.TaskStatusQueued || stored.InputJSON != task.InputJSON || stored.Prompt != task.Prompt || stored.Error != "" || stored.RouteRun != 2 {
				t.Fatalf("retry must retain original input and enqueue one new attempt: task=%+v err=%v", stored, err)
			}
			if _, err := s.RetryTask(task.UserID, task.ID); err == nil {
				t.Fatal("duplicate retry accepted while already queued")
			}
		})
	}
}

func TestContentModerationRetryCreatesNewBillingOrder(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	task, err := s.CreateTask("user", creationTextRequest())
	if err != nil {
		t.Fatal(err)
	}
	if task.BillingOrderID == "" {
		t.Fatal("paid fixture has no billing order")
	}
	if err := s.taskBilling().RefundBilling(task.BillingOrderID, "content_policy_violation"); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(task).Updates(map[string]any{"status": model.TaskStatusFailed, "error": "sensitive_words_detected"}).Error; err != nil {
		t.Fatal(err)
	}
	retried, err := s.RetryTask("user", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != model.TaskStatusQueued || retried.BillingOrderID == "" || retried.BillingOrderID == task.BillingOrderID {
		t.Fatalf("retry must use a new billing order: %+v", retried)
	}
	oldOrder, err := s.repo.BillingOrder(task.BillingOrderID)
	if err != nil || oldOrder.Status != model.BillingStatusRefunded {
		t.Fatalf("previous billing order changed: %+v %v", oldOrder, err)
	}
}

func TestContentModerationRetryStillWaitsForBillingReview(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	order := model.BillingOrder{ID: "moderation-review", UserID: "user", Status: model.BillingStatusUncertain}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	task := model.Task{ID: "moderation-review", UserID: "user", Type: "canvas_image", Status: model.TaskStatusFailed, Error: "content_policy_violation", InputJSON: `{}`, BillingOrderID: order.ID}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryTask("user", task.ID); err == nil || !strings.Contains(err.Error(), "费用仍在核对") {
		t.Fatalf("billing review must still prevent duplicate charges: %v", err)
	}
	stored, err := s.repo.TaskForUser("user", task.ID)
	if err != nil || stored.Status != model.TaskStatusFailed || stored.BillingOrderID != order.ID || stored.Error != task.Error {
		t.Fatalf("blocked retry changed the task: %+v %v", stored, err)
	}
}

func TestTaskCancellationPersistsFirstSourceAndDiagnostic(t *testing.T) {
	s, db, _ := agentMediaFixture(t)
	task := model.Task{ID: "cancel-child", UserID: "user", AgentRunID: "parent", Status: model.TaskStatusQueued}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskLifecycle().cancelTaskWithIntent(context.Background(), "user", task.ID, model.TaskCancellationIntent{Source: model.TaskCancellationParentFailed}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelTask(context.Background(), "user", task.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := s.repo.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := taskExecutionDiagnostic(stored)
	if stored.CancellationSource != model.TaskCancellationParentFailed || stored.CancellationRequestedAt == nil || diagnostic == nil || diagnostic.Phase != "cancellation" || diagnostic.SubmissionOutcome != "not_submitted" || diagnostic.TaskSubmitted {
		t.Fatalf("cancellation lost facts: %+v %+v", stored, diagnostic)
	}
}
