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
	busy    bool
	err     error
	connect connectOffer

	serial string
	runs   []power.Recording
}

type powerListMsg struct {
	serial string
	runs   []power.Recording
	err    error
}

type powerReportMsg struct {
	path string
	err  error
}

// powerConnectedMsg is the outcome of the menu going and getting the robot.
type powerConnectedMsg struct {
	err error
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
		if err != nil {
			return powerReportMsg{err: err}
		}

		path, err := report.Render("")
		return powerReportMsg{path: path, err: err}
	}
}

func (m *SettingsModel) updatePower(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenMain, 0)
		m.status = ""

	case "r":
		return m, m.enterPower()

	case "c":
		if !m.power.connect.open || m.power.busy {
			return m, nil
		}

		m.power.busy = true
		m.power.connect.busy = true
		m.power.err = nil

		return m, connect(func(err error) tea.Msg { return powerConnectedMsg{err: err} })

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

func (m *SettingsModel) viewPower() string {
	var b strings.Builder

	if m.power.busy {
		doing := "Reading the robot..."
		if m.power.connect.busy {
			doing = m.power.connect.working()
		}

		b.WriteString(helpStyle.Render("  "+fit(doing, textWidth(m.width))) + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.power.connect.open {
		for _, line := range wrap(m.power.connect.hint(), textWidth(m.width)) {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		if m.power.err != nil {
			for _, line := range wrap(m.power.err.Error(), textWidth(m.width)) {
				b.WriteString(errStyle.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("c connect · r retry · esc back", textWidth(m.width))) + "\n")
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

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter opens it in a browser · r refresh · esc back", textWidth(m.width))) + "\n")
	return b.String()
}
