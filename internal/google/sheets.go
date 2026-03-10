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
	rangeStr := fmt.Sprintf("'%s'!A:G", name)
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

	// Group entries by date
	entryMap := map[string][]models.Entry{}
	for _, e := range entries {
		entryMap[e.Date] = append(entryMap[e.Date], e)
	}

	var rows [][]interface{}

	// Header
	rows = append(rows, []interface{}{"Date", "Day", "Type", "Name", "Start", "End", "Hours"})

	currentRow := 2 // 1-indexed, after header
	weekStartRow := currentRow
	var weekFormulaRows []int

	daysInMonth := daysIn(t.Year(), t.Month())

	for day := 1; day <= daysInMonth; day++ {
		d := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := d.Weekday().String()[:3]
		dateStr := d.Format("2006-01-02")

		dayEntries := entryMap[dateStr]

		if len(dayEntries) == 0 {
			// Empty day — one blank row
			rows = append(rows, []interface{}{dateStr, dayName, "", "", "", "", ""})
			currentRow++
		} else {
			// One row per entry
			for i, e := range dayEntries {
				date := dateStr
				dn := dayName
				if i > 0 {
					// Only show date/day on first row
					date = ""
					dn = ""
				}

				entryType := capitalizeType(e.EntryType)
				hours := float64(e.DurationMinutes) / 60.0

				rows = append(rows, []interface{}{
					date, dn, entryType, e.Name, e.StartTime, e.EndTime, hours,
				})
				currentRow++
			}
		}

		// Week summary after Sunday or last day of month
		if d.Weekday() == time.Sunday || day == daysInMonth {
			_, week := d.ISOWeek()

			workFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Assignment")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)
			lunchFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Lunch")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)
			breakFormula := fmt.Sprintf(
				`=SUMPRODUCT((C%d:C%d="Break")*(G%d:G%d))`,
				weekStartRow, currentRow-1,
				weekStartRow, currentRow-1,
			)

			// Label row
			rows = append(rows, []interface{}{
				fmt.Sprintf("Week %d", week), "", "", "", "Work", "Lunch", "Break",
			})
			currentRow++

			// Formula row
			rows = append(rows, []interface{}{
				"", "", "", "", workFormula, lunchFormula, breakFormula,
			})
			weekFormulaRows = append(weekFormulaRows, currentRow)
			currentRow++

			// Blank separator
			rows = append(rows, []interface{}{"", "", "", "", "", "", ""})
			currentRow++

			weekStartRow = currentRow
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

	monthTotalRow := currentRow
	rows = append(rows, []interface{}{"Month Total", "", "", "", monthWorkFormula, monthLunchFormula, monthBreakFormula})
	currentRow++

	// Year accumulated — INDEX/MATCH to find previous month's year total
	prevMonth := t.AddDate(0, -1, 0)
	prevTab := prevMonth.Format("2006-01")
	yearWorkFormula := fmt.Sprintf(
		`=IFERROR(INDEX('%s'!E:E,MATCH("Year Accumulated",'%s'!A:A,0)),0)+E%d`,
		prevTab, prevTab, monthTotalRow,
	)
	yearLunchFormula := fmt.Sprintf(
		`=IFERROR(INDEX('%s'!F:F,MATCH("Year Accumulated",'%s'!A:A,0)),0)+F%d`,
		prevTab, prevTab, monthTotalRow,
	)
	yearBreakFormula := fmt.Sprintf(
		`=IFERROR(INDEX('%s'!G:G,MATCH("Year Accumulated",'%s'!A:A,0)),0)+G%d`,
		prevTab, prevTab, monthTotalRow,
	)

	rows = append(rows, []interface{}{"Year Accumulated", "", "", "", yearWorkFormula, yearLunchFormula, yearBreakFormula})

	// Write everything
	rangeStr := fmt.Sprintf("'%s'!A1", yearMonth)
	_, err = c.srv.Spreadsheets.Values.Update(c.spreadsheetID, rangeStr, &sheets.ValueRange{
		Values: rows,
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("write tab %s: %w", yearMonth, err)
	}

	return nil
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
