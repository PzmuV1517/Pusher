package tui

import (
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/power"
	tea "github.com/charmbracelet/bubbletea"
)

// Reading a recording means talking to the robot, which is slow enough to be
// noticed. Both steps run as commands rather than inline, so the menu says what
// it is doing instead of freezing while adb works.

type powerState struct {
	busy bool
	err  error

	serial string
	runs   []power.Recording

	report power.Report
	shown  bool
}

type powerListMsg struct {
	serial string
	runs   []power.Recording
	err    error
}

type powerReportMsg struct {
	report power.Report
	err    error
}

func (m *SettingsModel) enterPower() tea.Cmd {
	m.power = powerState{busy: true}
	m.goTo(screenPower, 0)

	return func() tea.Msg {
		serial, err := adb.Target()
		if err != nil {
			return powerListMsg{err: err}
		}

		runs, err := power.List(serial)
		return powerListMsg{serial: serial, runs: runs, err: err}
	}
}

func (m *SettingsModel) openRun(rec power.Recording) tea.Cmd {
	serial := m.power.serial
	m.power.busy = true

	return func() tea.Msg {
		report, err := power.Read(serial, rec)
		return powerReportMsg{report: report, err: err}
	}
}

func (m *SettingsModel) updatePower(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenMain, 0)
		m.status = ""

	case "r":
		return m, m.enterPower()

	case "up", "k":
		m.moveCursor(-1, len(m.power.runs))
	case "down", "j":
		m.moveCursor(1, len(m.power.runs))

	case "enter", " ":
		if m.power.busy || len(m.power.runs) == 0 {
			return m, nil
		}
		return m, m.openRun(m.power.runs[m.cursor])
	}

	return m, nil
}

func (m *SettingsModel) updatePowerReport(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.power.shown = false
		m.goTo(screenPower, m.cursor)
	}
	return m, nil
}

func (m *SettingsModel) viewPower() string {
	var b strings.Builder

	if m.power.busy {
		b.WriteString(helpStyle.Render("  "+fit("Reading the robot...", textWidth(m.width))) + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.power.err != nil {
		for _, line := range wrap(m.power.err.Error(), textWidth(m.width)) {
			b.WriteString(errStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if len(m.power.runs) == 0 {
		notice := "No recordings on the robot yet. Turn the power monitor on, deploy, " +
			"run an OpMode and stop it: the recording is written when it stops."

		if !power.Installed(m.projectRoot()) {
			notice = "The power monitor is not installed in this project, so nothing is " +
				"being recorded. Turn it on in the entry above this one, then deploy."
		}

		for _, line := range wrap(notice, textWidth(m.width)) {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r refresh · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	b.WriteString(helpStyle.Render("  "+fit("One run each, newest first.", textWidth(m.width))) + "\n\n")

	b.WriteString(m.renderList(len(m.power.runs), func(i int) string {
		run := m.power.runs[i]
		return renderRow(i == m.cursor, run.OpMode, run.When.Format("15:04:05"), 29, m.width)
	}))

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter open · r refresh · esc back", textWidth(m.width))) + "\n")
	return b.String()
}

func (m *SettingsModel) viewPowerReport() string {
	var b strings.Builder

	if m.power.err != nil {
		for _, line := range wrap(m.power.err.Error(), textWidth(m.width)) {
			b.WriteString(errStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	lines := m.power.report.Lines()

	b.WriteString(titleStyle.Render("  "+fit(m.power.report.Title(), textWidth(m.width))) + "\n\n")

	b.WriteString(m.renderList(len(lines), func(i int) string {
		return "  " + fit(lines[i], textWidth(m.width)) + "\n"
	}))

	b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
	return b.String()
}
