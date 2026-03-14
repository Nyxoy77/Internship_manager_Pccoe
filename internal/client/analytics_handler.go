package client

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yourusername/student-internship-manager/internal/service"
)

type AnalyticsHandler struct {
	service *service.AnalyticsService
}

func NewAnalyticsHandler(s *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: s}
}

/* -----------------------------
   1️⃣ Avg stipend
   GET /analytics/avg-stipend
--------------------------------*/

func (h *AnalyticsHandler) AvgStipend(c *gin.Context) {

	var (
		year     *int
		division *string
	)

	if y := strings.TrimSpace(c.Query("year")); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		year = &parsed
	}

	if d := strings.TrimSpace(c.Query("division")); d != "" {
		division = &d
	}

	data, err := h.service.GetAvgStipendByYearDivision(year, division)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

/* -----------------------------
   2️⃣ Paid internship %
   GET /analytics/paid-percentage
--------------------------------*/

func (h *AnalyticsHandler) PaidPercentage(c *gin.Context) {
	var (
		year     *int
		division *string
	)

	if y := strings.TrimSpace(c.Query("year")); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		year = &parsed
	}

	if d := strings.TrimSpace(c.Query("division")); d != "" {
		division = &d
	}
	data, err := h.service.GetPaidInternshipPercentage(year, division)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

/* -----------------------------
   3️⃣ Mode-wise distribution
   GET /analytics/mode-distribution
--------------------------------*/

func (h *AnalyticsHandler) ModeDistribution(c *gin.Context) {
	var (
		year     *int
		division *string
	)

	if y := strings.TrimSpace(c.Query("year")); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		year = &parsed
	}

	if d := strings.TrimSpace(c.Query("division")); d != "" {
		division = &d
	}
	data, err := h.service.GetModeWiseDistribution(year, division)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}
