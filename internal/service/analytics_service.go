package service

import (
	"strconv"

	"github.com/jmoiron/sqlx"
)

type AnalyticsService struct {
	db *sqlx.DB
}

func NewAnalyticsService(db *sqlx.DB) *AnalyticsService {
	return &AnalyticsService{db: db}
}

type AvgStipendRow struct {
	PassingYear int     `db:"passing_year" json:"passing_year"`
	Division    string  `db:"division" json:"division"`
	AvgStipend  float64 `db:"avg_stipend" json:"avg_stipend"`
}

func (s *AnalyticsService) GetAvgStipendByYearDivision(
	year *int,
	division *string,
) ([]AvgStipendRow, error) {

	query := `
		SELECT
			s.passing_year,
			s.division,
			AVG(i.monthly_stipend) AS avg_stipend
		FROM internships i
		JOIN students s ON s.prn = i.student_prn
		WHERE i.status = 'approved'
	`

	args := []interface{}{}
	argIdx := 1

	if year != nil {
		query += " AND s.passing_year = $" + itoa(argIdx)
		args = append(args, *year)
		argIdx++
	}

	if division != nil {
		query += " AND s.division = $" + itoa(argIdx)
		args = append(args, *division)
		argIdx++
	}

	query += " GROUP BY s.passing_year, s.division ORDER BY s.passing_year, s.division"

	var rows []AvgStipendRow
	err := s.db.Select(&rows, query, args...)
	return rows, err
}

type PaidPercentageRow struct {
	PassingYear int     `db:"passing_year" json:"passing_year"`
	PaidPercent float64 `db:"paid_percentage" json:"paid_percentage"`
}

func (s *AnalyticsService) GetPaidInternshipPercentage(
	year *int,
	division *string,
) ([]PaidPercentageRow, error) {

	query := `
		SELECT
			s.passing_year,
			COUNT(DISTINCT i.student_prn)
				FILTER (WHERE i.monthly_stipend > 0 AND i.status = 'approved')
				* 100.0
			/ COUNT(DISTINCT s.prn) AS paid_percentage
		FROM students s
		LEFT JOIN internships i ON i.student_prn = s.prn
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	if year != nil {
		query += " AND s.passing_year = $" + itoa(argIdx)
		args = append(args, *year)
		argIdx++
	}

	if division != nil {
		query += " AND s.division = $" + itoa(argIdx)
		args = append(args, *division)
		argIdx++
	}

	query += `
		GROUP BY s.passing_year
		ORDER BY s.passing_year
	`

	var rows []PaidPercentageRow
	err := s.db.Select(&rows, query, args...)
	return rows, err
}

type ModeDistributionRow struct {
	Mode  string `db:"mode" json:"mode"`
	Total int    `db:"total" json:"total"`
}

func (s *AnalyticsService) GetModeWiseDistribution(
	year *int,
	division *string,
) ([]ModeDistributionRow, error) {

	query := `
		SELECT
			i.mode,
			COUNT(*) AS total
		FROM internships i
		JOIN students s ON s.prn = i.student_prn
		WHERE i.status = 'approved'
	`

	args := []interface{}{}
	argIdx := 1

	if year != nil {
		query += " AND s.passing_year = $" + itoa(argIdx)
		args = append(args, *year)
		argIdx++
	}

	if division != nil {
		query += " AND s.division = $" + itoa(argIdx)
		args = append(args, *division)
		argIdx++
	}

	query += `
		GROUP BY i.mode
		ORDER BY total DESC
	`

	var rows []ModeDistributionRow
	err := s.db.Select(&rows, query, args...)
	return rows, err
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
