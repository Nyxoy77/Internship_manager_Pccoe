package service

import (
	"strings"

	"github.com/jmoiron/sqlx"
)

func logInternshipAudit(exec sqlx.Ext, internshipID int, action string, note string, actorUserID *int) error {
	action = strings.TrimSpace(action)
	note = strings.TrimSpace(note)
	if action == "" {
		return nil
	}
	_, err := exec.Exec(`
		INSERT INTO internship_audit_logs (internship_id, action, note, actor_user_id, actor_role)
		SELECT
			$1,
			$2,
			NULLIF($3, ''),
			u.id,
			u.role
		FROM (SELECT 1) AS seed
		LEFT JOIN users u ON u.id = $4
	`, internshipID, action, note, actorUserID)
	return err
}
