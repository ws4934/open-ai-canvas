package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"yingce/backend/internal/buildinfo"
	"yingce/backend/internal/model"
	"yingce/backend/internal/programupdate"
)

type UpdateManager interface {
	Status(context.Context) (programupdate.Status, error)
	Check(context.Context, bool) (programupdate.Status, error)
	Start(context.Context, string) (programupdate.Status, error)
	Rollback(context.Context, string) (programupdate.Status, error)
}

func (s *Service) ConfigureUpdateManager(manager UpdateManager) {
	s.updateManager = manager
}

func (s *Service) AdminUpdateStatus(ctx context.Context, actor *model.User) (programupdate.Status, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return programupdate.Status{}, err
	}
	if s.updateManager == nil {
		return unsupportedUpdateStatus("当前部署未启用程序文件更新"), nil
	}
	status, err := s.updateManager.Status(ctx)
	if err != nil {
		return status, WrapAppError(http.StatusServiceUnavailable, "读取程序更新状态失败", err)
	}
	return status, nil
}

func (s *Service) AdminCheckUpdate(ctx context.Context, actor *model.User, force bool) (programupdate.Status, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return programupdate.Status{}, err
	}
	if s.updateManager == nil {
		return programupdate.Status{}, NewAppError(http.StatusServiceUnavailable, "当前部署未配置版本检查")
	}
	status, err := s.updateManager.Check(ctx, force)
	if err != nil {
		if errors.Is(err, programupdate.ErrNoInstallableRelease) {
			return status, NewAppError(http.StatusBadGateway, programupdate.ErrNoInstallableRelease.Error())
		}
		return status, WrapAppError(http.StatusBadGateway, "检查更新失败，请稍后重试", err)
	}
	return status, nil
}

func (s *Service) AdminStartUpdate(ctx context.Context, actor *model.User, targetVersion string) (programupdate.Status, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return programupdate.Status{}, err
	}
	if s.updateManager == nil {
		return programupdate.Status{}, NewAppError(http.StatusServiceUnavailable, "当前部署未启用程序文件更新")
	}
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" {
		return programupdate.Status{}, NewAppError(http.StatusBadRequest, "目标版本不能为空")
	}
	status, err := s.updateManager.Start(ctx, targetVersion)
	if err != nil {
		return status, WrapAppError(http.StatusConflict, "无法开始更新，请刷新状态后重试", err)
	}
	return status, nil
}

func (s *Service) AdminRollbackUpdate(ctx context.Context, actor *model.User, reason string) (programupdate.Status, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return programupdate.Status{}, err
	}
	if s.updateManager == nil {
		return programupdate.Status{}, NewAppError(http.StatusServiceUnavailable, "当前部署未启用程序文件更新")
	}
	if strings.TrimSpace(reason) == "" {
		return programupdate.Status{}, NewAppError(http.StatusBadRequest, "请填写回退原因")
	}
	status, err := s.updateManager.Rollback(ctx, reason)
	if err != nil {
		return status, WrapAppError(http.StatusConflict, "无法开始回退，请检查备份和当前状态", err)
	}
	return status, nil
}

func unsupportedUpdateStatus(detail string) programupdate.Status {
	return programupdate.Status{
		Supported: false, UnsupportedReason: detail, CurrentVersion: buildinfo.Current().Version,
		Repository: programupdate.Repository,
		Operation:  programupdate.Operation{Phase: programupdate.PhaseIdle, Logs: []programupdate.LogEntry{}},
	}
}
