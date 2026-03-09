package models

type EntryType string

const (
	EntryAssignment EntryType = "assignment"
	EntryLunch      EntryType = "lunch"
	EntryEndDay     EntryType = "end_day"
	EntryHoliday    EntryType = "holiday"
)

type Entry struct {
	ID               int64     `json:"id"`
	Date             string    `json:"date"`               // YYYY-MM-DD
	EntryType        EntryType `json:"entry_type"`          // assignment, lunch, end_day, holiday
	Name             string    `json:"name"`                // assignment name (empty for lunch/end_day)
	StartTime        string    `json:"start_time"`          // HH:MM 24h
	EndTime          string    `json:"end_time"`            // HH:MM 24h, empty until closed
	DurationMinutes  int       `json:"duration_minutes"`    // calculated when end_time is set
	PostedToSheets   bool      `json:"posted_to_sheets"`
	PostedToCalendar bool      `json:"posted_to_calendar"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

type DayStatus struct {
	Date          string
	Entries       []Entry
	WorkMinutes   int
	LunchMinutes  int
}

type WeekStatus struct {
	StartDate    string // Monday
	EndDate      string // Sunday
	Days         []DayStatus
	WorkMinutes  int
	LunchMinutes int
}
