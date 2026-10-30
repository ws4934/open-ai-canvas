package app

import (
	"context"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestCloudAgentCleanupRetriesStaleRevision(t *testing.T) {
	s, db, args := agentMediaFixture(t)
	s.closeApprovedCloudAgentMediaWaiters()
	run, _ := agentMediaRun(t, s, args, "request_approval")
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatal(err)
	}
	approveAgentMediaDraft(t, s, run.ID)
	run, state := agentInterjectionState(t, s, run.ID)
	mediaID := state.MediaTaskID
	if mediaID == "" {
		t.Fatal("approved media task missing")
	}
	if err := s.repo.MutateCloudAgent("user", run.ID, run.Revision, func(current *model.CloudAgentExecution, _ *repository.Repository) error {
		current.Status, current.CleanupPending = "cancelled", true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	stale, _ := agentInterjectionState(t, s, run.ID)
	// A media waiter or runtime event can advance the revision after the
	// cancellation intent is committed but before its cleanup writes back.
	if err := s.repo.MutateCloudAgent("user", run.ID, stale.Revision, func(*model.CloudAgentExecution, *repository.Repository) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.finishCloudAgentCleanup(context.Background(), stale); err != nil {
			t.Fatal(err)
		}
	}
	run, state = agentInterjectionState(t, s, run.ID)
	if run.Status != "cancelled" || run.CleanupPending || state.MediaTaskID != "" {
		t.Fatalf("cleanup incomplete: status=%s pending=%v media=%s", run.Status, run.CleanupPending, state.MediaTaskID)
	}
	task, err := s.repo.TaskForUser("user", mediaID)
	if err != nil || task.Status != model.TaskStatusCancelled {
		t.Fatalf("child not cancelled: task=%+v err=%v", task, err)
	}
	canvas, err := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := creationDocument(canvas.PayloadJSON)
	nodes, _ := creationObjects(doc["nodes"])
	if nodes[args.NodeID]["metadata"].(map[string]any)["status"] != "error" {
		t.Fatal("cancelled node remained loading")
	}
	results := 0
	for _, event := range state.Events {
		if event.Type == "tool_failed" {
			results++
		}
	}
	var tasks int64
	if err := db.Model(&model.Task{}).Where("type = ?", "canvas_video").Count(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if results != 1 || tasks != 1 {
		t.Fatalf("cleanup replay duplicated work: results=%d tasks=%d", results, tasks)
	}
}
