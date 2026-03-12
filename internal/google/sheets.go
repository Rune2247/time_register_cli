package google

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rlf/time_register_cli/internal/models"
	"google.golang.org/api/sheets/v4"
)

type SheetsClient struct {
	srv           *sheets.Service
	spreadsheetID string
}

func NewSheetsClient(ctx context.Context, spreadsheetID string) (*SheetsClient, error) {
	opt, err := Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := sheets.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %w", err)
	}

	return &SheetsClient{srv: srv, spreadsheetID: spreadsheetID}, nil
}

// Sheet columns:
// A=Date, B=Day, C=Type, D=Name, E=Start, F=End, G=Hours, H=Notes

// ensureTab creates a tab if it doesn't exist. Returns true if created.
func (c *SheetsClient) ensureTab(name string) error {
	ss, err := c.srv.Spreadsheets.Get(c.spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("get spreadsheet: %w", err)
	}

	for _, sheet := range ss.Sheets {
		if sheet.Properties.Title == name {
			return nil
		}
	}

	_, err = c.srv.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{Title: name},
			},
		}},
	}).Do()
	if err != nil {
		return fmt.Errorf("create tab %s: %w", name, err)
	}
	return nil
}

// clearTab clears all content from a tab.
func (c *SheetsClient) clearTab(name string) error {
	rangeStr := fmt.Sprintf("'%s'!A:H", name)
	_, err := c.srv.Spreadsheets.Values.Clear(c.spreadsheetID, rangeStr, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return fmt.Errorf("clear tab %s: %w", name, err)
	}
	return nil
}

// WriteMonthTab builds a complete month tab from entries.
// It creates the tab if needed, clears it, and writes all data + formulas.
func (c *SheetsClient) WriteMonthTab(yearMonth string, entries []models.Entry) error {
	if err := c.ensureTab(yearMonth); err != nil {
		return err
	}
	if err := c.clearTab(yearMonth); err != nil {
		return err
	}

	t, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return fmt.Errorf("parse month: %w", err)
	}

	// Group entries by date and sort each day chronologically
	entryMap := map[string][]models.Entry{}
	for _, e := range entries {
		entryMap[e.Date] = append(entryMap[e.Date], e)
	}
	for date := range entryMap {
		sort.Slice(entryMap[date], func(i, j int) bool {
			return padTime(entryMap[date][i].StartTime) < padTime(entryMap[date][j].StartTime)
		})
	}

	var rows [][]interface{}

	// Header
	rows = append(rows, []interface{}{"Date", "Day", "Type", "Name", "Start", "End", "Hours", "Notes"})

	currentRow := 2 // 1-indexed, after header
	dataStartRow := currentRow // will be set after week header
	var weekFormulaRows []int
	needsWeekHeader := true

	daysInMonth := daysIn(t.Year(), t.Month())

	for day := 1; day <= daysInMonth; day++ {
		d := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := d.Weekday().String()[:3]
		dateStr := d.Format("2006-01-02")

		// Week header before the first day of each week
		if needsWeekHeader {
			_, week := d.ISOWeek()
			rows = append(rows, []interface{}{
				fmt.Sprintf("Week %d", week), "", "", "", "", "", "", "",
			})
			currentRow++
			dataStartRow = currentRow
			needsWeekHeader = false
		}

		dayEntries := entryMap[dateStr]

		// Date always gets its own row
		rows = append(rows, []interface{}{dateStr, dayName, "", "", "", "", "", ""})
		currentRow++

		for _, e := range dayEntries {
			entryType := capitalizeType(e.EntryType)
			hours := math.Round(float64(e.DurationMinutes)/60.0*100) / 100

			rows = append(rows, []interface{}{
				"", "", entryType, e.Name, e.StartTime, e.EndTime, hours, e.Notes,
			})
			currentRow++
		}

		// Week summary after Sunday or last day of month
		if d.Weekday() == time.Sunday || day == daysInMonth {
			workFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Assignment")*(G%d:G%d))`,
				dataStartRow, currentRow-1,
				dataStartRow, currentRow-1,
			)
			lunchFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Lunch")*(G%d:G%d))`,
				dataStartRow, currentRow-1,
				dataStartRow, currentRow-1,
			)
			breakFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Break")*(G%d:G%d))`,
				dataStartRow, currentRow-1,
				dataStartRow, currentRow-1,
			)

			// Summary labels + formulas
			rows = append(rows, []interface{}{
				"", "", "", "", "Work", "Lunch", "Break", "",
			})
			currentRow++

			rows = append(rows, []interface{}{
				"", "", "", "", workFormula, lunchFormula, breakFormula, "",
			})
			weekFormulaRows = append(weekFormulaRows, currentRow)
			currentRow++

			// Blank separator
			rows = append(rows, []interface{}{"", "", "", "", "", "", "", ""})
			currentRow++

			needsWeekHeader = true
		}
	}

	// Month total — direct sum of week formula rows
	monthWorkFormula := "=0"
	monthLunchFormula := "=0"
	monthBreakFormula := "=0"
	if len(weekFormulaRows) > 0 {
		monthWorkFormula = "="
		monthLunchFormula = "="
		monthBreakFormula = "="
		for i, row := range weekFormulaRows {
			if i > 0 {
				monthWorkFormula += "+"
				monthLunchFormula += "+"
				monthBreakFormula += "+"
			}
			monthWorkFormula += fmt.Sprintf("E%d", row)
			monthLunchFormula += fmt.Sprintf("F%d", row)
			monthBreakFormula += fmt.Sprintf("G%d", row)
		}
	}

	rows = append(rows, []interface{}{"Month Total", "", "", "", monthWorkFormula, monthLunchFormula, monthBreakFormula, ""})

	// Write everything
	rangeStr := fmt.Sprintf("'%s'!A1", yearMonth)
	_, err = c.srv.Spreadsheets.Values.Update(c.spreadsheetID, rangeStr, &sheets.ValueRange{
		Values: rows,
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("write tab %s: %w", yearMonth, err)
	}

	// Set Notes column (H) to clip so it doesn't overflow into adjacent cells
	sheetID, err := c.getSheetID(yearMonth)
	if err == nil {
		_, _ = c.srv.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{{
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						SheetId:          sheetID,
						StartColumnIndex: 7, // column H (0-indexed)
						EndColumnIndex:   8,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							WrapStrategy: "CLIP",
						},
					},
					Fields: "userEnteredFormat.wrapStrategy",
				},
			}},
		}).Do()
	}

	return nil
}

// getSheetID returns the numeric sheet ID for a tab name.
func (c *SheetsClient) getSheetID(name string) (int64, error) {
	ss, err := c.srv.Spreadsheets.Get(c.spreadsheetID).Do()
	if err != nil {
		return 0, err
	}
	for _, sheet := range ss.Sheets {
		if sheet.Properties.Title == name {
			return sheet.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("sheet %q not found", name)
}

func capitalizeType(t models.EntryType) string {
	s := string(t)
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// padTime normalizes a time like "8:00" to "08:00" for correct sorting.
func padTime(t string) string {
	if len(t) > 0 && len(t) < 5 {
		for i := range t {
			if t[i] == ':' && i < 2 {
				return "0" + t
			}
		}
	}
	return t
}
