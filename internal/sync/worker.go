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
	if err := w.syncOnce(); err != nil {
		log.Printf("sync error: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.syncOnce(); err != nil {
				log.Printf("sync error: %v", err)
			}
		case <-w.stop:
			return
		}
	}
}

func (w *Worker) syncOnce() error {
	ctx := context.Background()
	spreadsheetID, calendarID := actions.GetGoogleConfig(w.db)

	var err error

	// Sync sheets — rebuild month tabs that have unsynced entries
	if spreadsheetID != "" {
		sheetsClient, clientErr := googleapi.NewSheetsClient(ctx, spreadsheetID)
		if clientErr != nil {
			log.Printf("sheets client error (will retry): %v", clientErr)
		} else {
			sheetEntries, err := w.db.GetUnsyncedForSheets()
			if err != nil {
				return fmt.Errorf("get unsynced for sheets: %w", err)
			}
			if len(sheetEntries) > 0 {
				w.syncSheetsByMonth(sheetEntries, sheetsClient)
			}
		}
	}

	// Sync calendar — individual events
	if calendarID != "" {
		calendarClient, clientErr := googleapi.NewCalendarClient(ctx, calendarID)
		if clientErr != nil {
			log.Printf("calendar client error (will retry): %v", clientErr)
		} else {
			calEntries, err := w.db.GetUnsyncedForCalendar()
			if err != nil {
				return fmt.Errorf("get unsynced for calendar: %w", err)
			}
			for _, entry := range calEntries {
				w.syncToCalendar(entry, calendarClient)
			}
		}
	}

	_ = err
	return nil
}

// syncSheetsByMonth collects months that need syncing and rebuilds each tab.
func (w *Worker) syncSheetsByMonth(unsyncedEntries []models.Entry, client *googleapi.SheetsClient) {
	// Collect unique months that need syncing
	months := map[string]bool{}
	for _, e := range unsyncedEntries {
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		months[t.Format("2006-01")] = true
	}

	// Rebuild each month tab with ALL entries for that month
	for ym := range months {
		t, _ := time.Parse("2006-01", ym)
		firstDay := t.Format("2006-01-02")
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

		allEntries, err := w.db.GetEntriesByDateRange(firstDay, lastDay)
		if err != nil {
			log.Printf("get entries for %s: %v", ym, err)
			continue
		}

		if err := client.WriteMonthTab(ym, allEntries); err != nil {
			log.Printf("sheets sync failed for %s: %v", ym, err)
			continue
		}

		// Mark all entries in this month as synced
		for _, e := range allEntries {
			if err := w.db.MarkPostedToSheets(e.ID); err != nil {
				log.Printf("mark posted to sheets failed for entry %d: %v", e.ID, err)
			}
		}
		log.Printf("rebuilt sheet tab %s (%d entries)", ym, len(allEntries))
	}
}

func (w *Worker) syncToCalendar(entry models.Entry, client *googleapi.CalendarClient) {
	if err := client.CreateEvent(&entry); err != nil {
		log.Printf("calendar sync failed for entry %d: %v", entry.ID, err)
	} else {
		if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
			log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
		}
	}
}
