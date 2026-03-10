package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rlf/time_register_cli/internal/actions"
	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/models"
	syncpkg "github.com/rlf/time_register_cli/internal/sync"
)

const guideQuickText = `Setup Guide
==========

1. Set up Google API (run in terminal):
   timereg guide           Full step-by-step instructions
   timereg guide status    Check what's configured

2. Authenticate:
   timereg auth            Google OAuth login

3. Connect services (pick one):
   timereg config create-spreadsheet "TimeReg 2026"
   timereg config create-calendar "Work Log"
   -- or --
   timereg config set spreadsheet_id "URL or ID"
   timereg config set calendar_id "calendar@group.calendar.google.com"

4. Configure defaults:
   timereg config setup    Interactive wizard

5. Start the systray daemon (optional):
   timereg daemon          Run top bar widget + background sync
   timereg autostart enable  Start on login`

// Styles
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	resultStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

type phase int

const (
	phaseMenu phase = iota
	phaseInput
	phaseBackfillType
	phaseBackfillDate
	phaseBackfillTime
	phaseBackfillName
	phaseHolidayDate
	phaseEditDayList
	phaseEditEntryList
	phaseEditFieldSelect
	phaseEditValue
	phaseOptionsMenu
	phasePurgeFrom
	phasePurgeTo
	phaseResult
)

type menuItem struct {
	label string
	value string
}

var mainMenu = []menuItem{
	{label: "Start Assignment", value: "start"},
	{label: "Lunch", value: "lunch"},
	{label: "End Day", value: "end"},
	{label: "Backfill", value: "backfill"},
	{label: "Edit Entries", value: "edit"},
	{label: "Holiday", value: "holiday"},
	{label: "Status", value: "status"},
	{label: "Options", value: "options"},
}

var optionsMenu = []menuItem{
	{label: "Config", value: "config"},
	{label: "Sync Now", value: "sync"},
	{label: "Restart Daemon", value: "daemon"},
	{label: "Purge Time Range", value: "purge"},
	{label: "Setup Guide", value: "guide"},
}

var backfillTypes = []menuItem{
	{label: "Start Assignment", value: "assignment"},
	{label: "Lunch", value: "lunch"},
	{label: "End Day", value: "end"},
}

var editFieldOptions = []menuItem{
	{label: "Name", value: "name"},
	{label: "Start Time", value: "start"},
	{label: "End Time", value: "end"},
	{label: "Delete", value: "delete"},
}

type model struct {
	db            *db.DB
	phase         phase
	cursor        int
	menuItems     []menuItem
	textInput     textinput.Model
	inputPrompt   string
	result        string
	err           error
	quitting      bool
	statusHeader  string

	// backfill state
	backfillType string
	backfillDate string
	backfillTime string

	// edit state
	editDates     []string
	editEntries   []models.Entry
	editEntry     models.Entry
	editField     string // "name", "start", "end"

	// purge state
	purgeFrom string
}

func newModel(d *db.DB) model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	m := model{
		db:        d,
		phase:     phaseMenu,
		menuItems: mainMenu,
		textInput: ti,
	}
	m.statusHeader = m.buildStatusHeader()
	return m
}

func (m model) buildStatusHeader() string {
	var b strings.Builder

	today := actions.TodayInCopenhagen()
	now := actions.NowInCopenhagen()

	entries, err := m.db.GetEntriesByDate(today)
	if err == nil {
		var current *models.Entry
		for i := range entries {
			e := &entries[i]
			if e.EndTime == "" {
				current = e
			}
		}
		if current != nil {
			elapsed, _ := actions.CalcDurationMinutes(current.StartTime, now)
			h := elapsed / 60
			mins := elapsed % 60
			b.WriteString(dimStyle.Render(fmt.Sprintf("  Current: %s (%dh%02dm)", current.DisplayName(), h, mins)))
		} else {
			b.WriteString(dimStyle.Render("  Current: No active assignment"))
		}
		b.WriteString("\n")

		dayStatus, err := actions.GetDayStatus(m.db, today)
		if err == nil {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  Today:   %.1fh worked", float64(dayStatus.WorkMinutes)/60.0)))
			b.WriteString("\n")
		}
	}

	sheetID, calID := actions.GetGoogleConfig(m.db)
	googleStatus := "not configured"
	if sheetID != "" && calID != "" {
		if googleapi.IsAuthenticated() {
			googleStatus = "connected"
		} else {
			googleStatus = "not authenticated"
		}
	} else if sheetID != "" || calID != "" {
		googleStatus = "partially configured"
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Google:  %s", googleStatus)))
	b.WriteString("\n")

	unsynced, err := m.db.GetUnsyncedEntries()
	if err == nil && len(unsynced) > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Pending: %d entries to sync", len(unsynced))))
		b.WriteString("\n")
	}

	return b.String()
}

