package service

import (
	"strings"
	"testing"
)

func TestBuildInternshipListWhereClause_WithYearAndDivision(t *testing.T) {
	year := 2001
	whereClause, args, argPos := buildInternshipListWhereClause(
		"approved",
		"pending_review",
		"Acme",
		"Dr. Guide",
		"123B1B001",
		"2026-01-01",
		"2026-12-31",
		&year,
		"A",
	)

	expectedParts := []string{
		"i.status = $1",
		"i.workflow_status = $2",
		"LOWER(i.organization) LIKE LOWER($3)",
		"LOWER(s.guide_name) LIKE LOWER($4)",
		"LOWER(i.student_prn) LIKE LOWER($5)",
		"i.start_date >= $6::date",
		"i.end_date <= $7::date",
		"s.passing_year = $8",
		"LOWER(s.division) = LOWER($9)",
	}
	for _, part := range expectedParts {
		if !strings.Contains(whereClause, part) {
			t.Fatalf("where clause missing %q: %s", part, whereClause)
		}
	}

	if argPos != 10 {
		t.Fatalf("expected argPos 10, got %d", argPos)
	}

	if len(args) != 9 {
		t.Fatalf("expected 9 args, got %d", len(args))
	}

	if got, ok := args[7].(int); !ok || got != 2001 {
		t.Fatalf("expected year arg 2001 at index 7, got %#v", args[7])
	}
	if got, ok := args[8].(string); !ok || got != "A" {
		t.Fatalf("expected division arg A at index 8, got %#v", args[8])
	}
}

func TestBuildInternshipListWhereClause_EmptyFilters(t *testing.T) {
	whereClause, args, argPos := buildInternshipListWhereClause(
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		nil,
		"",
	)

	if whereClause != " WHERE 1=1" {
		t.Fatalf("unexpected where clause: %s", whereClause)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %d", len(args))
	}
	if argPos != 1 {
		t.Fatalf("expected argPos 1, got %d", argPos)
	}
}
