package repository

// DramaResourceReferences keeps active slots and historical takes owned by a project.
func (r *Repository) DramaResourceReferences(userID string, resourceIDs []string) ([]ResourceDirectReference, error) {
	var references []ResourceDirectReference
	for _, field := range []struct{ table, column string }{
		{"drama_casts", "image_resource_id"}, {"drama_segments", "video_resource_id"},
		{"drama_segments", "tail_frame_resource_id"}, {"drama_episodes", "final_resource_id"}, {"drama_takes", "resource_id"},
	} {
		var rows []struct{ ID, Name, ResourceID string }
		query := r.db.Table(field.table+" AS media").Joins("JOIN projects ON projects.id = media.project_id").
			Where("projects.user_id = ? AND media."+field.column+" <> ''", userID)
		if len(resourceIDs) > 0 {
			query = query.Where("media."+field.column+" IN ?", resourceIDs)
		}
		if err := query.Select("media.id, projects.name, media." + field.column + " AS resource_id").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			references = append(references, ResourceDirectReference{Kind: "短剧", ID: row.ID, Title: row.Name, ResourceID: row.ResourceID})
		}
	}
	return references, nil
}

func (r *Repository) requireNoDramaResourceReferences(userID string, resourceIDs []string) error {
	if len(resourceIDs) == 0 {
		return nil
	}
	references, err := r.DramaResourceReferences(userID, resourceIDs)
	if err != nil {
		return err
	}
	if len(references) > 0 {
		return ErrResourceCleanupStillReferenced
	}
	return nil
}
