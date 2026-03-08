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

		year, _ := strconv.Atoi(safeCell(row, 2))

		students = append(students, models.CreateStudentRequest{
			PRN:         safeCell(row, 0),
			Name:        safeCell(row, 1),
			GuideName:   safeCell(row, 4),
			PassingYear: year,
			Division:    safeCell(row, 3),
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

		year, _ := strconv.Atoi(safeCell(row, 2))

		students = append(students, models.CreateStudentRequest{
			PRN:         safeCell(row, 0),
			Name:        safeCell(row, 1),
			GuideName:   safeCell(row, 4),
			PassingYear: year,
			Division:    safeCell(row, 3),
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
