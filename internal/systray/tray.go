//go:build systray

package systray

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/rlf/time_register_cli/internal/actions"
	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
	syncpkg "github.com/rlf/time_register_cli/internal/sync"
)

type Tray struct {
	db     *db.DB
	worker *syncpkg.Worker
	stop   chan struct{}

	// Menu items
	mStatus    *systray.MenuItem
	mStart     *systray.MenuItem
	mLunch     *systray.MenuItem
	mEnd       *systray.MenuItem
	mSync      *systray.MenuItem
	mQuit      *systray.MenuItem
}

func Run(d *db.DB) {
	t := &Tray{
		db:   d,
		stop: make(chan struct{}),
	}
	systray.Run(t.onReady, t.onExit)
}

func (t *Tray) onReady() {
	systray.SetIcon(generateIcon())
	systray.SetTitle("TimeReg")
	systray.SetTooltip("TimeReg — Time Registration")

	t.mStatus = systray.AddMenuItem("Loading...", "Current status")
	t.mStatus.Disable()

	systray.AddSeparator()

	t.mStart = systray.AddMenuItem("Start Assignment", "Start a new assignment")
	t.mLunch = systray.AddMenuItem("Lunch", "Start lunch break")
	t.mEnd = systray.AddMenuItem("End Day", "End the work day")

	systray.AddSeparator()

	t.mSync = systray.AddMenuItem("Sync Now", "Sync entries to Google")
	t.mQuit = systray.AddMenuItem("Quit", "Quit TimeReg")

	// Start background sync worker (every 5 minutes)
	t.worker = syncpkg.NewWorker(t.db, 5*time.Minute)
	t.worker.Start()

	// Start status updater
	go t.updateStatusLoop()

	// Handle menu clicks
	go t.handleClicks()
}

func (t *Tray) onExit() {
	if t.worker != nil {
		t.worker.Stop()
	}
	close(t.stop)
}

func (t *Tray) handleClicks() {
	for {
		select {
		case <-t.mStart.ClickedCh:
			t.handleStart()
		case <-t.mLunch.ClickedCh:
			t.handleLunch()
		case <-t.mEnd.ClickedCh:
			t.handleEnd()
		case <-t.mSync.ClickedCh:
			t.handleSync()
		case <-t.mQuit.ClickedCh:
			systray.Quit()
			return
		case <-t.stop:
			return
		}
	}
}

func (t *Tray) handleStart() {
	// Use zenity for input dialog on Linux
	name, err := zenityInput("TimeReg", "Assignment name:")
	if err != nil || name == "" {
		return
	}

	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()
	if err := actions.StartAssignment(t.db, today, now, name); err != nil {
		zenityNotify("TimeReg Error", err.Error())
		return
	}

	zenityNotify("TimeReg", fmt.Sprintf("Started: %s at %s", name, now))
	t.updateStatus()
	go t.worker.SyncNow()
}

func (t *Tray) handleLunch() {
	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()
	if err := actions.StartLunch(t.db, today, now); err != nil {
		zenityNotify("TimeReg Error", err.Error())
		return
	}

	zenityNotify("TimeReg", fmt.Sprintf("Lunch started at %s", now))
	t.updateStatus()
	go t.worker.SyncNow()
}

func (t *Tray) handleEnd() {
	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()
	if err := actions.EndDay(t.db, today, now); err != nil {
		zenityNotify("TimeReg Error", err.Error())
		return
	}

	dayStatus, _ := actions.GetDayStatus(t.db, today)
	weekStatus, _ := actions.GetWeekStatus(t.db, today)

	msg := fmt.Sprintf("Day ended at %s\nToday: %.1fh worked\nThis week: %.1fh worked, %.1fh lunch",
		now,
		float64(dayStatus.WorkMinutes)/60.0,
		float64(weekStatus.WorkMinutes)/60.0,
		float64(weekStatus.LunchMinutes)/60.0,
	)
	zenityNotify("TimeReg", msg)
	t.updateStatus()
	go t.worker.SyncNow()
}

func (t *Tray) handleSync() {
	t.mSync.SetTitle("Syncing...")
	t.mSync.Disable()

	go func() {
		err := t.worker.SyncNow()
		if err != nil {
			zenityNotify("TimeReg Sync Error", err.Error())
		} else {
			zenityNotify("TimeReg", "Sync complete")
		}
		t.mSync.SetTitle("Sync Now")
		t.mSync.Enable()
	}()
}

func (t *Tray) updateStatusLoop() {
	t.updateStatus()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.updateStatus()
		case <-t.stop:
			return
		}
	}
}

func (t *Tray) updateStatus() {
	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()

	entries, err := t.db.GetEntriesByDate(today)
	if err != nil {
		t.mStatus.SetTitle("Error loading status")
		return
	}

	// Find the current open entry
	var current *models.Entry
	for i := range entries {
		if entries[i].EndTime == "" && entries[i].EntryType != models.EntryEndDay {
			current = &entries[i]
		}
	}

	if current == nil {
		// Check if day has ended
		for _, e := range entries {
			if e.EntryType == models.EntryEndDay {
				t.mStatus.SetTitle("Day ended")
				systray.SetTitle("TR")
				return
			}
		}
		t.mStatus.SetTitle("No active assignment")
		systray.SetTitle("TR")
		return
	}

	// Calculate elapsed time
	elapsed, _ := actions.CalcDurationMinutes(current.StartTime, now)
	hours := elapsed / 60
	mins := elapsed % 60

	label := current.Name
	if current.EntryType == models.EntryLunch {
		label = "Lunch"
	}

	statusText := fmt.Sprintf("%s (%dh%02dm)", label, hours, mins)
	t.mStatus.SetTitle(statusText)
	systray.SetTitle(fmt.Sprintf("TR %s", statusText))
}

// zenityInput shows an input dialog using zenity.
func zenityInput(title, prompt string) (string, error) {
	cmd := exec.Command("zenity", "--entry", "--title", title, "--text", prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim trailing newline
	result := string(out)
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// zenityNotify shows a notification using notify-send.
func zenityNotify(title, body string) {
	exec.Command("notify-send", title, body).Run()
}
