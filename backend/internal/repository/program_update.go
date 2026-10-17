package repository

type ProgramUpdateActivity struct {
	Tasks           int64 `json:"tasks"`
	Agents          int64 `json:"agents"`
	WaitingApproval int64 `json:"waitingApproval"`
	WaitingUser     int64 `json:"waitingUser"`
}

func (a ProgramUpdateActivity) Active() bool { return a.Tasks != 0 || a.Agents != 0 }

// Completed Agent roots keep text_replay as their task status; their execution
// checkpoint, rather than that task row, determines whether they still run.
func (r *Repository) ProgramUpdateActivity() (ProgramUpdateActivity, error) {
	var activity ProgramUpdateActivity
	// A single statement sees parent execution and derived task in one snapshot.
	err := r.db.Raw(`SELECT
 (SELECT COUNT(*) FROM tasks t WHERE t.status NOT IN ('succeeded','failed','cancelled')
  AND (t.operation <> 'cloud_agent' OR NOT EXISTS
   (SELECT 1 FROM cloud_agent_executions e WHERE e.id = t.id AND e.user_id = t.user_id))) AS tasks,
 (SELECT COUNT(*) FROM cloud_agent_executions WHERE status NOT IN ('completed','failed','cancelled','rejected') OR cleanup_pending = true) AS agents,
 (SELECT COUNT(*) FROM cloud_agent_executions WHERE status = 'waiting_approval') AS waiting_approval,
 (SELECT COUNT(*) FROM cloud_agent_executions WHERE status = 'waiting_user') AS waiting_user`).Scan(&activity).Error
	return activity, err
}
