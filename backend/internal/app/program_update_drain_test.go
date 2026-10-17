package app

import (
	"context"
	"errors"
	"strings"
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
	s := &Service{updateIntakeFenced: true}
	_, err := s.CreateTask("user", CreateTaskRequest{})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 503 {
		t.Fatalf("external request not fenced: %v", err)
	}
	_, err = s.CreateTask("user", CreateTaskRequest{admission: &taskAdmission{AgentRunID: "existing"}})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("internal continuation blocked: %v", err)
	}
	_, err = s.CreateCloudAgentRun("user", CloudAgentRequest{}, "")
	if !errors.As(err, &appErr) || appErr.Status != 503 {
		t.Fatalf("new Agent not fenced: %v", err)
	}
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
