package actions

import (
	"context"
	"log"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
)

// TriggerSync syncs all unsynced entries to Google Sheets and Calendar.
// Failures are logged but not returned — this is a best-effort fire-and-forget.
func TriggerSync(d *db.DB) {
	entries, err := d.GetUnsyncedEntries()
	if err != nil || len(entries) == 0 {
		return
	}

	spreadsheetID, calendarID := GetGoogleConfig(d)
	if spreadsheetID == "" && calendarID == "" {
		return
	}

	ctx := context.Background()

	var sheetsClient *googleapi.SheetsClient
	var calendarClient *googleapi.CalendarClient

	if spreadsheetID != "" {
		sheetsClient, err = googleapi.NewSheetsClient(ctx, spreadsheetID)
		if err != nil {
			log.Printf("sync: sheets client error: %v", err)
		}
	}

	if calendarID != "" {
		calendarClient, err = googleapi.NewCalendarClient(ctx, calendarID)
		if err != nil {
			log.Printf("sync: calendar client error: %v", err)
		}
	}

	for _, entry := range entries {
		if !entry.PostedToSheets && sheetsClient != nil {
			if err := sheetsClient.WriteEntry(&entry); err != nil {
				log.Printf("sync: sheets failed for entry %d: %v", entry.ID, err)
			} else {
				_ = d.MarkPostedToSheets(entry.ID)
			}
		}
		if !entry.PostedToCalendar && calendarClient != nil {
			if err := calendarClient.CreateEvent(&entry); err != nil {
				log.Printf("sync: calendar failed for entry %d: %v", entry.ID, err)
			} else {
				_ = d.MarkPostedToCalendar(entry.ID)
			}
		}
	}
}
