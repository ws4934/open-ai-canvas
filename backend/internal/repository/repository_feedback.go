package repository

import (
	"encoding/json"
	"strings"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FeedbackFilter struct {
	Type     string
	Status   string
	Keyword  string
	Archived *bool
	Page     int
	Limit    int
}

func (r *Repository) CreateFeedback(fb *model.UserFeedback) error {
	var attachments []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(fb.Attachments), &attachments); err != nil {
		return err
	}
	ids := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		ids = append(ids, attachment.ID)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(ids) > 0 {
			// Serialize attachment publication with resource cleanup/deletion.
			var resources []model.Resource
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ? AND user_id = ? AND status = ? AND kind IN ?",
				ids, fb.UserID, model.ResourceStatusReady, []string{"image", "video"}).Order("id").Find(&resources).Error; err != nil {
				return err
			}
			if len(resources) != len(ids) {
				return gorm.ErrRecordNotFound
			}
		}
		return tx.Create(fb).Error
	})
}

// FeedbackResourceReferences includes archived feedback until it is explicitly deleted.
func (r *Repository) FeedbackResourceReferences(userID string) ([]ResourceDirectReference, error) {
	var feedbacks []model.UserFeedback
	if err := r.db.Select("id", "title", "attachments").Where("user_id = ?", userID).Find(&feedbacks).Error; err != nil {
		return nil, err
	}
	var references []ResourceDirectReference
	for _, feedback := range feedbacks {
		var attachments []model.FeedbackAttachment
		if err := json.Unmarshal([]byte(feedback.Attachments), &attachments); err != nil {
			return nil, err
		}
		for _, attachment := range attachments {
			references = append(references, ResourceDirectReference{Kind: "反馈", ID: feedback.ID, Title: feedback.Title, ResourceID: attachment.ID})
		}
	}
	return references, nil
}

func (r *Repository) requireNoFeedbackResourceReferences(userID string, resourceIDs []string) error {
	if len(resourceIDs) == 0 {
		return nil
	}
	references, err := r.FeedbackResourceReferences(userID)
	if err != nil {
		return err
	}
	ids := make(map[string]bool, len(resourceIDs))
	for _, id := range resourceIDs {
		ids[id] = true
	}
	for _, reference := range references {
		if ids[reference.ResourceID] {
			return ErrResourceCleanupStillReferenced
		}
	}
	return nil
}

func (r *Repository) ListFeedbacksByUser(userID string) ([]model.UserFeedback, error) {
	var list []model.UserFeedback
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *Repository) GetFeedback(id string) (*model.UserFeedback, error) {
	var fb model.UserFeedback
	err := r.db.Where("id = ?", id).First(&fb).Error
	if err != nil {
		return nil, err
	}
	return &fb, nil
}

func (r *Repository) ListAllFeedbacks(filter FeedbackFilter) ([]model.UserFeedback, int64, error) {
	query := r.db.Model(&model.UserFeedback{})

	if filter.Type != "" && filter.Type != "all" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status == "archived" {
		query = query.Where("archived = ?", true)
	} else if filter.Status != "" && filter.Status != "all" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Archived != nil {
		query = query.Where("archived = ?", *filter.Archived)
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("title LIKE ? OR content LIKE ? OR user_name LIKE ? OR contact LIKE ?", like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var list []model.UserFeedback
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *Repository) UpdateFeedback(fb *model.UserFeedback) error {
	return r.db.Save(fb).Error
}

func (r *Repository) DeleteFeedback(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.UserFeedback{}).Error
}