func RunTUI(d *db.DB) error {
	p := tea.NewProgram(newModel(d))
	_, err := p.Run()
	return err
}

func RunEditTUI(d *db.DB) error {
	m := newModel(d)
	dates, err := m.db.GetDistinctDates(30)
	if err != nil {
		return err
	}
	if len(dates) == 0 {
		fmt.Println("No entries to edit.")
		return nil
	}
	m.editDates = dates
	m.phase = phaseEditDayList
	m.menuItems = m.datesToMenuItems()
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.phase == phaseMenu || m.phase == phaseResult {
				m.quitting = true
				return m, tea.Quit
			}
			// Go back to menu from any input phase
			m.phase = phaseMenu
			m.cursor = 0
			m.menuItems = mainMenu
			m.err = nil
			return m, nil
		case "esc":
			return m.goBack()

		}
	}

	switch m.phase {
	case phaseMenu, phaseBackfillType, phaseEditDayList, phaseEditEntryList, phaseEditFieldSelect, phaseOptionsMenu:
		return m.updateMenu(msg)
	case phaseInput, phaseBackfillDate, phaseBackfillTime, phaseBackfillName, phaseHolidayDate, phaseEditValue, phasePurgeFrom, phasePurgeTo:
		return m.updateInput(msg)
	case phaseResult:
		return m.updateResult(msg)
	}

	return m, nil
}

func (m model) goBack() (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseOptionsMenu:
		m.phase = phaseMenu
		m.cursor = 0
		m.menuItems = mainMenu
		m.err = nil
		return m, nil
	case phaseEditEntryList:
		m.phase = phaseEditDayList
		m.cursor = 0
		m.menuItems = m.datesToMenuItems()
		m.err = nil
		return m, nil
	case phaseEditFieldSelect:
		// Back to entry list
		m.phase = phaseEditEntryList
		m.cursor = 0
		m.menuItems = m.entriesToMenuItems()
		m.err = nil
		return m, nil
	case phasePurgeFrom:
		m.phase = phaseOptionsMenu
		m.cursor = 0
		m.menuItems = optionsMenu
		m.err = nil
		return m, nil
	case phasePurgeTo:
		m.phase = phasePurgeFrom
		m.inputPrompt = "From date (d/m, e.g. 3/4 for April 3rd)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "3/4"
		m.err = nil
		return m, nil
	case phaseEditValue:
		// Back to field select
		m.phase = phaseEditFieldSelect
		m.cursor = 0
		m.menuItems = editFieldOptions
		m.err = nil
		return m, nil
	default:
		m.phase = phaseMenu
		m.cursor = 0
		m.menuItems = mainMenu
		m.err = nil
		return m, nil
	}
}

func (m model) datesToMenuItems() []menuItem {
	items := make([]menuItem, len(m.editDates))
	for i, d := range m.editDates {
		items[i] = menuItem{label: d, value: d}
	}
	return items
}

func (m model) entriesToMenuItems() []menuItem {
	items := make([]menuItem, len(m.editEntries))
	for i := range m.editEntries {
		items[i] = menuItem{
			label: m.editEntries[i].FormatLine(),
			value: fmt.Sprintf("%d", m.editEntries[i].ID),
		}
	}
	return items
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case "enter":
			return m.handleMenuSelect()
		}
	}
	return m, nil
}

