package tui

import (
	"github.com/andreibanu/pusher/internal/power"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A terminal narrower than the layout wants is still a terminal. Every guard
// used to treat one as eighty columns instead, so rows were padded past the
// edge, wrapped, and the view ran off the bottom of the screen leaving the
// frame under it on display: scrolling a small window showed duplicates.
func TestNoScreenOverflowsASmallTerminal(t *testing.T) {
	for _, width := range []int{24, 28, 30, 32, 34, 40, 50, 60, 72, 80, 120} {
		for _, height := range []int{8, 10, 12, 16, 20, 24, 40} {
			for _, screen := range []struct {
				name  string
				enter func(*SettingsModel)
			}{
				{"main", func(m *SettingsModel) { m.screen = screenMain }},
				{"profiles", func(m *SettingsModel) { m.screen = screenProfiles }},
				{"home network", func(m *SettingsModel) { m.screen = screenHomeNetwork }},
				{"threads", func(m *SettingsModel) { m.screen = screenThreads }},
				{"deploy", func(m *SettingsModel) { m.screen = screenDeploy }},
				{"extreme", func(m *SettingsModel) { m.screen = screenExtreme }},
				{"update", func(m *SettingsModel) { m.screen = screenUpdate }},
				{"blob", func(m *SettingsModel) { m.screen = screenBlob }},
				{"blob branches", func(m *SettingsModel) {
					m.screen = screenBlobBranch
					m.blob.branches = []string{"main", "RSTController", "feedforward", "a-branch-with-a-very-long-name"}
				}},
				{"power runs", func(m *SettingsModel) {
					m.screen = screenPower
					m.power.runs = []power.Recording{
						{Name: "TeleOP-1756089600000.txt", OpMode: "TeleOP"},
						{Name: "AVeryLongAutonomousOpModeName-1756089600000.txt", OpMode: "AVeryLongAutonomousOpModeName"},
					}
				}},
				{"power, nothing there", func(m *SettingsModel) {
					m.screen = screenPower
					m.power.runs = nil
				}},
				{"blob branches, still looking", func(m *SettingsModel) {
					m.screen = screenBlobBranch
					m.blob.branchBusy = true
				}},
			} {
				m := &SettingsModel{height: height, width: width, confirmDeleteIndex: -1}
				m.refreshProfiles()
				m.networks = []string{"ICHB-Robotics", "14270-RC", "a-very-long-network-name-indeed", "x"}
				screen.enter(m)

				for step := 0; step < 16; step++ {
					view := m.View()

					if got := renderedRows(view, width); got > height {
						t.Errorf("%s at %dx%d, step %d: %d rows of %d",
							screen.name, width, height, step, got, height)
						break
					}

					for _, line := range strings.Split(view, "\n") {
						if got := lipgloss.Width(line); got > width {
							t.Errorf("%s at %dx%d, step %d: a line is %d wide: %q",
								screen.name, width, height, step, got, stripANSI(line))
							break
						}
					}

					m.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
			}
		}
	}
}

// fill has to budget in rows the terminal will use, not newlines, so a row that
// wraps costs the window an entry instead of costing the screen its bottom.
// Every row a screen renders today is fitted and so never wraps, which is why
// nothing else here notices: this is what stops the next unfitted one from
// pushing the footer off the screen.
func TestFillBudgetsWrappedRowsNotNewlines(t *testing.T) {
	const width, tall = 40, 12

	m := &SettingsModel{height: tall, width: width, confirmDeleteIndex: -1}

	// Three terminal rows each, with one newline, which is the case counting
	// newlines gets wrong.
	long := strings.Repeat("x", width*3-1) + "\n"

	view := m.fill("", "\n  footer\n", 8, func(int) string { return long })

	if got := renderedRows(view, width); got > tall {
		t.Errorf("fill rendered %d rows of %d", got, tall)
	}
	if !strings.Contains(view, "footer") {
		t.Error("the footer was pushed off the screen by rows that did not fit")
	}
}
