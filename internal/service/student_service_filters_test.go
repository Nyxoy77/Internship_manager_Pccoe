package service

import (
	"strings"
	"testing"
)

func TestBuildStudentListWhereClause_AllFilters(t *testing.T) {
	year := 2026
	whereClause, args, argPos := buildStudentListWhereClause(&year, "A", "123B", "shivam", "mentor")

	expectedParts := []string{
		"s.passing_year = $1",
		"LOWER(s.division) = LOWER($2)",
		"LOWER(s.prn) LIKE LOWER($3)",
		"LOWER(s.name) LIKE LOWER($4)",
		"LOWER(s.guide_name) LIKE LOWER($5)",
	}
	for _, part := range expectedParts {
		if !strings.Contains(whereClause, part) {
			t.Fatalf("where clause missing %q: %s", part, whereClause)
		}
	}

	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
	if argPos != 6 {
		t.Fatalf("expected argPos 6, got %d", argPos)
	}
}

func TestBuildStudentListWhereClause_EmptyFilters(t *testing.T) {
	whereClause, args, argPos := buildStudentListWhereClause(nil, "", "", "", "")
	if whereClause != "" {
		t.Fatalf("expected empty where clause, got %s", whereClause)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %d", len(args))
	}
	if argPos != 1 {
		t.Fatalf("expected argPos 1, got %d", argPos)
	}
}

