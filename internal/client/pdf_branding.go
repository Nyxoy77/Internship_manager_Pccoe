package client

import (
	"os"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

func drawReportHeader(pdf *fpdf.Fpdf, title string) {
	logoFound := false
	logoPaths := []string{}
	if envPath := strings.TrimSpace(os.Getenv("REPORT_LOGO_PATH")); envPath != "" {
		logoPaths = append(logoPaths, envPath)
	}
	logoPaths = append(logoPaths,
		"assets/pccoe-logo.png",
		"./assets/pccoe-logo.png",
		"../assets/pccoe-logo.png",
		"backend/assets/pccoe-logo.png",
	)

	for _, path := range logoPaths {
		if _, err := os.Stat(path); err == nil {
			opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
			pdf.ImageOptions(path, 10, 8, 18, 18, false, opts, 0, "")
			logoFound = true
			break
		}
	}

	leftX := 10.0
	if logoFound {
		leftX = 32.0
	}

	pdf.SetXY(leftX, 10)
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	pdf.SetX(leftX)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Generated: "+time.Now().Format("2006-01-02 15:04:05"), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}
