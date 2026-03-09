package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rlf/time_register_cli/internal/actions"
	"github.com/rlf/time_register_cli/internal/db"
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
	{label: "Reopen Day", value: "reopen"},
	{label: "Backfill", value: "backfill"},
	{label: "Holiday", value: "holiday"},
	{label: "Status", value: "status"},
	{label: "Config", value: "config"},
	{label: "Setup Guide", value: "guide"},
}

var backfillTypes = []menuItem{
	{label: "Start Assignment", value: "assignment"},
	{label: "Lunch", value: "lunch"},
	{label: "End Day", value: "end"},
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

	// backfill state
	backfillType string
	backfillDate string
	backfillTime string
}

func newModel(d *db.DB) model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	return model{
		db:        d,
		phase:     phaseMenu,
		menuItems: mainMenu,
		textInput: ti,
	}
}

func RunTUI(d *db.DB) error {
	p := tea.NewProgram(newModel(d))
	_, err := p.Run()
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
			m.phase = phaseMenu
			m.cursor = 0
			m.menuItems = mainMenu
			m.err = nil
			return m, nil
		}
	}

	switch m.phase {
	case phaseMenu, phaseBackfillType:
		return m.updateMenu(msg)
	case phaseInput, phaseBackfillDate, phaseBackfillTime, phaseBackfillName, phaseHolidayDate:
		return m.updateInput(msg)
	case phaseResult:
		return m.updateResult(msg)
	}

	return m, nil
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

	if m.phase == phaseBackfillType {
		m.backfillType = selected
		m.phase = phaseBackfillDate
		m.inputPrompt = "Date (d/m, e.g. 3/4 for April 3rd)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "3/4"
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
		err := actions.StartLunch(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		return m.showResult(err)

	case "end":
		err := actions.EndDay(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		return m.showResult(err)

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
		err := actions.PrintStatus(m.db, actions.TodayInCopenhagen())
		return m.showResult(err)

	case "reopen":
		err := actions.ReopenDay(m.db, actions.TodayInCopenhagen())
		return m.showResult(err)

	case "config":
		err := actions.PrintAllConfig(m.db)
		return m.showResult(err)

	case "guide":
		m.phase = phaseResult
		m.result = guideQuickText
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
		err := actions.StartAssignment(m.db, actions.TodayInCopenhagen(), actions.NowInCopenhagen(), value)
		return m.showResult(err)

	case phaseHolidayDate:
		date := actions.TodayInCopenhagen()
		if value != "" {
			parsed, err := parseSlashDate(value)
			if err != nil {
				m.err = err
				return m, nil
			}
			date = parsed
		}
		err := actions.MarkHoliday(m.db, date)
		return m.showResult(err)

	case phaseBackfillDate:
		if value == "" {
			m.err = fmt.Errorf("date cannot be empty")
			return m, nil
		}
		parsed, err := parseSlashDate(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.backfillDate = parsed
		m.phase = phaseBackfillTime
		m.inputPrompt = "Time (HH:MM, e.g. 8:30 or 16:00)"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "8:30"
		m.err = nil
		return m, nil

	case phaseBackfillTime:
		if value == "" {
			m.err = fmt.Errorf("time cannot be empty")
			return m, nil
		}
		// Accept both HH:MM and HHMM
		timeStr := value
		if !strings.Contains(timeStr, ":") {
			parsed, err := parseBacklogTime(timeStr)
			if err != nil {
				m.err = err
				return m, nil
			}
			timeStr = parsed
		}
		m.backfillTime = timeStr

		switch m.backfillType {
		case "end":
			err := actions.EndDay(m.db, m.backfillDate, m.backfillTime)
			return m.showResult(err)
		case "lunch":
			err := actions.StartLunch(m.db, m.backfillDate, m.backfillTime)
			return m.showResult(err)
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
		err := actions.StartAssignment(m.db, m.backfillDate, m.backfillTime, value)
		return m.showResult(err)
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
		b.WriteString("What would you like to do?\n\n")
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

	case phaseInput, phaseBackfillDate, phaseBackfillTime, phaseBackfillName, phaseHolidayDate:
		b.WriteString(fmt.Sprintf("%s:\n\n", m.inputPrompt))
		b.WriteString("  " + m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("  Error: %s", m.err)))
		}
		b.WriteString(dimStyle.Render("\n\n  enter submit • esc back"))

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
