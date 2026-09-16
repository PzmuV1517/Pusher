package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/follower"
	tea "github.com/charmbracelet/bubbletea"
)

// Follower logs are listed the way profiles and power traces are, because they
// are the same shape of thing: recordings on the robot, chosen by when they
// happened, pulled when one is picked. Three lists that behaved differently
// would be three things to learn rather than one.
//
// The difference is what having none of them means. A competition build cannot
// record at all, so an empty list is the normal state rather than a problem,
// and the two ways of having none go to completely different places.

type followerState struct {
	busy    bool
	err     error
	connect connectOffer

	serial string
	runs   []follower.Run
}

type followerListMsg struct {
	serial string
	runs   []follower.Run
	err    error
}

type followerPageMsg struct {
	path string
	err  error
}

// followerConnectedMsg is the outcome of the menu going and getting the robot.
type followerConnectedMsg struct {
	err error
}

func (m *SettingsModel) enterFollower() tea.Cmd {
	m.follower = followerState{busy: true}
	m.goTo(screenBlobFollower, 0)

	return func() tea.Msg {
		serial, err := adb.Target()
		if err != nil {
			return followerListMsg{err: err}
		}

		runs, err := follower.List(serial)
		return followerListMsg{serial: serial, runs: runs, err: err}
	}
}

// openFollowerRun pulls one log and draws it.
//
// Pulled every time rather than cached. The recorder rewrites the whole file
// every couple of seconds while the run is going, so the same log asked for
// twice is two different files, and the second one is the one with more of the
// run in it.
func (m *SettingsModel) openFollowerRun(run follower.Run) tea.Cmd {
	serial := m.follower.serial
	m.follower.busy = true

	return func() tea.Msg {
		local := filepath.Join(os.TempDir(), run.Name)

		log, err := follower.Pull(serial, run, local)
		if err != nil {
			return followerPageMsg{err: err}
		}

		path, err := log.Render("")
		return followerPageMsg{path: path, err: err}
	}
}

func (m *SettingsModel) updateFollower(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenBlob, blobIndex("Follower logs"))
		m.status = ""

	case "r":
		return m, m.enterFollower()

	case "c":
		if !m.follower.connect.open || m.follower.busy {
			return m, nil
		}

		m.follower.busy = true
		m.follower.connect.busy = true
		m.follower.err = nil

		return m, connect(func(err error) tea.Msg { return followerConnectedMsg{err: err} })

	case "up", "k":
		m.moveCursor(-1, len(m.follower.runs))
	case "down", "j":
		m.moveCursor(1, len(m.follower.runs))

	case "enter", " ":
		if m.follower.busy || len(m.follower.runs) == 0 {
			return m, nil
		}
		return m, m.openFollowerRun(m.follower.runs[m.cursor])
	}

	return m, nil
}

func (m *SettingsModel) viewFollower() string {
	var b strings.Builder

	if m.follower.busy {
		doing := "Reading the robot..."
		if m.follower.connect.busy {
			doing = m.follower.connect.working()
		}

		b.WriteString(helpStyle.Render("  "+fit(doing, textWidth(m.width))) + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.follower.connect.open {
		for _, line := range wrap(m.follower.connect.hint(), textWidth(m.width)) {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("c connect · r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	if m.follower.err != nil {
		// Having no logs is not a failure. One of these robots is running a
		// build that physically cannot record and the other was not asked to,
		// and neither of them is adb being broken, so neither is painted red.
		style := errStyle
		if nothingToShow(m.follower.err) {
			style = helpStyle
		}

		for _, line := range wrap(m.follower.err.Error(), textWidth(m.width)) {
			b.WriteString(style.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	b.WriteString(helpStyle.Render("  "+fit("One run each, newest first. Pulled when you pick one.", textWidth(m.width))) + "\n\n")

	return m.fill(b.String(),
		"\n"+helpStyle.Render("  "+fit("enter opens it in a browser · r refresh · esc back", textWidth(m.width)))+"\n",
		len(m.follower.runs), func(i int) string {
			run := m.follower.runs[i]
			return renderRow(i == m.cursor, run.Label(), run.Detail(), 29, m.width)
		})
}

// nothingToShow reports whether an error is one of the two ordinary ways for a
// robot to have no logs on it.
func nothingToShow(err error) bool {
	return errors.Is(err, follower.ErrNoRecorder) || errors.Is(err, follower.ErrNothingRecorded)
}
