# TimeReg CLI

Time register CLI for Ubuntu -- a Go CLI tool that tracks your daily work assignments and syncs to Google Sheets and Google Calendar.

## Features

- Track work assignments with start and end times throughout the day
- Automatic lunch break handling with configurable default time and smart adjustment logic
- End-of-day summary with daily and weekly work/lunch hour totals
- Backfill support for missed entries (past dates and times)
- Holiday marking for non-working days
- Sync entries to a Google Sheets spreadsheet with monthly tabs, weekly summaries, and year-accumulated totals
- Create Google Calendar events for each completed assignment and lunch block
- Background sync worker that retries failed API calls with backoff
- Ubuntu systray widget showing current assignment and elapsed time
- Interactive TUI menu and direct CLI subcommands

## How It Works

TimeReg tracks your workday as a sequence of time blocks stored in a local SQLite database.

1. **Start an assignment** -- Run `timereg start "Assignment Name"` or select it from the interactive menu. This begins a new work block and closes any previous one.
2. **Take lunch** -- Run `timereg lunch`. The current assignment ends and a lunch block begins. If triggered after the configured lunch time (default 12:00), the boundary is adjusted back to that time. Lunch ends when the next assignment starts.
3. **End the day** -- Run `timereg end`. The last assignment is closed and a daily/weekly summary is printed. If not triggered manually, the day defaults to ending at the configured time (default 16:00).
4. **Backfill** -- Forgot to log something? `timereg backfill` walks you through adding a past entry with date, time, and assignment name.
5. **Mark a holiday** -- `timereg holiday` marks a full day as non-working (0 hours in the spreadsheet).

A background sync worker (running in the systray daemon) picks up new entries and pushes them to Google Sheets and Google Calendar. An entry is only marked as synced after receiving a successful API response.

## Tech Stack

- **Go** -- primary language
- **SQLite** (via `modernc.org/sqlite`, pure Go) -- local data store
- **Cobra** -- CLI framework and subcommand routing
- **Bubbletea / Bubbles** -- interactive terminal UI
- **Google Sheets API v4** -- monthly spreadsheet with formulas and summaries
- **Google Calendar API v3** -- calendar events per assignment
- **golang.org/x/oauth2** -- Google OAuth2 authentication
- **getlantern/systray** -- Ubuntu systray/AppIndicator integration

## Project Structure

```
time_register_cli/
├── cmd/
│   └── timereg/
│       └── main.go           # Entry point
├── internal/
│   ├── actions/              # Modular business logic (all features)
│   │   ├── assignment.go     # StartAssignment
│   │   ├── lunch.go          # StartLunch
│   │   ├── endday.go         # EndDay + status output
│   │   ├── holiday.go        # MarkHoliday
│   │   ├── status.go         # GetDayStatus, GetWeekStatus
│   │   ├── config.go         # GetConfig, SetConfig
│   │   ├── setup.go          # Interactive setup wizard
│   │   └── helpers.go        # Time/date utilities
│   ├── cli/
│   │   ├── tui.go            # Interactive TUI menu (bubbletea)
│   │   ├── commands.go       # Cobra subcommand handlers
│   │   ├── daemon.go         # Daemon + autostart management
│   │   └── guide.go          # Setup guide + status check
│   ├── db/
│   │   ├── sqlite.go         # SQLite connection + migrations
│   │   ├── entries.go        # Entry CRUD operations
│   │   └── config.go         # Config key/value CRUD
│   ├── google/
│   │   ├── auth.go           # OAuth2 flow + token management
│   │   ├── sheets.go         # Sheets API (month tabs, formulas)
│   │   └── calendar.go       # Calendar API (events)
│   ├── sync/
│   │   └── worker.go         # Background sync goroutine
│   ├── systray/
│   │   ├── tray.go           # Systray widget (build tag: systray)
│   │   ├── icon.go           # Generated tray icon
│   │   └── stub.go           # Fallback when systray deps missing
│   └── models/
│       └── entry.go          # Data structures
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Prerequisites

- **Go 1.21+** -- [Install Go](https://go.dev/doc/install)
- **Ubuntu / Linux** -- tested on Ubuntu 24.04+
- **Google Cloud project** with Sheets API and Calendar API enabled (see `timereg guide`)

### Optional (for systray support)

```bash
sudo apt install libayatana-appindicator3-dev libgtk-3-dev
```

Also used by the systray (typically pre-installed on Ubuntu):
- `zenity` -- input dialogs
- `notify-send` -- desktop notifications

## Getting Started

### 1. Build

```bash
# Basic build (CLI only)
go build -o timereg ./cmd/timereg/

# Full build with systray support
go build -tags systray -o timereg ./cmd/timereg/
```

### 2. Install (optional)

Move the binary somewhere on your PATH:

```bash
sudo mv timereg /usr/local/bin/
```

### 3. Set up Google integration

Run the built-in guide for step-by-step instructions:

```bash
timereg guide
```

Or check what's already configured:

```bash
timereg guide status
```

### 4. Quick start

```bash
timereg                        # Interactive TUI menu
timereg start "Task name"      # Start an assignment
timereg lunch                  # Take lunch
timereg start "Another task"   # Switch to next assignment
timereg end                    # End the day
timereg status                 # View today + week summary
```

### 5. Systray daemon (optional)

```bash
timereg daemon                 # Run systray + background sync
timereg autostart enable       # Start on login
```

## Configuration

Run `timereg config` to open an interactive configuration menu. The following settings are available:

- **Google Spreadsheet ID** -- the target spreadsheet for time entries
- **Google Calendar ID** -- the calendar where work events are created
- **Default lunch time** -- when lunch is assumed to start (default: 12:00)
- **Default end-of-day time** -- when the workday ends if not closed manually (default: 16:00)
- **Timezone** -- used for all time calculations (default: Europe/Copenhagen)
- **Google OAuth login** -- authenticate with your Google account

All configuration is stored in the local SQLite database at `~/.config/timereg/config.db`.
