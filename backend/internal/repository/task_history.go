package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yingce/backend/internal/model"
)

type TaskHistorySkipped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type TaskHistoryClearResult struct {
	ClearedIDs []string             `json:"clearedIds"`
	Skipped    []TaskHistorySkipped `json:"skipped"`
}

// 清理只改变用户历史可见性，保留任务、产物、订单和审计供其他业务引用。
func (r *Repository) ClearTaskHistory(userID string, ids []string) (*TaskHistoryClearResult, error) {
	result := &TaskHistoryClearResult{ClearedIDs: []string{}, Skipped: []TaskHistorySkipped{}}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var tasks []model.Task
		query := tx.Select("id", "status").Where("user_id = ? AND id IN ?", userID, ids).Order("id")
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&tasks).Error; err != nil {
			return err
		}
		byID := make(map[string]model.TaskStatus, len(tasks))
		for _, task := range tasks {
			byID[task.ID] = task.Status
		}
		for _, id := range ids {
			status, found := byID[id]
			if !found {
				result.Skipped = append(result.Skipped, TaskHistorySkipped{ID: id, Reason: "not_found"})
			} else if status != model.TaskStatusSucceeded && status != model.TaskStatusFailed && status != model.TaskStatusCancelled {
				result.Skipped = append(result.Skipped, TaskHistorySkipped{ID: id, Reason: "active"})
			} else {
				result.ClearedIDs = append(result.ClearedIDs, id)
			}
		}
		if len(result.ClearedIDs) == 0 {
			return nil
		}
		// UpdateColumn 不改执行时间；锁行与重试事务互斥，不能把刚重试的活动任务隐藏。
		return tx.Model(&model.Task{}).Where("user_id = ? AND id IN ? AND history_hidden_at IS NULL", userID, result.ClearedIDs).
			UpdateColumn("history_hidden_at", time.Now()).Error
	})
	return result, err
}
