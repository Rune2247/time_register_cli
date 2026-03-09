package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rlf/time_register_cli/internal/actions"
	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/models"
)

type Worker struct {
	db       *db.DB
	interval time.Duration
	stop     chan struct{}
}

func NewWorker(d *db.DB, interval time.Duration) *Worker {
	return &Worker{
		db:       d,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins the background sync loop.
func (w *Worker) Start() {
	go w.run()
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	close(w.stop)
}

// SyncNow runs a single sync cycle. Can be called directly for immediate sync.
func (w *Worker) SyncNow() error {
	return w.syncOnce()
}

func (w *Worker) run() {
	// Run once immediately
	w.autoEndDay()
	if err := w.syncOnce(); err != nil {
		log.Printf("sync error: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.autoEndDay()
			if err := w.syncOnce(); err != nil {
				log.Printf("sync error: %v", err)
			}
		case <-w.stop:
			return
		}
	}
}

// autoEndDay checks if there are open entries from previous days (or today past
// the default end time) and automatically closes them.
func (w *Worker) autoEndDay() {
	defaultEnd, _ := w.db.GetConfig("default_end_time")
	if defaultEnd == "" {
		defaultEnd = "16:00"
	}

	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()

	// Check today: if past default end time and there's an open entry, close it
	if now > defaultEnd {
		open, err := w.db.GetLastOpenEntry(today)
		if err != nil {
			log.Printf("auto end-day check error: %v", err)
			return
		}
		if open != nil {
			log.Printf("Auto-ending day at default time %s", defaultEnd)
			if err := actions.EndDaySilent(w.db, today, defaultEnd); err != nil {
				log.Printf("auto end-day error: %v", err)
			}
		}
	}

	// Check previous days (up to 7 days back) for unclosed entries
	// Past days with no end-of-day get closed at 23:59 (end of that day)
	loc, _ := time.LoadLocation("Europe/Copenhagen")
	for i := 1; i <= 7; i++ {
		pastDate := time.Now().In(loc).AddDate(0, 0, -i).Format("2006-01-02")
		open, err := w.db.GetLastOpenEntry(pastDate)
		if err != nil {
			continue
		}
		if open != nil {
			log.Printf("Auto-ending unclosed day %s at 23:59", pastDate)
			if err := actions.EndDaySilent(w.db, pastDate, "23:59"); err != nil {
				log.Printf("auto end-day error for %s: %v", pastDate, err)
			}
		}
	}
}

func (w *Worker) syncOnce() error {
	entries, err := w.db.GetUnsyncedEntries()
	if err != nil {
		return fmt.Errorf("get unsynced: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	ctx := context.Background()

	// Get config
	spreadsheetID, _ := w.db.GetConfig("spreadsheet_id")
	calendarID, _ := w.db.GetConfig("calendar_id")

	var sheetsClient *googleapi.SheetsClient
	var calendarClient *googleapi.CalendarClient

	if spreadsheetID != "" {
		sheetsClient, err = googleapi.NewSheetsClient(ctx, spreadsheetID)
		if err != nil {
			log.Printf("sheets client error (will retry): %v", err)
		}
	}

	if calendarID != "" {
		calendarClient, err = googleapi.NewCalendarClient(ctx, calendarID)
		if err != nil {
			log.Printf("calendar client error (will retry): %v", err)
		}
	}

	for _, entry := range entries {
		w.syncEntry(entry, sheetsClient, calendarClient)
	}

	return nil
}

func (w *Worker) syncEntry(entry models.Entry, sheetsClient *googleapi.SheetsClient, calendarClient *googleapi.CalendarClient) {
	// Sync to Sheets
	if !entry.PostedToSheets && sheetsClient != nil {
		if err := sheetsClient.WriteEntry(&entry); err != nil {
			log.Printf("sheets sync failed for entry %d: %v", entry.ID, err)
		} else {
			if err := w.db.MarkPostedToSheets(entry.ID); err != nil {
				log.Printf("mark posted to sheets failed for entry %d: %v", entry.ID, err)
			}
		}
	}

	// Sync to Calendar
	if !entry.PostedToCalendar && calendarClient != nil {
		// Skip end_day entries — they're just markers
		if entry.EntryType == models.EntryEndDay {
			if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
				log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
			}
			return
		}

		if err := calendarClient.CreateEvent(&entry); err != nil {
			log.Printf("calendar sync failed for entry %d: %v", entry.ID, err)
		} else {
			if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
				log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
			}
		}
	}
}
