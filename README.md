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
│       └── main.go         # Entry point
├── internal/
│   ├── cli/
│   │   ├── tui.go          # Interactive TUI menu
│   │   ├── commands.go     # Direct subcommand handlers
│   │   └── backfill.go     # Backfill flow
│   ├── db/
│   │   ├── sqlite.go       # SQLite connection + migrations
│   │   ├── entries.go      # Entry CRUD operations
│   │   └── config.go       # Config CRUD operations
│   ├── google/
│   │   ├── auth.go         # OAuth2 flow
│   │   ├── sheets.go       # Sheets API integration
│   │   └── calendar.go     # Calendar API integration
│   ├── sync/
│   │   └── worker.go       # Background sync goroutine
│   ├── systray/
│   │   └── tray.go         # Ubuntu systray/AppIndicator
│   └── models/
│       └── entry.go        # Data structures
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Getting Started

_To be filled in._

## Configuration

Run `timereg config` to open an interactive configuration menu. The following settings are available:

- **Google Spreadsheet ID** -- the target spreadsheet for time entries
- **Google Calendar ID** -- the calendar where work events are created
- **Default lunch time** -- when lunch is assumed to start (default: 12:00)
- **Default end-of-day time** -- when the workday ends if not closed manually (default: 16:00)
- **Timezone** -- used for all time calculations (default: Europe/Copenhagen)
- **Google OAuth login** -- authenticate with your Google account

All configuration is stored in the local SQLite database at `~/.config/timereg/config.db`.
