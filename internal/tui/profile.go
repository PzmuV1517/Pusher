package tui

import (
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// The profile viewer is the power viewer with a different noun in it, written
// the same way on purpose: two lists of recordings that behaved differently
// would be two things to learn rather than one.
//
// Reading a recording means talking to the robot, which is slow enough to be
// noticed. Both steps run as commands rather than inline, so the menu says what
// it is doing instead of freezing while adb works.

type profileState struct {
	busy    bool
	err     error
	connect connectOffer

	serial string
	runs   []profile.Recording
}

type profileListMsg struct {
	serial string
	runs   []profile.Recording
	err    error
}

type profileReportMsg struct {
	path string
	err  error
}

// profileConnectedMsg is the outcome of the menu going and getting the robot.
type profileConnectedMsg struct {
	err error
}

func (m *SettingsModel) enterProfile() tea.Cmd {
	m.profile = profileState{busy: true}
	m.goTo(screenProfile, 0)

	return func() tea.Msg {
		serial, err := adb.Target()
		if err != nil {
			return profileListMsg{err: err}
		}

		runs, err := profile.List(serial)
		return profileListMsg{serial: serial, runs: runs, err: err}
	}
}

func (m *SettingsModel) openProfileRun(rec profile.Recording) tea.Cmd {
	serial := m.profile.serial
	m.profile.busy = true

	return func() tea.Msg {
		report, err := profile.Read(serial, rec)
		if err != nil {
			return profileReportMsg{err: err}
		}

		path, err := report.Render("")
		return profileReportMsg{path: path, err: err}
	}
}

func (m *SettingsModel) updateProfile(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenMain, 0)
		m.status = ""

	case "r":
		return m, m.enterProfile()

	case "c":
		if !m.profile.connect.open || m.profile.busy {
			return m, nil
		}

		m.profile.busy = true
		m.profile.connect.busy = true
		m.profile.err = nil

		return m, connect(func(err error) tea.Msg { return profileConnectedMsg{err: err} })

	case "up", "k":
		m.moveCursor(-1, len(m.profile.runs))
	case "down", "j":
		m.moveCursor(1, len(m.profile.runs))

	case "enter", " ":
		if m.profile.busy || len(m.profile.runs) == 0 {
			return m, nil
		}
		return m, m.openProfileRun(m.profile.runs[m.cursor])
	}

	return m, nil
}

func (m *SettingsModel) viewProfile() string {
	var b strings.Builder

	if m.profile.busy {
		doing := "Reading the robot..."
		if m.profile.connect.busy {
			doing = m.profile.connect.working()
		}

		b.WriteString(helpStyle.Render("  "+fit(doing, textWidth(m.width))) + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.profile.connect.open {
		for _, line := range wrap(m.profile.connect.hint(), textWidth(m.width)) {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		if m.profile.err != nil {
			for _, line := range wrap(m.profile.err.Error(), textWidth(m.width)) {
				b.WriteString(errStyle.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("c connect · r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.profile.err != nil {
		for _, line := range wrap(m.profile.err.Error(), textWidth(m.width)) {
			b.WriteString(errStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if len(m.profile.runs) == 0 {
		notice := "No profiles on the robot yet. Turn the loop profiler on, deploy, " +
			"run an OpMode and stop it: the profile is written when it stops."

		if !profile.Installed(m.projectRoot()) {
			notice = "The loop profiler is not installed in this project, so nothing is " +
				"being recorded. Turn it on in the entry above this one, then deploy."
		}

		for _, line := range wrap(notice, textWidth(m.width)) {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r refresh · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	b.WriteString(helpStyle.Render("  "+fit("One run each, newest first.", textWidth(m.width))) + "\n\n")

	return m.fill(b.String(),
		"\n"+helpStyle.Render("  "+fit("enter opens it in a browser · r refresh · esc back", textWidth(m.width)))+"\n",
		len(m.profile.runs), func(i int) string {
			run := m.profile.runs[i]
			return renderRow(i == m.cursor, run.OpMode, run.When.Format("15:04:05"), 29, m.width)
		})
}
