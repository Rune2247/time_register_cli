package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

const defaultLunchMinutes = 30

func StartLunch(d *db.DB, date, triggerTime string) error {
	entries, err := d.GetEntriesByDate(date)
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}

	// Find the entry that covers the lunch trigger time
	var covering *models.Entry
	for i := range entries {
		e := &entries[i]
		if e.EntryType == models.EntryHoliday {
			continue
		}
		if e.StartTime <= triggerTime && (e.EndTime == "" || e.EndTime > triggerTime) {
			covering = e
		}
	}

	if covering != nil && covering.EndTime != "" {
		// Closed entry spans lunch time — split it
		return splitForLunch(d, date, covering, triggerTime)
	}

	// Open entry or no covering entry — use original behavior
	if err := closeOpenEntry(d, date, triggerTime); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryLunch,
		Name:      "Lunch",
		StartTime: triggerTime,
		EndTime:   "",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert lunch: %w", err)
	}

	fmt.Printf("Lunch started at %s (id: %d)\n", triggerTime, id)
	return nil
}

// splitForLunch splits a closed entry around a 30-minute lunch break.
// e.g. Work 8:00-16:00 + lunch at 12:00 → Work 8:00-12:00, Lunch 12:00-12:30, Work 12:30-16:00
func splitForLunch(d *db.DB, date string, covering *models.Entry, lunchStart string) error {
	lunchEnd, err := AddMinutes(lunchStart, defaultLunchMinutes)
	if err != nil {
		return err
	}

	originalEnd := covering.EndTime
	originalName := covering.Name
	originalType := covering.EntryType

	// 1. Shrink covering entry to end at lunch start
	dur, _ := CalcDurationMinutes(covering.StartTime, lunchStart)
	if err := d.UpdateEntryEndTime(covering.ID, lunchStart, dur); err != nil {
		return fmt.Errorf("shrink entry: %w", err)
	}
	fmt.Printf("%s %s-%s\n", originalName, covering.StartTime, lunchStart)

	// 2. Insert lunch entry (closed, 30 min)
	lunchDur, _ := CalcDurationMinutes(lunchStart, lunchEnd)
	lunchEntry := &models.Entry{
		Date:            date,
		EntryType:       models.EntryLunch,
		Name:            "Lunch",
		StartTime:       lunchStart,
		EndTime:         lunchEnd,
		DurationMinutes: lunchDur,
	}
	if _, err := d.InsertEntry(lunchEntry); err != nil {
		return fmt.Errorf("insert lunch: %w", err)
	}
	fmt.Printf("Lunch %s-%s\n", lunchStart, lunchEnd)

	// 3. Insert continuation entry (same name/type, from lunch end to original end)
	if lunchEnd < originalEnd {
		contDur, _ := CalcDurationMinutes(lunchEnd, originalEnd)
		contEntry := &models.Entry{
			Date:            date,
			EntryType:       originalType,
			Name:            originalName,
			StartTime:       lunchEnd,
			EndTime:         originalEnd,
			DurationMinutes: contDur,
		}
		if _, err := d.InsertEntry(contEntry); err != nil {
			return fmt.Errorf("insert continuation: %w", err)
		}
		fmt.Printf("%s %s-%s\n", originalName, lunchEnd, originalEnd)
	}

	return nil
}
