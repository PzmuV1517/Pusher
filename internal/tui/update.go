package tui

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/selfupdate"
	tea "github.com/charmbracelet/bubbletea"
)

type updateState struct {
	install selfupdate.Install
	release selfupdate.Release

	checking bool
	busy     bool
	done     bool
	result   string
	err      error
}

type releaseFoundMsg struct {
	release selfupdate.Release
	err     error
}

type updateAppliedMsg struct {
	result string
	err    error
}

func findRelease() tea.Msg {
	release, err := selfupdate.Latest()
	return releaseFoundMsg{release: release, err: err}
}

func applyUpdate(install selfupdate.Install, release selfupdate.Release) tea.Cmd {
	return func() tea.Msg {
		if install.Method == selfupdate.Homebrew {
			out, err := selfupdate.UpgradeBrew(install.Formula, release.Version())
			return updateAppliedMsg{result: selfupdate.LastLine(out), err: err}
		}

		if err := selfupdate.Apply(release, install.Path); err != nil {
			return updateAppliedMsg{err: err}
		}
		return updateAppliedMsg{result: "Installed " + release.Tag}
	}
}

func (m *SettingsModel) enterUpdate() tea.Cmd {
	install, err := selfupdate.Detect()

	m.update = updateState{install: install, err: err, checking: err == nil}
	m.goTo(screenUpdate, 0)

	if err != nil {
		return nil
	}
	return findRelease
}

func (m *SettingsModel) updateUpdate(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		if m.update.busy {

			return m, nil
		}
		m.goTo(screenMain, 0)
		m.status = ""

	case "enter", " ", "right", "l":
		if m.update.busy || m.update.checking || m.update.done {
			return m, nil
		}
		if m.update.err != nil || !m.update.release.Newer() {
			return m, nil
		}

		m.update.busy = true
		m.update.err = nil
		return m, applyUpdate(m.update.install, m.update.release)
	}

	return m, nil
}

func (m *SettingsModel) viewUpdate() string {
	var b strings.Builder

	via := m.update.install.Method.String()
	if m.update.install.Formula != "" {
		via += " (formula " + m.update.install.Formula + ")"
	}

	b.WriteString(renderField("Installed via", via, m.width))
	b.WriteString(renderField("Location", m.update.install.Path, m.width))
	b.WriteString(renderField("Running", selfupdate.Current(), m.width))

	switch {
	case m.update.checking:
		b.WriteString(renderField("Latest", "checking...", m.width))
	case m.update.err != nil && m.update.release.Tag == "":
		b.WriteString(renderField("Latest", "unavailable", m.width))
	default:
		b.WriteString(renderField("Latest", m.update.release.Tag, m.width))
	}

	b.WriteString("\n")

	switch {
	case m.update.err != nil:
		b.WriteString(errStyle.Render("  "+m.update.err.Error()) + "\n")
	case m.update.busy:
		b.WriteString(okStyle.Render("  Updating, this can take a moment...") + "\n")
	case m.update.done:
		b.WriteString(okStyle.Render("  "+m.update.result) + "\n")
		b.WriteString(helpStyle.Render("  Restart pusher to run the new version.") + "\n")
	case m.update.checking:
		b.WriteString(helpStyle.Render("  Looking for a newer release...") + "\n")
	case !m.update.release.Newer():
		b.WriteString(okStyle.Render("  Already up to date.") + "\n")
	default:
		b.WriteString(valueStyle.Render(
			fmt.Sprintf("  %s is available.", m.update.release.Tag)) + "\n")
		if m.update.install.Method == selfupdate.Homebrew {
			b.WriteString(helpStyle.Render("  enter hands this to brew upgrade.") + "\n")
		} else {
			b.WriteString(helpStyle.Render("  "+fit("enter replaces this binary in place.", textWidth(m.width))) + "\n")
		}
	}

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter update · esc back", textWidth(m.width))) + "\n")
	return b.String()
}

// The label is padded to a fixed column, so on a narrow terminal the value has
// to be cut to what is left rather than pushed past the edge.
func renderField(label, value string, width int) string {
	const column = 16

	room := textWidth(width) - column - 1
	if room < 1 {
		room = 1
	}

	if value == "" {
		value = unsetStyle.Render(fit("unknown", room))
	} else {
		value = valueStyle.Render(fit(value, room))
	}
	return fmt.Sprintf("  %-*s %s\n", column, fit(label, column), value)
}

func (m *SettingsModel) updateLabel() string {
	return selfupdate.Current()
}
