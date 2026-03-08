package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"github.com/yourusername/student-internship-manager/internal/models"
)

func ParseStudentFile(
	reader io.Reader,
	filename string,
) ([]models.CreateStudentRequest, error) {

	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".csv":
		return parseCSV(reader)
	case ".xlsx":
		return parseExcel(reader)
	default:
		return nil, fmt.Errorf("unsupported file format")
	}
}

func parseCSV(r io.Reader) ([]models.CreateStudentRequest, error) {

	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}

	var students []models.CreateStudentRequest

	for i, row := range rows {
		if i == 0 {
			continue // header
		}

		rawYear := safeCell(row, 2)
		year, _ := strconv.Atoi(rawYear)
		rawPRN := safeCell(row, 0)
		rawDivision := safeCell(row, 3)
		rawGuide := safeCell(row, 4)

		students = append(students, models.CreateStudentRequest{
			PRN:         rawPRN,
			Name:        safeCell(row, 1),
			GuideName:   rawGuide,
			PassingYear: year,
			Division:    rawDivision,
			ProcessedRow: len(students) + 1,
			SheetRow:     i + 1,
			RawPRN:       rawPRN,
			RawYear:      rawYear,
			RawDivision:  rawDivision,
			RawGuideName: rawGuide,
		})
	}

	return students, nil
}

func parseExcel(r io.Reader) ([]models.CreateStudentRequest, error) {

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	var students []models.CreateStudentRequest

	for i, row := range rows {
		if i == 0 {
			continue
		}

		rawYear := safeCell(row, 2)
		year, _ := strconv.Atoi(rawYear)
		rawPRN := safeCell(row, 0)
		rawDivision := safeCell(row, 3)
		rawGuide := safeCell(row, 4)

		students = append(students, models.CreateStudentRequest{
			PRN:         rawPRN,
			Name:        safeCell(row, 1),
			GuideName:   rawGuide,
			PassingYear: year,
			Division:    rawDivision,
			ProcessedRow: len(students) + 1,
			SheetRow:     i + 1,
			RawPRN:       rawPRN,
			RawYear:      rawYear,
			RawDivision:  rawDivision,
			RawGuideName: rawGuide,
		})
	}

	return students, nil
}

func safeCell(row []string, idx int) string {
	if idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}
