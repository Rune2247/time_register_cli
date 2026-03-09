package google

import (
	"context"
	"fmt"
	"time"

	"github.com/rlf/time_register_cli/internal/models"
	"google.golang.org/api/calendar/v3"
)

type CalendarClient struct {
	srv        *calendar.Service
	calendarID string
}

func NewCalendarClient(ctx context.Context, calendarID string) (*CalendarClient, error) {
	opt, err := Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := calendar.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar service: %w", err)
	}

	return &CalendarClient{srv: srv, calendarID: calendarID}, nil
}

// CreateEvent creates a calendar event for a completed entry.
func (c *CalendarClient) CreateEvent(entry *models.Entry) error {
	if entry.EndTime == "" {
		return fmt.Errorf("entry has no end time")
	}

	title := entry.Name
	if entry.EntryType == models.EntryLunch {
		title = "Lunch"
	} else if entry.EntryType == models.EntryEndDay {
		return nil // don't create events for end-day markers
	} else if entry.EntryType == models.EntryHoliday {
		title = "Holiday"
	}

	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	startDT, err := time.ParseInLocation("2006-01-02 15:04", entry.Date+" "+entry.StartTime, loc)
	if err != nil {
		return fmt.Errorf("parse start: %w", err)
	}

	endDT, err := time.ParseInLocation("2006-01-02 15:04", entry.Date+" "+entry.EndTime, loc)
	if err != nil {
		return fmt.Errorf("parse end: %w", err)
	}

	event := &calendar.Event{
		Summary: title,
		Start: &calendar.EventDateTime{
			DateTime: startDT.Format(time.RFC3339),
			TimeZone: "Europe/Copenhagen",
		},
		End: &calendar.EventDateTime{
			DateTime: endDT.Format(time.RFC3339),
			TimeZone: "Europe/Copenhagen",
		},
	}

	_, err = c.srv.Events.Insert(c.calendarID, event).Do()
	if err != nil {
		return fmt.Errorf("create calendar event: %w", err)
	}

	return nil
}
