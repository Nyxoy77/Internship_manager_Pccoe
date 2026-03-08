package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListInternships_InvalidYear_ReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewInternshipHandler(nil)
	router := gin.New()
	router.GET("/internships", handler.ListInternships)

	req := httptest.NewRequest(http.MethodGet, "/internships?year=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
