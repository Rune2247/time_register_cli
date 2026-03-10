package actions

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/models"
)

// RebuildCalendar deletes all calendar events and re-syncs from the database.
func RebuildCalendar(d *db.DB) error {
	_, calID := GetGoogleConfig(d)
	if calID == "" {
		return fmt.Errorf("no calendar configured")
	}

	ctx := context.Background()
	cal, err := googleapi.NewCalendarClient(ctx, calID)
	if err != nil {
		return fmt.Errorf("calendar client: %w", err)
	}

	fmt.Println("Deleting all calendar events...")
	deleted, err := cal.DeleteAllEvents()
	if err != nil {
		return fmt.Errorf("delete events: %w", err)
	}
	fmt.Printf("Deleted %d events\n", deleted)

	if err := d.ResetAllCalendarFlags(); err != nil {
		return fmt.Errorf("reset calendar flags: %w", err)
	}

	entries, err := d.GetUnsyncedForCalendar()
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}

	fmt.Printf("Re-syncing %d entries to calendar...\n", len(entries))
	failed := 0
	for _, entry := range entries {
		if err := cal.CreateEvent(&entry); err != nil {
			log.Printf("calendar sync failed for entry %d: %v", entry.ID, err)
			failed++
			continue
		}
		if err := d.MarkPostedToCalendar(entry.ID); err != nil {
			log.Printf("mark posted failed for entry %d: %v", entry.ID, err)
		}
	}

	if failed > 0 {
		fmt.Printf("Calendar rebuild complete (%d failed)\n", failed)
	} else {
		fmt.Printf("Calendar rebuild complete (%d entries synced)\n", len(entries))
	}
	return nil
}

// RebuildSheets resets all sheet tabs and re-syncs from the database.
func RebuildSheets(d *db.DB) error {
	sheetID, _ := GetGoogleConfig(d)
	if sheetID == "" {
		return fmt.Errorf("no spreadsheet configured")
	}

	ctx := context.Background()
	sc, err := googleapi.NewSheetsClient(ctx, sheetID)
	if err != nil {
		return fmt.Errorf("sheets client: %w", err)
	}

	entries, err := d.GetAllEntries()
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No entries to sync")
		return nil
	}

	// Group entries by month
	months := map[string][]int{} // yearMonth -> indices into entries
	for i, e := range entries {
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		ym := t.Format("2006-01")
		months[ym] = append(months[ym], i)
	}

	// Rebuild each month tab
	failed := 0
	total := 0
	for ym, indices := range months {
		monthEntries := make([]models.Entry, len(indices))
		for j, idx := range indices {
			monthEntries[j] = entries[idx]
		}

		if err := sc.WriteMonthTab(ym, monthEntries); err != nil {
			fmt.Printf("Failed to rebuild %s: %v\n", ym, err)
			failed++
			continue
		}
		total += len(monthEntries)
		fmt.Printf("Rebuilt %s (%d entries)\n", ym, len(monthEntries))
	}

	// Mark all as synced to sheets
	for _, e := range entries {
		d.MarkPostedToSheets(e.ID)
	}

	if failed > 0 {
		fmt.Printf("Sheets rebuild complete (%d months failed)\n", failed)
	} else {
		fmt.Printf("Sheets rebuild complete (%d entries across %d months)\n", total, len(months))
	}
	return nil
}
