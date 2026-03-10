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

// PurgeTimeRange deletes all entries in the given date range from SQLite,
// Google Calendar, and Google Sheets. It prints a summary of what was removed.
func PurgeTimeRange(d *db.DB, fromDate, toDate string) error {
	// Validate from < to
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return fmt.Errorf("parse from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return fmt.Errorf("parse to date: %w", err)
	}
	if from.After(to) {
		return fmt.Errorf("from date (%s) must be before to date (%s)", fromDate, toDate)
	}

	// Get entries before deleting so we can report counts
	entries, err := d.GetEntriesByDateRange(fromDate, toDate)
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Printf("No entries found between %s and %s\n", fromDate, toDate)
		return nil
	}

	// Count by type
	var workCount, lunchCount, otherCount int
	for _, e := range entries {
		switch e.EntryType {
		case models.EntryAssignment:
			workCount++
		case models.EntryLunch:
			lunchCount++
		default:
			otherCount++
		}
	}

	ctx := context.Background()
	sheetID, calID := GetGoogleConfig(d)

	// Delete calendar events for each day in range
	if calID != "" {
		cal, err := googleapi.NewCalendarClient(ctx, calID)
		if err != nil {
			log.Printf("calendar client error: %v", err)
		} else {
			for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
				dateStr := day.Format("2006-01-02")
				if err := cal.DeleteEventsForDate(dateStr); err != nil {
					log.Printf("delete calendar events for %s: %v", dateStr, err)
				}
			}
			fmt.Println("Cleared calendar events")
		}
	}

	// Clear sheet entries for the date range
	if sheetID != "" {
		sc, err := googleapi.NewSheetsClient(ctx, sheetID)
		if err != nil {
			log.Printf("sheets client error: %v", err)
		} else {
			if err := sc.ClearEntriesForDateRange(fromDate, toDate); err != nil {
				log.Printf("clear sheet entries: %v", err)
			} else {
				fmt.Println("Cleared spreadsheet entries")
			}
		}
	}

	// Delete from SQLite
	deleted, err := d.DeleteEntriesByDateRange(fromDate, toDate)
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}

	fmt.Printf("Purged %s to %s\n", fromDate, toDate)
	fmt.Printf("Removed: %d entries (%d assignments, %d lunch breaks", deleted, workCount, lunchCount)
	if otherCount > 0 {
		fmt.Printf(", %d other", otherCount)
	}
	fmt.Println(")")

	return nil
}
