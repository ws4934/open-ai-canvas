package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"yingce/backend/internal/kernel"
)

// Intake leases close the race between the last empty-queue observation and a
// request persisting its first task. Internal Agent continuation does not enter
// this gate: its existing execution remains visible until it is terminal.
func (s *Service) AcquireProgramUpdateIntake() (func(), error) {
	s.updateIntakeMu.Lock()
	defer s.updateIntakeMu.Unlock()
	if s.updateIntakeFenced {
		err := kernel.NewAppError(http.StatusServiceUnavailable, "程序正在更新，暂不接受新任务；已有任务、审批和取消仍可继续")
		err.Retryable = true
		return nil, err
	}
	s.updateIntakeActive++
	var once sync.Once
	return func() { once.Do(func() { s.updateIntakeMu.Lock(); s.updateIntakeActive--; s.updateIntakeMu.Unlock() }) }, nil
}

func (s *Service) ProgramUpdateDraining() bool {
	s.updateIntakeMu.Lock()
	defer s.updateIntakeMu.Unlock()
	return s.updateIntakeFenced
}

func (s *Service) ResumeAfterProgramUpdate() {
	s.updateIntakeMu.Lock()
	s.updateIntakeFenced = false
	s.updateIntakeMu.Unlock()
}

func (s *Service) DrainForProgramUpdate(ctx context.Context) (returnErr error) {
	s.updateIntakeMu.Lock()
	s.updateIntakeFenced = true
	s.updateIntakeMu.Unlock()
	defer func() {
		if returnErr != nil {
			s.ResumeAfterProgramUpdate()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		activity, err := s.repo.WithContext(ctx).ProgramUpdateActivity()
		if err != nil {
			return fmt.Errorf("读取待完成任务失败：%w", err)
		}
		s.updateIntakeMu.Lock()
		requests := s.updateIntakeActive
		s.updateIntakeMu.Unlock()
		s.piRunnerMu.Lock()
		runners := len(s.piRunners) + len(s.piRunnerRestarts)
		s.piRunnerMu.Unlock()
		s.approvedMediaMu.Lock()
		mediaWaiters := len(s.approvedMediaWaiters)
		s.approvedMediaMu.Unlock()
		if !activity.Active() && requests == 0 && runners == 0 && mediaWaiters == 0 && s.ActiveWorkerTasks() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("排空超时或已取消：任务 %d，Agent %d（待审批 %d、待输入 %d），请求 %d；已恢复接单", activity.Tasks, activity.Agents, activity.WaitingApproval, activity.WaitingUser, requests)
		case <-ticker.C:
		}
	}
}
