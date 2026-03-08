ALTER TABLE internships
ADD COLUMN IF NOT EXISTS workflow_status VARCHAR(30) NOT NULL DEFAULT 'certificate_pending';

ALTER TABLE internships
DROP CONSTRAINT IF EXISTS internships_workflow_status_check;

ALTER TABLE internships
ADD CONSTRAINT internships_workflow_status_check
CHECK (workflow_status IN ('certificate_pending', 'certificate_uploaded', 'pending_review', 'approved', 'rejected'));

UPDATE internships i
SET workflow_status = CASE
  WHEN i.status = 'approved' THEN 'approved'
  WHEN i.status = 'rejected' THEN 'rejected'
  WHEN EXISTS (SELECT 1 FROM certificates c WHERE c.internship_id = i.id) THEN 'certificate_uploaded'
  ELSE 'certificate_pending'
END;

CREATE TABLE IF NOT EXISTS internship_audit_logs (
  id SERIAL PRIMARY KEY,
  internship_id INT NOT NULL REFERENCES internships(id) ON DELETE CASCADE,
  action VARCHAR(50) NOT NULL,
  note TEXT,
  actor_user_id INT REFERENCES users(id),
  actor_role VARCHAR(20),
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_internship_audit_logs_internship_id
ON internship_audit_logs(internship_id, created_at DESC);
