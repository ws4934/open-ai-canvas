package repository

import (
	"strings"

	"yingce/backend/internal/model"
)

type WorkbenchTaskPage struct {
	Tasks    []model.Task
	Total    int64
	Page     int
	PageSize int
	HasMore  bool
}

func (r *Repository) WorkbenchTasks(userID, kind string, page, pageSize int, activeOnly bool, search string) (WorkbenchTaskPage, error) {
	page = max(1, page)
	if pageSize < 1 {
		pageSize = 20
	}
	pageSize = min(100, pageSize)
	result := WorkbenchTaskPage{Tasks: []model.Task{}, Page: page, PageSize: pageSize}
	if kind != model.WorkbenchKindImage && kind != model.WorkbenchKindVideo {
		return result, nil
	}
	query := r.taskListQuery(userID, "", kind)
	if activeOnly {
		query = query.Where("status IN ?", []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning})
	} else {
		query = query.Where("status IN ?", []model.TaskStatus{model.TaskStatusSucceeded, model.TaskStatusFailed, model.TaskStatusCancelled})
	}
	if search = strings.TrimSpace(search); search != "" {
		query = query.Where("LOWER(prompt) LIKE ? ESCAPE ?", "%"+strings.ToLower(escapeLike(search))+"%", `\`)
	}
	if err := query.Count(&result.Total).Error; err != nil {
		return result, err
	}
	lastPage := max(1, int((result.Total+int64(pageSize)-1)/int64(pageSize)))
	result.Page = min(page, lastPage)
	if err := query.Select(taskListColumns).Order("created_at desc, id desc").Offset((result.Page - 1) * pageSize).Limit(pageSize).Find(&result.Tasks).Error; err != nil {
		return result, err
	}
	result.HasMore = int64(result.Page*pageSize) < result.Total
	return result, nil
}
