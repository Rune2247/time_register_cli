package actions

import (
	"context"
	"fmt"
	"log"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/models"
)

// UpdateEntryAndResolveOverlaps updates an entry and adjusts overlapping entries on the same day.
func UpdateEntryAndResolveOverlaps(d *db.DB, entry models.Entry) error {
	// Calculate duration
	dur := 0
	if entry.StartTime != "" && entry.EndTime != "" {
		dur, _ = CalcDurationMinutes(entry.StartTime, entry.EndTime)
	}

	if err := d.UpdateEntry(entry.ID, entry.Name, entry.StartTime, entry.EndTime, dur); err != nil {
		return fmt.Errorf("update entry: %w", err)
	}

	// Resolve overlaps with other entries on the same day
	if err := resolveOverlaps(d, entry.Date, entry.ID); err != nil {
		return fmt.Errorf("resolve overlaps: %w", err)
	}

	// Reset sync flags so the day gets re-synced
	if err := d.ResetSyncFlagsForDate(entry.Date); err != nil {
		return fmt.Errorf("reset sync flags: %w", err)
	}

	TriggerSync(d)
	return nil
}

// DeleteEntryAndResync deletes an entry and resets sync flags for the day.
func DeleteEntryAndResync(d *db.DB, entry models.Entry) error {
	if err := d.DeleteEntry(entry.ID); err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}

	if err := d.ResetSyncFlagsForDate(entry.Date); err != nil {
		return fmt.Errorf("reset sync flags: %w", err)
	}

	fmt.Printf("Deleted entry: %s (%s-%s)\n", entry.Name, entry.StartTime, entry.EndTime)
	return nil
}

// ResyncDay deletes calendar events for a day and resets sync flags so everything gets re-posted.
func ResyncDay(d *db.DB, date string) error {
	ctx := context.Background()

	_, calID := GetGoogleConfig(d)
	if calID != "" {
		cal, err := googleapi.NewCalendarClient(ctx, calID)
		if err != nil {
			log.Printf("calendar client error: %v", err)
		} else {
			if err := cal.DeleteEventsForDate(date); err != nil {
				log.Printf("delete calendar events: %v", err)
			} else {
				fmt.Printf("Cleared calendar events for %s\n", date)
			}
		}
	}

	if err := d.ResetSyncFlagsForDate(date); err != nil {
		return fmt.Errorf("reset sync flags: %w", err)
	}

	fmt.Printf("Day %s marked for re-sync\n", date)
	TriggerSync(d)
	return nil
}

// resolveOverlaps adjusts entries that overlap with the edited entry.
func resolveOverlaps(d *db.DB, date string, editedID int64) error {
	entries, err := d.GetEntriesByDate(date)
	if err != nil {
		return err
	}

	var edited *models.Entry
	for i := range entries {
		if entries[i].ID == editedID {
			edited = &entries[i]
			break
		}
	}
	if edited == nil {
		return nil
	}

	for _, other := range entries {
		if other.ID == editedID {
			continue
		}
		if other.StartTime == "" || other.EndTime == "" {
			continue
		}
		if edited.StartTime == "" || edited.EndTime == "" {
			continue
		}

		// Check for overlap: other.Start < edited.End AND other.End > edited.Start
		if other.StartTime < edited.EndTime && other.EndTime > edited.StartTime {
			newStart := other.StartTime
			newEnd := other.EndTime

			// If edited completely covers other, delete other
			if edited.StartTime <= other.StartTime && edited.EndTime >= other.EndTime {
				if err := d.DeleteEntry(other.ID); err != nil {
					return fmt.Errorf("delete covered entry %d: %w", other.ID, err)
				}
				fmt.Printf("Removed overlapped entry: %s (%s-%s)\n", other.Name, other.StartTime, other.EndTime)
				continue
			}

			// If edited overlaps the start of other, push other's start forward
			if edited.EndTime > other.StartTime && edited.EndTime < other.EndTime {
				newStart = edited.EndTime
			}

			// If edited overlaps the end of other, pull other's end back
			if edited.StartTime > other.StartTime && edited.StartTime < other.EndTime {
				newEnd = edited.StartTime
			}

			if newStart != other.StartTime || newEnd != other.EndTime {
				dur, _ := CalcDurationMinutes(newStart, newEnd)
				if err := d.UpdateEntry(other.ID, other.Name, newStart, newEnd, dur); err != nil {
					return fmt.Errorf("adjust entry %d: %w", other.ID, err)
				}
				fmt.Printf("Adjusted %s: %s-%s → %s-%s\n", other.Name, other.StartTime, other.EndTime, newStart, newEnd)
			}
		}
	}

	return nil
}
