package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
)

// TriggerSync rebuilds sheet tabs that have unsynced entries.
// Calendar sync is left to the daemon since it needs complete entries.
func TriggerSync(d *db.DB) {
	d.SyncMutex.Lock()
	defer d.SyncMutex.Unlock()

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

	// Collect months that need rebuilding
	months := map[string]bool{}
	for _, e := range entries {
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		months[t.Format("2006-01")] = true
	}

	synced := 0
	failed := 0
	for ym := range months {
		t, _ := time.Parse("2006-01", ym)
		firstDay := t.Format("2006-01-02")
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

		allEntries, err := d.GetEntriesByDateRange(firstDay, lastDay)
		if err != nil {
			fmt.Printf("Sync: could not get entries for %s: %v\n", ym, err)
			failed++
			continue
		}

		if err := sheetsClient.WriteMonthTab(ym, allEntries); err != nil {
			fmt.Printf("Sync: sheets failed for %s: %v\n", ym, err)
			failed++
			continue
		}

		for _, e := range allEntries {
			d.MarkPostedToSheets(e.ID)
		}
		synced += len(allEntries)
	}

	if synced > 0 {
		fmt.Printf("Synced %d entries to sheets", synced)
		if failed > 0 {
			fmt.Printf(" (%d months failed)", failed)
		}
		fmt.Println()
	} else if failed > 0 {
		fmt.Printf("Sync failed: %d months\n", failed)
	}
}
