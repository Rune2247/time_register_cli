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

// Sheet columns:
// A=Date, B=Day, C=Type, D=Name, E=Start, F=End, G=Hours

// EnsureMonthTab creates a tab for the given month if it doesn't exist,
// and populates it with headers, day rows, week summary rows, and formulas.
func (c *SheetsClient) EnsureMonthTab(yearMonth string) error {
	ss, err := c.srv.Spreadsheets.Get(c.spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("get spreadsheet: %w", err)
	}

	for _, sheet := range ss.Sheets {
		if sheet.Properties.Title == yearMonth {
			return nil
		}
	}

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

	return c.populateMonthTab(yearMonth)
}

func (c *SheetsClient) populateMonthTab(yearMonth string) error {
	t, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return fmt.Errorf("parse month: %w", err)
	}

	var rows [][]interface{}

	// Header row: A=Date, B=Day, C=Type, D=Name, E=Start, F=End, G=Hours
	rows = append(rows, []interface{}{"Date", "Day", "Type", "Name", "Start", "End", "Hours"})

	dataStartRow := 2 // 1-indexed, row after header
	currentRow := dataStartRow
	weekStartRow := currentRow

	daysInMonth := daysIn(t.Year(), t.Month())

	for day := 1; day <= daysInMonth; day++ {
		d := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := d.Weekday().String()[:3]
		dateStr := d.Format("2006-01-02")

		rows = append(rows, []interface{}{dateStr, dayName, "", "", "", "", ""})
		currentRow++

		if d.Weekday() == time.Sunday || day == daysInMonth {
			_, week := d.ISOWeek()

			// Work: sum hours where type is not Lunch/Break/Holiday and type is not empty
			workFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d<>"Lunch")*(C%d:C%d<>"Break")*(C%d:C%d<>"Holiday")*(C%d:C%d<>"")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)
			// Lunch: sum hours where type is Lunch
			lunchFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Lunch")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)
			// Break: sum hours where type is Break
			breakFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Break")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)

			rows = append(rows, []interface{}{
				fmt.Sprintf("Week %d", week), "", "", "", "Work", "Lunch", "Break",
			})
			rows = append(rows, []interface{}{
				"", "", "", "", workFormula, lunchFormula, breakFormula,
			})

			rows = append(rows, []interface{}{"", "", "", "", "", "", ""}) // blank row
			currentRow += 3
			weekStartRow = currentRow
		}
	}

	// Month total — sum the formula rows (which are 1 row below each "Week N" label)
	monthWorkFormula := fmt.Sprintf(
		`=SUMPRODUCT((LEFT(A%d:A%d,4)="Week")*(E%d:E%d))`,
		dataStartRow, currentRow-1,
		dataStartRow+1, currentRow,
	)
	monthLunchFormula := fmt.Sprintf(
		`=SUMPRODUCT((LEFT(A%d:A%d,4)="Week")*(F%d:F%d))`,
		dataStartRow, currentRow-1,
		dataStartRow+1, currentRow,
	)
	monthBreakFormula := fmt.Sprintf(
		`=SUMPRODUCT((LEFT(A%d:A%d,4)="Week")*(G%d:G%d))`,
		dataStartRow, currentRow-1,
		dataStartRow+1, currentRow,
	)

	rows = append(rows, []interface{}{"Month Total", "", "", "", monthWorkFormula, monthLunchFormula, monthBreakFormula})

	// Year accumulated
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
	yearBreakFormula := fmt.Sprintf(
		`=IFERROR('%s'!G%d,0)+G%d`,
		prevMonthTab, currentRow+1, currentRow,
	)

	rows = append(rows, []interface{}{"Year Accumulated", "", "", "", yearWorkFormula, yearLunchFormula, yearBreakFormula})

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
func (c *SheetsClient) WriteEntry(entry *models.Entry) error {
	t, err := time.Parse("2006-01-02", entry.Date)
	if err != nil {
		return fmt.Errorf("parse date: %w", err)
	}

	yearMonth := t.Format("2006-01")

	if err := c.EnsureMonthTab(yearMonth); err != nil {
		return err
	}

	rangeStr := fmt.Sprintf("'%s'!A:G", yearMonth)
	resp, err := c.srv.Spreadsheets.Values.Get(c.spreadsheetID, rangeStr).Do()
	if err != nil {
		return fmt.Errorf("read sheet: %w", err)
	}

	dateStr := entry.Date
	hours := float64(entry.DurationMinutes) / 60.0

	entryType := string(entry.EntryType)
	// Capitalize first letter for display
	if len(entryType) > 0 {
		entryType = string(entryType[0]-32) + entryType[1:]
	}

	name := entry.Name

	// Find empty row for this date or position after last entry for this date
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
		for i, row := range resp.Values {
			if len(row) > 0 {
				cellVal := fmt.Sprintf("%v", row[0])
				if cellVal == dateStr {
					insertRow = i + 2
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

	writeRange := fmt.Sprintf("'%s'!A%d:G%d", yearMonth, insertRow, insertRow)
	dayName := t.Weekday().String()[:3]

	vr := &sheets.ValueRange{
		Values: [][]interface{}{
			{dateStr, dayName, entryType, name, entry.StartTime, entry.EndTime, hours},
		},
	}

	_, err = c.srv.Spreadsheets.Values.Update(c.spreadsheetID, writeRange, vr).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("write entry to sheet: %w", err)
	}

	return nil
}

// ResetMonthTabs re-populates the month tabs for all months in the given date range,
// clearing all entry data and restoring the blank template with formulas.
func (c *SheetsClient) ResetMonthTabs(fromDate, toDate string) error {
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return fmt.Errorf("parse from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return fmt.Errorf("parse to date: %w", err)
	}

	done := map[string]bool{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		ym := d.Format("2006-01")
		if done[ym] {
			continue
		}
		done[ym] = true
		if err := c.populateMonthTab(ym); err != nil {
			return fmt.Errorf("reset tab %s: %w", ym, err)
		}
	}

	return nil
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
