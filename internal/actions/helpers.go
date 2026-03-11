package actions

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

// closeOpenEntry closes the last open entry on the given date by setting its
// end_time and calculating duration. Called before starting a new entry.
func closeOpenEntry(d *db.DB, date, endTime string) error {
	open, err := d.GetLastOpenEntry(date)
	if err != nil {
		return fmt.Errorf("get open entry: %w", err)
	}
	if open == nil {
		return nil
	}

	duration, err := CalcDurationMinutes(open.StartTime, endTime)
	if err != nil {
		return fmt.Errorf("calc duration: %w", err)
	}

	return d.UpdateEntryEndTime(open.ID, endTime, duration)
}

// CalcDurationMinutes returns the difference in minutes between two HH:MM times.
// Handles midnight crossing (e.g. 23:00 to 01:00 = 120 minutes).
func CalcDurationMinutes(startTime, endTime string) (int, error) {
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return 0, fmt.Errorf("parse start time %q: %w", startTime, err)
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return 0, fmt.Errorf("parse end time %q: %w", endTime, err)
	}
	diff := int(end.Sub(start).Minutes())
	if diff < 0 {
		diff += 24 * 60
	}
	return diff, nil
}

// AddMinutes adds minutes to a HH:MM time string and returns the result as HH:MM.
func AddMinutes(timeStr string, minutes int) (string, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return "", fmt.Errorf("parse time %q: %w", timeStr, err)
	}
	t = t.Add(time.Duration(minutes) * time.Minute)
	return t.Format("15:04"), nil
}

// NowInCopenhagen returns the current time in Europe/Copenhagen as HH:MM.
func NowInCopenhagen() string {
	return time.Now().In(models.CopenhagenTZ).Format("15:04")
}

// TodayInCopenhagen returns today's date in Europe/Copenhagen as YYYY-MM-DD.
func TodayInCopenhagen() string {
	return time.Now().In(models.CopenhagenTZ).Format("2006-01-02")
}

// DateInCurrentYear converts day + month to YYYY-MM-DD using the current year in Copenhagen.
func DateInCurrentYear(day, month int) string {
	year := time.Now().In(models.CopenhagenTZ).Year()
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// ParseSlashDate converts "d/m" string to YYYY-MM-DD using current year.
func ParseSlashDate(s string) (string, error) {
	var day, month int
	_, err := fmt.Sscanf(s, "%d/%d", &day, &month)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: expected d/m format", s)
	}
	return DateInCurrentYear(day, month), nil
}

// ParseBacklogDate converts separate day and month strings to YYYY-MM-DD.
func ParseBacklogDate(dayStr, monthStr string) (string, error) {
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return "", fmt.Errorf("invalid day %q: %w", dayStr, err)
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return "", fmt.Errorf("invalid month %q: %w", monthStr, err)
	}
	return DateInCurrentYear(day, month), nil
}

// ParseTimeInput converts HHMM (e.g. "830", "1600") to HH:MM.
// If the input already contains ":", it is returned as-is.
func ParseTimeInput(s string) (string, error) {
	for i := range s {
		if s[i] == ':' {
			// Normalize to HH:MM (pad single-digit hour)
			parts := s[:i]
			rest := s[i+1:]
			for len(parts) < 2 {
				parts = "0" + parts
			}
			return parts + ":" + rest, nil
		}
	}
	for len(s) < 4 {
		s = "0" + s
	}
	if len(s) != 4 {
		return "", fmt.Errorf("invalid time %q: expected HHMM format", s)
	}
	return s[:2] + ":" + s[2:], nil
}

// GetGoogleConfig returns the spreadsheet and calendar IDs from config.
func GetGoogleConfig(d *db.DB) (spreadsheetID, calendarID string) {
	spreadsheetID, _ = d.GetConfig("spreadsheet_id")
	calendarID, _ = d.GetConfig("calendar_id")
	return
}
