package actions

import (
	"context"
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
)

// TriggerSync pushes unsynced entries to Google Sheets immediately.
// Calendar sync is left to the daemon since it needs complete entries.
func TriggerSync(d *db.DB) {
	entries, err := d.GetUnsyncedForSheets()
	if err != nil {
		fmt.Printf("Sync: could not get unsynced entries: %v\n", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	spreadsheetID, _ := GetGoogleConfig(d)
	if spreadsheetID == "" {
		return
	}

	ctx := context.Background()
	sheetsClient, err := googleapi.NewSheetsClient(ctx, spreadsheetID)
	if err != nil {
		fmt.Printf("Sync: sheets connection failed: %v\n", err)
		return
	}

	synced := 0
	failed := 0
	for _, entry := range entries {
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

	if synced > 0 {
		fmt.Printf("Synced %d entries to sheets", synced)
		if failed > 0 {
			fmt.Printf(" (%d failed)", failed)
		}
		fmt.Println()
	} else if failed > 0 {
		fmt.Printf("Sync failed: %d entries\n", failed)
	}
}
