package google

import (
	"context"
	"fmt"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/sheets/v4"
)

// CreateSpreadsheet creates a new Google Spreadsheet with the given name
// and returns the spreadsheet ID.
func CreateSpreadsheet(ctx context.Context, name string) (string, error) {
	opt, err := Authenticate(ctx)
	if err != nil {
		return "", err
	}

	srv, err := sheets.NewService(ctx, opt)
	if err != nil {
		return "", fmt.Errorf("create sheets service: %w", err)
	}

	ss := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: name,
		},
	}

	created, err := srv.Spreadsheets.Create(ss).Do()
	if err != nil {
		return "", fmt.Errorf("create spreadsheet: %w", err)
	}

	return created.SpreadsheetId, nil
}

// CreateCalendar creates a new Google Calendar with the given name
// and returns the calendar ID.
func CreateCalendar(ctx context.Context, name string) (string, error) {
	opt, err := Authenticate(ctx)
	if err != nil {
		return "", err
	}

	srv, err := calendar.NewService(ctx, opt)
	if err != nil {
		return "", fmt.Errorf("create calendar service: %w", err)
	}

	cal := &calendar.Calendar{
		Summary:  name,
		TimeZone: "Europe/Copenhagen",
	}

	created, err := srv.Calendars.Insert(cal).Do()
	if err != nil {
		return "", fmt.Errorf("create calendar: %w", err)
	}

	return created.Id, nil
}
