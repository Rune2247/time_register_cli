# TimeReg CLI

Time register CLI for Ubuntu — a Go CLI tool that tracks your daily work assignments and syncs to Google Sheets and Google Calendar.

## Features

- Track work assignments, breaks, and lunch with start/end times
- Add timestamped notes to entries to document what you worked on
- Manage notes across days — browse, navigate, and delete
- Automatic lunch split — inserting lunch auto-splits overlapping assignments
- End-of-day summary with daily and weekly work/lunch/break totals
- Backfill past entries and edit existing ones
- Holiday marking for non-working days
- Sync to Google Sheets with monthly tabs, daily/weekly summaries, and month totals
- Google Calendar events for each completed entry
- Background sync daemon with 1-hour interval
- Ubuntu systray widget showing current assignment and elapsed time
- Interactive TUI menu and direct CLI subcommands
- Purge entries from DB, Calendar, and Sheets

## Commands

| Command | Description |
|---------|-------------|
| `timereg` | Interactive TUI menu |
| `timereg start <name>` | Start assignment (closes any open entry) |
| `timereg lunch <from> <to>` | Register lunch, auto-splits overlapping assignments |
| `timereg break` | Start a break (closes open entry) |
| `timereg end` | End the day, close open entry, print summary |
| `timereg note <text>` | Add timestamped note to current open entry |
| `timereg status` | Today + current week summary |
| `timereg week` | Full week summary (Mon–Sun) |
| `timereg backfill` | Interactive TUI for adding past entries |
| `timereg backlog <d/m> <time> <name>` | Quick-add a past entry |
| `timereg edit` | Interactive editor for existing entries |
| `timereg holiday [d/m]` | Mark a day as holiday |
| `timereg sync` | Manually sync unposted entries |
| `timereg purge <from d/m> <to d/m>` | Delete entries from DB, Calendar, and Sheets |
| `timereg config` | Show/set configuration |
| `timereg auth` | Force re-authentication with Google |
| `timereg guide` | Google Cloud setup instructions |
| `timereg daemon` | Run systray widget + background sync |

## Daily Workflow

```bash
timereg start "Project Alpha"    # Start working
timereg lunch 12:00 12:30        # Splits assignment around lunch
timereg start "Code Review"      # Switch task
timereg note "Reviewed PR #42"   # Document what you did
timereg end                      # End day, prints summary
```

## TUI Menu

The interactive menu (`timereg` with no args) shows:

- Current assignment and elapsed time
- Today's worked hours
- Google sync status (with auth warning if token expired)
- Pending sync count and unclosed days

Menu options include Start, Lunch, Break, Add Note, Manage Notes, End Day, Backfill, Edit, Holiday, Status, and an Options submenu with Sync, Rebuild Calendar/Sheets, Purge, and more.

**Manage Notes** lets you browse notes across days with left/right arrow keys and delete individual notes with confirmation.

## Google Sheets Layout

Each month gets its own tab (e.g., `2026-03`):

- Header row: Date, Day, Type, Name, Start, End, Hours, Notes
- Each day has a date row, entry rows, and a daily summary row (work/lunch/break totals)
- Weekly summary after each Sunday with SUMPRODUCT formulas
- Month total at the bottom summing all weeks

## Tech Stack

- **Go** — primary language
- **SQLite** (via `modernc.org/sqlite`, pure Go) — local data store at `~/.config/timereg/`
- **Cobra** — CLI framework
- **Bubbletea / Bubbles** — interactive terminal UI
- **Google Sheets API v4** — monthly spreadsheet with formulas
- **Google Calendar API v3** — calendar events
- **golang.org/x/oauth2** — Google OAuth2
- **getlantern/systray** — Ubuntu systray/AppIndicator (optional, build tag: `systray`)

## Project Structure

```
cmd/timereg/main.go              # Entry point
internal/
  actions/                       # Business logic
    assignment.go                # Start assignment
    break.go                     # Start break
    lunch.go                     # Insert lunch with auto-split
    endday.go                    # End day + summary
    note.go                      # Add notes
    edit.go                      # Edit entries + resolve overlaps
    holiday.go                   # Mark holidays
    status.go                    # Day/week status
    purge.go                     # Purge date ranges
    rebuild.go                   # Rebuild Calendar/Sheets from DB
    sync.go                      # Trigger sync
    config.go                    # Config helpers
    setup.go                     # Interactive setup wizard
    helpers.go                   # Time/date utilities
  cli/
    tui.go                       # Interactive TUI menu (bubbletea)
    commands.go                  # Cobra subcommand handlers
    daemon.go                    # Daemon + autostart management
    guide.go                     # Setup guide + status check
  db/
    sqlite.go                    # SQLite connection + migrations
    entries.go                   # Entry CRUD
    config.go                    # Config key/value CRUD
  google/
    auth.go                      # OAuth2 flow + token management
    sheets.go                    # Sheets API (month tabs, formulas)
    calendar.go                  # Calendar API (events)
    create.go                    # Create new Sheets/Calendar resources
  sync/
    worker.go                    # Background sync goroutine
  systray/
    tray.go                      # Systray widget (build tag: systray)
    icon.go                      # Generated tray icon
    stub.go                      # Fallback when systray deps missing
  models/
    entry.go                     # Data structures + timezone
```

## Install

```bash
./install.sh                     # Builds + installs to ~/.local/bin
```

Or manually:

```bash
go build -o timereg ./cmd/timereg/              # CLI only
go build -tags systray -o timereg ./cmd/timereg/ # With systray
```

## Prerequisites

- **Go 1.21+** — [Install Go](https://go.dev/doc/install)
- **Linux** — tested on Ubuntu 24.04+
- **Google Cloud project** with Sheets API and Calendar API enabled (run `timereg guide`)

### Optional (for systray)

```bash
sudo apt install libayatana-appindicator3-dev libgtk-3-dev
```

## Setup

```bash
timereg guide          # Step-by-step Google Cloud setup
timereg auth           # Authenticate with Google
timereg config setup   # Interactive configuration wizard
```

## Configuration

Stored in SQLite at `~/.config/timereg/timereg.db`:

| Key | Description | Default |
|-----|-------------|---------|
| `spreadsheet_id` | Google Sheets ID (accepts full URL) | — |
| `calendar_id` | Google Calendar ID | — |
| `timezone` | IANA timezone | Europe/Copenhagen |

## Time Format

- Times: `830` or `08:30` (both become `08:30`)
- Dates: `d/m` format (e.g., `3/4` = April 3rd)
- All times in Europe/Copenhagen timezone
