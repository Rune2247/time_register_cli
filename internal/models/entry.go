package models

import (
	"fmt"
	"time"
)

// CopenhagenTZ is the shared timezone used for all time calculations.
var CopenhagenTZ *time.Location

func init() {
	var err error
	CopenhagenTZ, err = time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		CopenhagenTZ = time.UTC
	}
}

type EntryType string

const (
	EntryAssignment EntryType = "assignment"
	EntryLunch      EntryType = "lunch"
	EntryBreak      EntryType = "break"
	EntryHoliday    EntryType = "holiday"
)

type Entry struct {
	ID               int64     `json:"id"`
	Date             string    `json:"date"`
	EntryType        EntryType `json:"entry_type"`
	Name             string    `json:"name"`
	StartTime        string    `json:"start_time"`
	EndTime          string    `json:"end_time"`
	DurationMinutes  int       `json:"duration_minutes"`
	Notes            string    `json:"notes"`
	PostedToSheets   bool      `json:"posted_to_sheets"`
	PostedToCalendar bool      `json:"posted_to_calendar"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// DisplayName returns the display label for an entry based on its type.
func (e *Entry) DisplayName() string {
	switch e.EntryType {
	case EntryLunch:
		return "Lunch"
	case EntryBreak:
		return "Break"
	case EntryHoliday:
		return "Holiday"
	default:
		return e.Name
	}
}

// FormatLine returns a single-line display for an entry (e.g. "08:00-12:00  Work").
func (e *Entry) FormatLine() string {
	name := e.DisplayName()
	switch e.EntryType {
	case EntryHoliday:
		return "           Holiday"
	default:
		end := e.EndTime
		if end == "" {
			end = "..."
		}
		return fmt.Sprintf("%s-%s  %s", e.StartTime, end, name)
	}
}

// AccumulateMinutes sums work, lunch, and break minutes from a slice of entries.
func AccumulateMinutes(entries []Entry) (workMinutes, lunchMinutes, breakMinutes int) {
	for _, e := range entries {
		if e.DurationMinutes <= 0 {
			continue
		}
		switch e.EntryType {
		case EntryAssignment:
			workMinutes += e.DurationMinutes
		case EntryLunch:
			lunchMinutes += e.DurationMinutes
		case EntryBreak:
			breakMinutes += e.DurationMinutes
		}
	}
	return
}

// FormatStatusSummary returns a formatted string with day and week hours.
func FormatStatusSummary(dayStatus *DayStatus, weekStatus *WeekStatus) string {
	s := fmt.Sprintf("Today: %.1fh worked\nThis week: %.1fh worked, %.1fh lunch",
		float64(dayStatus.WorkMinutes)/60.0,
		float64(weekStatus.WorkMinutes)/60.0,
		float64(weekStatus.LunchMinutes)/60.0,
	)
	if weekStatus.BreakMinutes > 0 {
		s += fmt.Sprintf(", %.1fh break", float64(weekStatus.BreakMinutes)/60.0)
	}
	return s
}

type DayStatus struct {
	Date         string
	Entries      []Entry
	WorkMinutes  int
	LunchMinutes int
	BreakMinutes int
}

type WeekStatus struct {
	StartDate    string
	EndDate      string
	Days         []DayStatus
	WorkMinutes  int
	LunchMinutes int
	BreakMinutes int
}
