package actions

import (
	"context"
	"fmt"
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
			fmt.Printf("Warning: calendar client error: %v\n", err)
		} else {
			for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
				dateStr := day.Format("2006-01-02")
				if err := cal.DeleteEventsForDate(dateStr); err != nil {
					fmt.Printf("Warning: could not delete calendar events for %s: %v\n", dateStr, err)
				}
			}
			fmt.Println("Cleared calendar events")
		}
	}

	// Delete from SQLite
	deleted, err := d.DeleteEntriesByDateRange(fromDate, toDate)
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}

	// Rebuild affected month tabs with remaining entries
	if sheetID != "" {
		sc, err := googleapi.NewSheetsClient(ctx, sheetID)
		if err != nil {
			fmt.Printf("Warning: sheets client error: %v\n", err)
		} else {
			done := map[string]bool{}
			for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
				ym := day.Format("2006-01")
				if done[ym] {
					continue
				}
				done[ym] = true

				mt, _ := time.Parse("2006-01", ym)
				firstDay := mt.Format("2006-01-02")
				lastDay := time.Date(mt.Year(), mt.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

				remaining, err := d.GetEntriesByDateRange(firstDay, lastDay)
				if err != nil {
					fmt.Printf("Warning: could not get entries for %s: %v\n", ym, err)
					continue
				}

				if err := sc.WriteMonthTab(ym, remaining); err != nil {
					fmt.Printf("Warning: could not rebuild sheet tab %s: %v\n", ym, err)
				} else {
					for _, e := range remaining {
						d.MarkPostedToSheets(e.ID)
					}
					fmt.Printf("Rebuilt sheet tab %s\n", ym)
				}
			}
		}
	}

	fmt.Printf("Purged %s to %s\n", fromDate, toDate)
	if deleted > 0 {
		fmt.Printf("Removed: %d entries (%d assignments, %d lunch breaks", deleted, workCount, lunchCount)
		if otherCount > 0 {
			fmt.Printf(", %d other", otherCount)
		}
		fmt.Println(")")
	} else {
		fmt.Println("No local entries found (cleaned Google services only)")
	}

	return nil
}