func (m model) handleMenuSelect() (tea.Model, tea.Cmd) {
	selected := m.menuItems[m.cursor].value

	if m.phase == phaseEditDayList {
		return m.handleEditDaySelect(selected)
	}
	if m.phase == phaseEditEntryList {
		return m.handleEditEntrySelect(selected)
	}
	if m.phase == phaseEditFieldSelect {
		return m.handleEditFieldSelect(selected)
	}

	if m.phase == phaseOptionsMenu {
		return m.handleOptionsSelect(selected)
	}

	if m.phase == phaseBackfillType {
		m.backfillType = selected
		m.phase = phaseBackfillDate
		now := time.Now().In(models.CopenhagenTZ)
		todaySlash := fmt.Sprintf("%d/%d", now.Day(), int(now.Month()))
		m.inputPrompt = "Date (d/m, e.g. 3/4 for April 3rd)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = todaySlash
		return m, nil
	}

	// Main menu
	switch selected {
	case "start":
		m.phase = phaseInput
		m.inputPrompt = "Assignment name"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "e.g. Client meeting"
		return m, nil

	case "lunch":
		return m.showResultWithCapture(func() error {
			return actions.StartLunch(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		})

	case "end":
		return m.showResultWithCapture(func() error {
			return actions.EndDay(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		})

	case "edit":
		dates, err := m.db.GetDistinctDates(30)
		if err != nil {
			m.phase = phaseResult
			m.err = err
			return m, nil
		}
		if len(dates) == 0 {
			m.phase = phaseResult
			m.result = "No entries to edit."
			return m, nil
		}
		m.editDates = dates
		m.phase = phaseEditDayList
		m.cursor = 0
		m.menuItems = m.datesToMenuItems()
		return m, nil

	case "backfill":
		m.phase = phaseBackfillType
		m.cursor = 0
		m.menuItems = backfillTypes
		return m, nil

	case "holiday":
		m.phase = phaseHolidayDate
		m.inputPrompt = "Date (d/m, e.g. 24/12 for Dec 24th, or Enter for today)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "24/12"
		return m, nil

	case "status":
		return m.showResultWithCapture(func() error {
			return actions.PrintStatus(m.db, actions.TodayInCopenhagen())
		})

	case "options":
		m.phase = phaseOptionsMenu
		m.cursor = 0
		m.menuItems = optionsMenu
		return m, nil
	}

	return m, nil
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m.handleInputSubmit()
		case "tab":
			if m.textInput.Value() == "" && m.textInput.Placeholder != "" {
				m.textInput.SetValue(m.textInput.Placeholder)
				m.textInput.CursorEnd()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) handleInputSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.textInput.Value())

	switch m.phase {
	case phaseInput:
		// Start assignment with entered name
		if value == "" {
			m.err = fmt.Errorf("assignment name cannot be empty")
			return m, nil
		}
		return m.showResultWithCapture(func() error {
			return actions.StartAssignment(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen(), value)
		})

	case phaseHolidayDate:
		date := actions.TodayInCopenhagen()
		if value != "" {
			parsed, err := actions.ParseSlashDate(value)
			if err != nil {
				m.err = err
				return m, nil
			}
			date = parsed
		}
		return m.showResultWithCapture(func() error {
			return actions.MarkHoliday(m.db, date)
		})

	case phaseBackfillDate:
		if value == "" {
			m.err = fmt.Errorf("date cannot be empty")
			return m, nil
		}
		parsed, err := actions.ParseSlashDate(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.backfillDate = parsed
		m.phase = phaseBackfillTime
		m.inputPrompt = "Time (HH:MM, e.g. 8:30 or 16:00)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "8:00"
		m.err = nil
		return m, nil

	case phaseBackfillTime:
		if value == "" {
			m.err = fmt.Errorf("time cannot be empty")
			return m, nil
		}
		timeStr, err := actions.ParseTimeInput(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.backfillTime = timeStr

		switch m.backfillType {
		case "end":
			return m.showResultWithCapture(func() error {
				return actions.EndDay(m.db, m.backfillDate, m.backfillTime)
			})
		case "lunch":
			return m.showResultWithCapture(func() error {
				return actions.StartLunch(m.db, m.backfillDate, m.backfillTime)
			})
		default:
			m.phase = phaseBackfillName
			m.inputPrompt = "Assignment name"
			m.textInput.SetValue("")
			m.textInput.Placeholder = "e.g. Client meeting"
			m.err = nil
			return m, nil
		}

	case phaseBackfillName:
		if value == "" {
			m.err = fmt.Errorf("assignment name cannot be empty")
			return m, nil
		}
		return m.showResultWithCapture(func() error {
			return actions.StartAssignment(m.db, m.backfillDate, m.backfillTime, value)
		})

	case phasePurgeFrom:
		if value == "" {
			m.err = fmt.Errorf("date cannot be empty")
			return m, nil
		}
		parsed, err := actions.ParseSlashDate(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.purgeFrom = parsed
		m.phase = phasePurgeTo
		m.inputPrompt = fmt.Sprintf("To date (d/m, from: %s)", value)
		m.textInput.SetValue("")
		m.textInput.Placeholder = "10/4"
		m.err = nil
		return m, nil

	case phasePurgeTo:
		if value == "" {
			m.err = fmt.Errorf("date cannot be empty")
			return m, nil
		}
		parsed, err := actions.ParseSlashDate(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		purgeFrom := m.purgeFrom
		return m.showResultWithCapture(func() error {
			return actions.PurgeTimeRange(m.db, purgeFrom, parsed)
		})

	case phaseEditValue:
		if value == "" {
			m.err = fmt.Errorf("value cannot be empty")
			return m, nil
		}
		entry := m.editEntry
		switch m.editField {
		case "name":
			entry.Name = value
		case "start":
			parsed, err := actions.ParseTimeInput(value)
			if err != nil {
				m.err = err
				return m, nil
			}
			entry.StartTime = parsed
		case "end":
			parsed, err := actions.ParseTimeInput(value)
			if err != nil {
				m.err = err
				return m, nil
			}
			entry.EndTime = parsed
		}
		return m.showResultWithCapture(func() error {
			err := actions.UpdateEntryAndResolveOverlaps(m.db, entry)
			if err != nil {
				return err
			}
			return actions.ResyncDay(m.db, entry.Date)
		})
	}

	return m, nil
}

func (m model) handleOptionsSelect(selected string) (tea.Model, tea.Cmd) {
	switch selected {
	case "config":
		return m.showResultWithCapture(func() error {
			return actions.PrintAllConfig(m.db)
		})
	case "sync":
		return m.showResultWithCapture(func() error {
			sheetID, calID := actions.GetGoogleConfig(m.db)
			if sheetID == "" && calID == "" {
				return fmt.Errorf("no spreadsheet or calendar configured")
			}
			entries, err := m.db.GetUnsyncedEntries()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("Everything is up to date.")
				return nil
			}
			fmt.Printf("Syncing %d entries...\n", len(entries))
			w := syncpkg.NewWorker(m.db, 0)
			return w.SyncNow()
		})
	case "daemon":
		return m.showResultWithCapture(func() error {
			return RestartDaemon()
		})
	case "purge":
		m.phase = phasePurgeFrom
		m.inputPrompt = "From date (d/m, e.g. 3/4 for April 3rd)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "3/4"
		m.err = nil
		return m, nil
	case "guide":
		m.phase = phaseResult
		m.result = guideQuickText
		return m, nil
	}
	return m, nil
}

func (m model) handleEditDaySelect(date string) (tea.Model, tea.Cmd) {
	entries, err := m.db.GetEntriesByDate(date)
	if err != nil {
		m.phase = phaseResult
		m.err = err
		return m, nil
	}
	if len(entries) == 0 {
		m.phase = phaseResult
		m.result = "No entries for this day."
		return m, nil
	}
	m.editEntries = entries
	m.phase = phaseEditEntryList
	m.cursor = 0
	m.menuItems = m.entriesToMenuItems()
	return m, nil
}

func (m model) handleEditEntrySelect(idStr string) (tea.Model, tea.Cmd) {
	var id int64
	fmt.Sscanf(idStr, "%d", &id)

	for _, e := range m.editEntries {
		if e.ID == id {
			m.editEntry = e
			break
		}
	}
	m.phase = phaseEditFieldSelect
	m.cursor = 0
	m.menuItems = editFieldOptions
	return m, nil
}

func (m model) handleEditFieldSelect(field string) (tea.Model, tea.Cmd) {
	if field == "delete" {
		return m.showResultWithCapture(func() error {
			err := actions.DeleteEntryAndResync(m.db, m.editEntry)
			if err != nil {
				return err
			}
			return actions.ResyncDay(m.db, m.editEntry.Date)
		})
	}

	m.editField = field
	m.phase = phaseEditValue
	m.textInput.SetValue("")
	m.err = nil

	switch field {
	case "name":
		m.inputPrompt = fmt.Sprintf("New name (current: %s)", m.editEntry.Name)
		m.textInput.Placeholder = m.editEntry.Name
	case "start":
		m.inputPrompt = fmt.Sprintf("New start time (current: %s)", m.editEntry.StartTime)
		m.textInput.Placeholder = m.editEntry.StartTime
	case "end":
		m.inputPrompt = fmt.Sprintf("New end time (current: %s)", m.editEntry.EndTime)
		m.textInput.Placeholder = m.editEntry.EndTime
	}
	return m, nil
}

// captureOutput runs a function and captures its stdout output.
func captureOutput(fn func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String(), err
}

func (m model) showResultWithCapture(fn func() error) (tea.Model, tea.Cmd) {
	output, err := captureOutput(fn)
	m.phase = phaseResult
	m.err = err
	if err == nil {
		m.result = strings.TrimSpace(output)
		if m.result == "" {
			m.result = "Done!"
		}
	}
	return m, nil
}

func (m model) showResult(err error) (tea.Model, tea.Cmd) {
	m.phase = phaseResult
	m.err = err
	if err == nil {
		m.result = "Done!"
	}
	return m, nil
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			// Back to main menu
			m.phase = phaseMenu
			m.cursor = 0
			m.menuItems = mainMenu
			m.err = nil
			m.result = ""
			m.statusHeader = m.buildStatusHeader()
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("TimeReg"))
	b.WriteString("\n\n")

	switch m.phase {
	case phaseMenu:
		b.WriteString(m.statusHeader)
		b.WriteString("\n")
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • q quit"))

	case phaseBackfillType:
		b.WriteString("What type of entry?\n\n")
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • esc back"))

	case phaseOptionsMenu:
		b.WriteString("Options:\n\n")
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • esc back"))

	case phaseEditDayList:
		b.WriteString("Select a day to edit:\n\n")
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • esc back"))

	case phaseEditEntryList:
		date := ""
		if len(m.editEntries) > 0 {
			date = m.editEntries[0].Date
		}
		b.WriteString(fmt.Sprintf("Entries for %s:\n\n", date))
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • esc back"))

	case phaseEditFieldSelect:
		b.WriteString(fmt.Sprintf("Edit %s (%s-%s):\n\n", m.editEntry.DisplayName(), m.editEntry.StartTime, m.editEntry.EndTime))
		for i, item := range m.menuItems {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s", item.label)))
			} else {
				b.WriteString(fmt.Sprintf("    %s", item.label))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("\n  ↑/↓ navigate • enter select • esc back"))

	case phaseInput, phaseBackfillDate, phaseBackfillTime, phaseBackfillName, phaseHolidayDate, phaseEditValue, phasePurgeFrom, phasePurgeTo:
		b.WriteString(fmt.Sprintf("%s:\n\n", m.inputPrompt))
		b.WriteString("  " + m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("  Error: %s", m.err)))
		}
		b.WriteString(dimStyle.Render("\n\n  enter submit • tab autocomplete • esc back"))

	case phaseResult:
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.err)))
		} else {
			b.WriteString(resultStyle.Render(m.result))
		}
		b.WriteString(dimStyle.Render("\n\n  enter continue • q quit"))
	}

	b.WriteString("\n")
	return b.String()
}
