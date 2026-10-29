package app

import (
	"strings"

	"yingce/backend/internal/repository"
)

type TaskHistoryClearRequest struct {
	IDs []string `json:"ids"`
}

type TaskHistoryClearResult = repository.TaskHistoryClearResult

func (s *Service) ClearTaskHistory(userID string, request TaskHistoryClearRequest) (*TaskHistoryClearResult, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, Unauthorized("请先登录")
	}
	if len(request.IDs) == 0 || len(request.IDs) > 200 {
		return nil, BadAuthRequest("请选择 1 至 200 条任务记录")
	}
	ids := make([]string, 0, len(request.IDs))
	seen := make(map[string]bool, len(request.IDs))
	for _, value := range request.IDs {
		id := strings.TrimSpace(value)
		if id == "" || len(id) > 36 {
			return nil, BadAuthRequest("任务 ID 无效")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return s.repo.ClearTaskHistory(userID, ids)
}
