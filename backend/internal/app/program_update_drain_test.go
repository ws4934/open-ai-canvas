package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/programupdate"
	"yingce/backend/internal/repository"
)

func TestProgramUpdateDrainWaitsForIntakeWithoutStoppingWorkers(t *testing.T) {
	db := newSQLiteTestDB(t)
	if err := db.AutoMigrate(&model.Task{}, &model.CloudAgentExecution{}); err != nil {
		t.Fatal(err)
	}
	s := &Service{repo: repository.New(db)}
	s.backgroundWorkers().Start()
	t.Cleanup(func() { _ = s.StopWorker(context.Background()) })
	release, err := s.AcquireProgramUpdateIntake()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.DrainForProgramUpdate(ctx) }()
	deadline := time.Now().Add(time.Second)
	for !s.ProgramUpdateDraining() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if _, err := s.AcquireProgramUpdateIntake(); err == nil {
		t.Fatal("accepted new intake while draining")
	}
	if s.IsDraining() {
		t.Fatal("update intake fence stopped worker dispatch")
	}
	select {
	case err := <-done:
		t.Fatalf("drained with active request: %v", err)
	default:
	}
	release()
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !s.ProgramUpdateDraining() {
		t.Fatal("fence released before restart")
	}
	s.ResumeAfterProgramUpdate()
	release, err = s.AcquireProgramUpdateIntake()
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestProgramUpdateDrainCountsLogicalAgentsAndResumesOnTimeout(t *testing.T) {
	db := newSQLiteTestDB(t)
	if err := db.AutoMigrate(&model.Task{}, &model.CloudAgentExecution{}); err != nil {
		t.Fatal(err)
	}
	for _, task := range []model.Task{
		{ID: "completed-root", UserID: "user", Operation: "cloud_agent", Status: model.TaskStatusTextReplay},
		{ID: "approval-root", UserID: "user", Operation: "cloud_agent", Status: model.TaskStatusTextReplay},
		{ID: "ordinary", UserID: "user", Status: model.TaskStatusQueued},
	} {
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, agent := range []model.CloudAgentExecution{
		{ID: "completed-root", UserID: "user", Status: "completed"},
		{ID: "approval-root", UserID: "user", Status: "waiting_approval"},
		{ID: "input-root", UserID: "user", Status: "waiting_user"},
	} {
		if err := db.Create(&agent).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{repo: repository.New(db)}
	activity, err := s.repo.ProgramUpdateActivity()
	if err != nil || activity.Tasks != 1 || activity.Agents != 2 || activity.WaitingApproval != 1 || activity.WaitingUser != 1 {
		t.Fatalf("activity=%+v err=%v", activity, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.DrainForProgramUpdate(ctx); err == nil {
		t.Fatal("drained waiting approval/input")
	}
	if s.ProgramUpdateDraining() {
		t.Fatal("timeout left intake closed")
	}
	if err := db.Model(&model.Task{}).Where("id = ?", "ordinary").Update("status", model.TaskStatusSucceeded).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.CloudAgentExecution{}).Where("id IN ?", []string{"approval-root", "input-root"}).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DrainForProgramUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProgramUpdateDrainAllowsExistingAgentTaskAdmission(t *testing.T) {
	s, db, args := agentMediaFixture(t)
	run, state := agentMediaRun(t, s, args, "request_approval")
	s.updateIntakeFenced = true
	_, err := s.CreateTask("user", creationTextRequest())
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 503 {
		t.Fatalf("external request not fenced: %v", err)
	}
	_, err = s.CreateCloudAgentRun("user", CloudAgentRequest{}, "")
	if !errors.As(err, &appErr) || appErr.Status != 503 {
		t.Fatalf("new Agent not fenced: %v", err)
	}
	if err := s.advanceCloudAgentTool(run, &state); err != nil {
		t.Fatalf("existing Agent could not request approval: %v", err)
	}
	waiting, err := s.CloudAgentRun("user", run.ID)
	if err != nil || waiting.Status != "waiting_approval" || waiting.Approval == nil {
		t.Fatalf("existing Agent approval = %#v, %v", waiting, err)
	}
	var count int64
	if err := db.Model(&model.Task{}).Where("type = ?", "canvas_video").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("media submitted before approval: count=%d error=%v", count, err)
	}
	seconds := 8
	settings := &CloudAgentMediaSettings{
		ChannelID: args.ChannelID, ChannelModelKey: args.ChannelModelKey,
		Size: args.Size, DurationSeconds: &seconds, VideoGenerateAudio: args.VideoGenerateAudio,
	}
	if err := s.DecideCloudAgentApproval("user", run.ID, waiting.Approval.ID, "approve", "", settings); err != nil {
		t.Fatalf("existing approval edit was blocked: %v", err)
	}
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatalf("approved internal task could not advance: %v", err)
	}
	run, err = s.repo.CloudAgent("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err = cloudAgentDecode(run)
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.repo.TaskForUser("user", state.MediaTaskID)
	if err != nil || task.Status != model.TaskStatusQueued || task.AgentRunID != run.ID || task.ApprovalID != waiting.Approval.ID || task.GenerationID == "" || task.BillingOrderID == "" {
		t.Fatalf("approved internal task admission = %#v, %v", task, err)
	}
	var leases []model.CloudAgentResourceLease
	if err := db.Where("user_id = ? AND owner_id IN ?", "user", []string{waiting.Approval.ID, "task:" + task.ID}).Find(&leases).Error; err != nil || len(leases) == 0 {
		t.Fatalf("approved task lost its resource leases: %#v, %v", leases, err)
	}
	for _, lease := range leases {
		if lease.OwnerID != "task:"+task.ID || lease.RunID != task.ID {
			t.Fatalf("approval resource lease was not transferred to the task: %#v", lease)
		}
	}
	var input canvasGenerationInput
	if err := json.Unmarshal([]byte(task.InputJSON), &input); err != nil || input.Config.VideoSeconds != "8" {
		t.Fatalf("edited approval options were not applied: %#v, %v", input.Config, err)
	}
	if err := db.Create(&model.Resource{ID: "drain-output", UserID: "user", Kind: "video", Status: model.ResourceStatusReady, MimeType: "video/mp4"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "result_json": `{"mode":"video","video":{"storageKey":"resource:drain-output"}}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatalf("existing Agent could not consume the completed task: %v", err)
	}
	canvas, err := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if err != nil {
		t.Fatal(err)
	}
	doc := mustCreationDocument(t, canvas.PayloadJSON)
	node := cloudAgentNodeByID(creationMaps(doc["nodes"]), args.NodeID)
	metadata, _ := node["metadata"].(map[string]any)
	if metadata["status"] != "success" || metadata["storageKey"] != "resource:drain-output" {
		t.Fatalf("completed media was not written back while draining: %#v", node)
	}
	if !s.ProgramUpdateDraining() {
		t.Fatal("internal continuation reopened external intake")
	}
}

func TestProgramUpdateDrainRejectsUnregisteredAndTerminalAgentPreparations(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	for _, run := range []model.CloudAgentExecution{
		{ID: "active", UserID: "user", CanvasID: "canvas", Status: "running"},
		{ID: "foreign", UserID: "other", CanvasID: "canvas", Status: "running"},
		{ID: "completed", UserID: "user", CanvasID: "canvas", Status: "completed"},
		{ID: "cancelled", UserID: "user", CanvasID: "canvas", Status: "cancelled", CleanupPending: true},
	} {
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	s.updateIntakeFenced = true
	for _, tc := range []struct{ name, runID, canvasID string }{
		{"ordinary preparation", "", "canvas"},
		{"missing run", "missing", "canvas"},
		{"foreign run", "foreign", "canvas"},
		{"completed run", "completed", "canvas"},
		{"cancelled cleanup", "cancelled", "canvas"},
		{"different canvas", "active", "other-canvas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := creationTextRequest()
			req.ProjectID = tc.canvasID
			req.creationPrepare = &creationTaskPreparation{agentRunID: tc.runID}
			if _, err := s.CreateTask("user", req); !isAppErrorStatus(err, 503) {
				t.Fatalf("untrusted preparation bypassed update intake: %v", err)
			}
		})
	}
	t.Run("active preparation still validates input", func(t *testing.T) {
		req := creationTextRequest()
		req.ProjectID, req.Prompt = "canvas", ""
		req.creationPrepare = &creationTaskPreparation{agentRunID: "active"}
		if task, err := s.CreateTask("user", req); err == nil || isAppErrorStatus(err, 503) || task != nil {
			t.Fatalf("expected normal admission validation for active Agent: task=%#v error=%v", task, err)
		}
	})
}

type updateManagerStub struct {
	calls      int
	force      bool
	checkError error
}

func (m *updateManagerStub) Status(context.Context) (programupdate.Status, error) {
	m.calls++
	return programupdate.Status{}, nil
}
func (m *updateManagerStub) Check(_ context.Context, force bool) (programupdate.Status, error) {
	m.calls++
	m.force = force
	return programupdate.Status{}, m.checkError
}
func (m *updateManagerStub) Start(context.Context, string) (programupdate.Status, error) {
	m.calls++
	return programupdate.Status{}, nil
}
func (m *updateManagerStub) Rollback(context.Context, string) (programupdate.Status, error) {
	m.calls++
	return programupdate.Status{}, nil
}

func TestProgramUpdateAdminGateAndForce(t *testing.T) {
	manager := &updateManagerStub{}
	s := &Service{updateManager: manager}
	for _, actor := range []*model.User{nil, {Role: model.UserRoleUser}} {
		if _, err := s.AdminUpdateStatus(context.Background(), actor); err == nil {
			t.Fatal("status authorized")
		}
		if _, err := s.AdminCheckUpdate(context.Background(), actor, true); err == nil {
			t.Fatal("check authorized")
		}
		if _, err := s.AdminStartUpdate(context.Background(), actor, "v1.2.3.4"); err == nil {
			t.Fatal("start authorized")
		}
		if _, err := s.AdminRollbackUpdate(context.Background(), actor, "test"); err == nil {
			t.Fatal("rollback authorized")
		}
	}
	if manager.calls != 0 {
		t.Fatal("unprivileged request reached updater")
	}
	if _, err := s.AdminCheckUpdate(context.Background(), &model.User{Role: model.UserRoleAdmin}, true); err != nil || !manager.force {
		t.Fatal("forced check not forwarded", err)
	}
}

func TestProgramUpdateAdminReportsMissingReleaseArtifacts(t *testing.T) {
	manager := &updateManagerStub{checkError: programupdate.ErrNoInstallableRelease}
	s := &Service{updateManager: manager}
	_, err := s.AdminCheckUpdate(context.Background(), &model.User{Role: model.UserRoleAdmin}, true)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Message != "尚无可安装的程序 Release" {
		t.Fatalf("missing installable release explanation: %v", err)
	}
}
