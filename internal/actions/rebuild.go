package actions

import (
	"context"
	"fmt"
	"log"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
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

	fromDate := entries[0].Date
	toDate := entries[len(entries)-1].Date

	fmt.Printf("Resetting sheet tabs for %s to %s...\n", fromDate, toDate)
	if err := sc.ResetMonthTabs(fromDate, toDate); err != nil {
		return fmt.Errorf("reset tabs: %w", err)
	}

	if err := d.ResetAllSheetsFlags(); err != nil {
		return fmt.Errorf("reset sheets flags: %w", err)
	}

	sheetsEntries, err := d.GetUnsyncedForSheets()
	if err != nil {
		return fmt.Errorf("get unsynced: %w", err)
	}

	fmt.Printf("Re-syncing %d entries to sheets...\n", len(sheetsEntries))
	failed := 0
	for _, entry := range sheetsEntries {
		if err := sc.WriteEntry(&entry); err != nil {
			log.Printf("sheets sync failed for entry %d: %v", entry.ID, err)
			failed++
			continue
		}
		if err := d.MarkPostedToSheets(entry.ID); err != nil {
			log.Printf("mark posted failed for entry %d: %v", entry.ID, err)
		}
	}

	if failed > 0 {
		fmt.Printf("Sheets rebuild complete (%d failed)\n", failed)
	} else {
		fmt.Printf("Sheets rebuild complete (%d entries synced)\n", len(sheetsEntries))
	}
	return nil
}
