package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func InsertLunch(d *db.DB, date, startTime, endTime string) error {
	dur, err := CalcDurationMinutes(startTime, endTime)
	if err != nil {
		return fmt.Errorf("calc lunch duration: %w", err)
	}

	entries, err := d.GetEntriesByDate(date)
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}

	// Find entries that the lunch overlaps with and split them
	for i := range entries {
		e := &entries[i]
		if e.EntryType == models.EntryHoliday || e.EntryType == models.EntryLunch || e.EntryType == models.EntryBreak {
			continue
		}
		eEnd := e.EndTime
		if eEnd == "" {
			// Open entry — close it at lunch start
			if e.StartTime < startTime {
				closeDur, err := CalcDurationMinutes(e.StartTime, startTime)
				if err != nil {
					return fmt.Errorf("calc duration for %s-%s: %w", e.StartTime, startTime, err)
				}
				if err := d.UpdateEntryEndTime(e.ID, startTime, closeDur); err != nil {
					return fmt.Errorf("close open entry: %w", err)
				}
				fmt.Printf("%s %s-%s\n", e.Name, e.StartTime, startTime)
			}
			continue
		}

		// Check overlap: entry.Start < lunchEnd AND entry.End > lunchStart
		if e.StartTime < endTime && eEnd > startTime {
			if err := splitEntry(d, date, e, startTime, endTime); err != nil {
				return err
			}
		}
	}

	// Insert the lunch entry
	lunchEntry := &models.Entry{
		Date:            date,
		EntryType:       models.EntryLunch,
		Name:            "Lunch",
		StartTime:       startTime,
		EndTime:         endTime,
		DurationMinutes: dur,
	}
	id, err := d.InsertEntry(lunchEntry)
	if err != nil {
		return fmt.Errorf("insert lunch: %w", err)
	}

	fmt.Printf("Lunch %s-%s (id: %d)\n", startTime, endTime, id)
	TriggerSync(d)
	return nil
}

// splitEntry splits a closed entry around a time window (e.g. lunch or break).
// e.g. Work 8:00-16:00 + split 12:00-12:30 → Work 8:00-12:00, Work 12:30-16:00
func splitEntry(d *db.DB, date string, e *models.Entry, splitStart, splitEnd string) error {
	originalEnd := e.EndTime
	originalName := e.Name
	originalType := e.EntryType

	// Shrink entry to end at split start
	if e.StartTime < splitStart {
		dur, err := CalcDurationMinutes(e.StartTime, splitStart)
		if err != nil {
			return fmt.Errorf("calc duration for %s-%s: %w", e.StartTime, splitStart, err)
		}
		if err := d.UpdateEntryEndTime(e.ID, splitStart, dur); err != nil {
			return fmt.Errorf("shrink entry: %w", err)
		}
		fmt.Printf("%s %s-%s\n", originalName, e.StartTime, splitStart)
	} else {
		// Entry starts within or at split start — delete it
		if err := d.DeleteEntry(e.ID); err != nil {
			return fmt.Errorf("delete covered entry: %w", err)
		}
	}

	// Insert continuation entry after the split window
	if splitEnd < originalEnd {
		contDur, err := CalcDurationMinutes(splitEnd, originalEnd)
		if err != nil {
			return fmt.Errorf("calc continuation duration for %s-%s: %w", splitEnd, originalEnd, err)
		}
		contEntry := &models.Entry{
			Date:            date,
			EntryType:       originalType,
			Name:            originalName,
			StartTime:       splitEnd,
			EndTime:         originalEnd,
			DurationMinutes: contDur,
		}
		if _, err := d.InsertEntry(contEntry); err != nil {
			return fmt.Errorf("insert continuation: %w", err)
		}
		fmt.Printf("%s %s-%s\n", originalName, splitEnd, originalEnd)
	}

	return nil
}
