package google

import (
	"context"
	"fmt"
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

// EnsureMonthTab creates a tab for the given month if it doesn't exist,
// and populates it with headers, day rows, week summary rows, and formulas.
func (c *SheetsClient) EnsureMonthTab(yearMonth string) error {
	// Check if tab exists
	ss, err := c.srv.Spreadsheets.Get(c.spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("get spreadsheet: %w", err)
	}

	for _, sheet := range ss.Sheets {
		if sheet.Properties.Title == yearMonth {
			return nil // already exists
		}
	}

	// Create tab
	addReq := &sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Title: yearMonth,
			},
		},
	}

	_, err = c.srv.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{addReq},
	}).Do()
	if err != nil {
		return fmt.Errorf("create tab %s: %w", yearMonth, err)
	}

	// Populate structure
	return c.populateMonthTab(yearMonth)
}

func (c *SheetsClient) populateMonthTab(yearMonth string) error {
	t, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return fmt.Errorf("parse month: %w", err)
	}

	var rows [][]interface{}

	// Header row
	rows = append(rows, []interface{}{"Date", "Day", "Assignment", "Start", "End", "Hours"})

	dataStartRow := 2 // 1-indexed, row after header
	currentRow := dataStartRow
	weekStartRow := currentRow

	// Iterate all days in the month
	daysInMonth := daysIn(t.Year(), t.Month())

	for day := 1; day <= daysInMonth; day++ {
		d := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := d.Weekday().String()[:3]
		dateStr := d.Format("2006-01-02")

		rows = append(rows, []interface{}{dateStr, dayName, "", "", "", ""})
		currentRow++

		// After Sunday, insert week summary
		if d.Weekday() == time.Sunday || day == daysInMonth {
			_, week := d.ISOWeek()

			// Work hours formula: SUM of hours column for this week's rows, excluding "Lunch"
			workFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d<>"Lunch")*(C%d:C%d<>"")*(F%d:F%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)
			lunchFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Lunch")*(F%d:F%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)

			rows = append(rows, []interface{}{
				fmt.Sprintf("Week %d", week), "", "", "Work:", workFormula, lunchFormula,
			})

			rows = append(rows, []interface{}{""}) // blank row after week
			currentRow += 2
			weekStartRow = currentRow
		}
	}

	// Month total row
	// Collect all week summary row numbers for summing
	monthWorkFormula := fmt.Sprintf(
		`=SUMPRODUCT((LEFT(A%d:A%d,4)="Week")*(E%d:E%d))`,
		dataStartRow, currentRow-1,
		dataStartRow, currentRow-1,
	)
	monthLunchFormula := fmt.Sprintf(
		`=SUMPRODUCT((LEFT(A%d:A%d,4)="Week")*(F%d:F%d))`,
		dataStartRow, currentRow-1,
		dataStartRow, currentRow-1,
	)

	rows = append(rows, []interface{}{"Month Total", "", "", "", monthWorkFormula, monthLunchFormula})

	// Year accumulated: try to reference previous month
	prevMonth := t.AddDate(0, -1, 0)
	prevMonthTab := prevMonth.Format("2006-01")
	yearWorkFormula := fmt.Sprintf(
		`=IFERROR('%s'!E%d,0)+E%d`,
		prevMonthTab, currentRow+1, currentRow,
	)
	yearLunchFormula := fmt.Sprintf(
		`=IFERROR('%s'!F%d,0)+F%d`,
		prevMonthTab, currentRow+1, currentRow,
	)

	rows = append(rows, []interface{}{"Year Accumulated", "", "", "", yearWorkFormula, yearLunchFormula})

	// Write all rows
	rangeStr := fmt.Sprintf("'%s'!A1", yearMonth)
	vr := &sheets.ValueRange{
		Values: rows,
	}

	_, err = c.srv.Spreadsheets.Values.Update(c.spreadsheetID, rangeStr, vr).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("populate tab %s: %w", yearMonth, err)
	}

	return nil
}

