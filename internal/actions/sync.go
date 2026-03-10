package actions

import (
	"context"
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
)

// TriggerSync syncs all unsynced entries to Google Sheets and Calendar.
// Prints status so the user knows what happened.
func TriggerSync(d *db.DB) {
	entries, err := d.GetUnsyncedEntries()
	if err != nil {
		fmt.Printf("Sync: could not get unsynced entries: %v\n", err)
		return
	}
	if len(entries) == 0 {
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
			fmt.Printf("Sync: sheets connection failed: %v\n", err)
		}
	}

	if calendarID != "" {
		calendarClient, err = googleapi.NewCalendarClient(ctx, calendarID)
		if err != nil {
			fmt.Printf("Sync: calendar connection failed: %v\n", err)
		}
	}

	synced := 0
	failed := 0
	for _, entry := range entries {
		if !entry.PostedToSheets && sheetsClient != nil {
			if err := sheetsClient.WriteEntry(&entry); err != nil {
				fmt.Printf("Sync: sheets failed for %s %s: %v\n", entry.Date, entry.DisplayName(), err)
				failed++
			} else {
				if err := d.MarkPostedToSheets(entry.ID); err != nil {
					fmt.Printf("Sync: could not mark entry %d as posted to sheets: %v\n", entry.ID, err)
				}
				synced++
			}
		}
		if !entry.PostedToCalendar && calendarClient != nil {
			if err := calendarClient.CreateEvent(&entry); err != nil {
				fmt.Printf("Sync: calendar failed for %s %s: %v\n", entry.Date, entry.DisplayName(), err)
				failed++
			} else {
				if err := d.MarkPostedToCalendar(entry.ID); err != nil {
					fmt.Printf("Sync: could not mark entry %d as posted to calendar: %v\n", entry.ID, err)
				}
				synced++
			}
		}
	}

	if synced > 0 {
		fmt.Printf("Synced %d items", synced)
		if failed > 0 {
			fmt.Printf(" (%d failed)", failed)
		}
		fmt.Println()
	} else if failed > 0 {
		fmt.Printf("Sync failed: %d items\n", failed)
	}
}
