\set ON_ERROR_STOP on

BEGIN;

CREATE TEMP TABLE student_import_staging (
    prn text,
    name text,
    passing_year integer,
    division text,
    guide_name text,
    organization_email text
);

\copy student_import_staging (prn, name, passing_year, division, guide_name, organization_email) FROM '/tmp/SY_BTech_Student_Table.csv' WITH (FORMAT csv, HEADER true);

\copy student_import_staging (prn, name, passing_year, division, guide_name, organization_email) FROM '/tmp/TY_BTech_Student_Table.csv' WITH (FORMAT csv, HEADER true);

INSERT INTO students (prn, name, guide_name, passing_year, division)
SELECT
    TRIM(prn),
    TRIM(name),
    TRIM(COALESCE(guide_name, '')),
    passing_year,
    UPPER(TRIM(division))
FROM student_import_staging
WHERE TRIM(COALESCE(prn, '')) <> ''
ON CONFLICT (prn) DO UPDATE
SET
    name = EXCLUDED.name,
    guide_name = CASE
        WHEN EXCLUDED.guide_name <> '' THEN EXCLUDED.guide_name
        ELSE students.guide_name
    END,
    passing_year = EXCLUDED.passing_year,
    division = EXCLUDED.division;

COMMIT;

SELECT
    COUNT(*) AS total_students,
    COUNT(*) FILTER (WHERE guide_name = '') AS students_without_guides
FROM students;

SELECT
    passing_year,
    division,
    COUNT(*) AS student_count
FROM students
GROUP BY passing_year, division
ORDER BY passing_year, division;
