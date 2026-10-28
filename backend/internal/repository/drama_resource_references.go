package repository

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