// WriteEntry writes a single entry to the correct position in the month tab.
// It finds the first empty row for the entry's date and writes there.
func (c *SheetsClient) WriteEntry(entry *models.Entry) error {
	t, err := time.Parse("2006-01-02", entry.Date)
	if err != nil {
		return fmt.Errorf("parse date: %w", err)
	}

	yearMonth := t.Format("2006-01")

	if err := c.EnsureMonthTab(yearMonth); err != nil {
		return err
	}

	// Read all values to find where to insert
	rangeStr := fmt.Sprintf("'%s'!A:F", yearMonth)
	resp, err := c.srv.Spreadsheets.Values.Get(c.spreadsheetID, rangeStr).Do()
	if err != nil {
		return fmt.Errorf("read sheet: %w", err)
	}

	dateStr := entry.Date
	hours := float64(entry.DurationMinutes) / 60.0

	name := entry.DisplayName()

	// Find the row for this date that's empty (no assignment yet)
	// or find the first row after the last entry for this date
	insertRow := -1
	for i, row := range resp.Values {
		if len(row) > 0 {
			cellVal := fmt.Sprintf("%v", row[0])
			if cellVal == dateStr && (len(row) < 3 || fmt.Sprintf("%v", row[2]) == "") {
				insertRow = i + 1 // 1-indexed
				break
			}
		}
	}

	if insertRow == -1 {
		// No empty row for this date — need to insert a new row
		// Find the position after the last entry for this date, or after the date row itself
		for i, row := range resp.Values {
			if len(row) > 0 {
				cellVal := fmt.Sprintf("%v", row[0])
				if cellVal == dateStr {
					insertRow = i + 2 // after this date's row (1-indexed, +1 for next row)
					// Keep scanning to find last entry for this date
					for j := i + 1; j < len(resp.Values); j++ {
						if len(resp.Values[j]) > 0 {
							nextVal := fmt.Sprintf("%v", resp.Values[j][0])
							if nextVal == dateStr {
								insertRow = j + 2
							} else {
								break
							}
						} else {
							break
						}
					}
					break
				}
			}
		}
	}

	if insertRow == -1 {
		return fmt.Errorf("could not find row for date %s in tab %s", dateStr, yearMonth)
	}

	// Write the entry data
	writeRange := fmt.Sprintf("'%s'!A%d:F%d", yearMonth, insertRow, insertRow)
	dayName := t.Weekday().String()[:3]

	vr := &sheets.ValueRange{
		Values: [][]interface{}{
			{dateStr, dayName, name, entry.StartTime, entry.EndTime, hours},
		},
	}

	_, err = c.srv.Spreadsheets.Values.Update(c.spreadsheetID, writeRange, vr).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("write entry to sheet: %w", err)
	}

	return nil
}

// ClearEntriesForDateRange blanks out the data columns (Assignment, Start, End, Hours)
// for all rows whose date falls within [fromDate, toDate], preserving the sheet structure.
func (c *SheetsClient) ClearEntriesForDateRange(fromDate, toDate string) error {
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return fmt.Errorf("parse from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return fmt.Errorf("parse to date: %w", err)
	}

	// Collect all months in the range
	months := map[string]bool{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		months[d.Format("2006-01")] = true
	}

	for yearMonth := range months {
		if err := c.clearEntriesInTab(yearMonth, fromDate, toDate); err != nil {
			return err
		}
	}

	return nil
}

func (c *SheetsClient) clearEntriesInTab(yearMonth, fromDate, toDate string) error {
	rangeStr := fmt.Sprintf("'%s'!A:F", yearMonth)
	resp, err := c.srv.Spreadsheets.Values.Get(c.spreadsheetID, rangeStr).Do()
	if err != nil {
		return fmt.Errorf("read sheet %s: %w", yearMonth, err)
	}

	for i, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		cellVal := fmt.Sprintf("%v", row[0])
		// Check if this is a date row in the purge range
		_, parseErr := time.Parse("2006-01-02", cellVal)
		if parseErr != nil {
			continue
		}
		if cellVal < fromDate || cellVal > toDate {
			continue
		}
		// Only clear rows that have data in column C (assignment name)
		if len(row) < 3 || fmt.Sprintf("%v", row[2]) == "" {
			continue
		}

		// Blank out columns C-F (keep date and day name)
		rowNum := i + 1 // 1-indexed
		clearRange := fmt.Sprintf("'%s'!C%d:F%d", yearMonth, rowNum, rowNum)
		vr := &sheets.ValueRange{
			Values: [][]interface{}{{"", "", "", ""}},
		}
		_, err := c.srv.Spreadsheets.Values.Update(c.spreadsheetID, clearRange, vr).
			ValueInputOption("RAW").Do()
		if err != nil {
			return fmt.Errorf("clear row %d in %s: %w", rowNum, yearMonth, err)
		}
	}

	return nil
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

