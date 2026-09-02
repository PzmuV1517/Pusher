package tui

import (
	"testing"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/power"
	"github.com/andreibanu/pusher/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// Going down a list and back up was enough to break these menus, and the shape
// of it was always the same: the block grew a row the moment a scroll marker
// appeared and lost one again at either end, so the whole view changed height
// as the cursor moved. The footer walks up and down the screen, and a frame
// that had been taller leaves its bottom rows behind under the shorter one.
//
// Down only was already tested and passed, because the first marker to appear
// is the one at the bottom and it appears on the very first frame. It takes
// coming back up for the height to change.
func TestAMenuIsTheSameHeightGoingUpAsGoingDown(t *testing.T) {
	for _, height := range []int{8, 10, 12, 14, 16, 20, 24, 40} {
		for _, width := range []int{32, 40, 60, 80, 120} {
			for _, sc := range settingsScreens() {
				m := &SettingsModel{height: height, width: width, confirmDeleteIndex: -1}
				m.refreshProfiles()
				m.networks = []string{"ICHB-Robotics", "14270-RC", "another-network", "x"}
				sc.enter(m)

				first, ok := -1, true
				walk(m, func() {
					rows := renderedRows(m.View(), width)

					// Not merely fitting: a frame exactly as tall as the
					// window scrolls the terminal as its last line is written,
					// and the renderer then repaints one row out and leaves the
					// previous frame's row on screen. That is the duplicated
					// entry, so the invariant is one row of slack, not none.
					if rows >= height {
						t.Errorf("%s at %dx%d: %d rows leaves no slack in a %d row window",
							sc.name, width, height, rows, height)
						ok = false
						return
					}
					if first < 0 {
						first = rows
						return
					}
					if rows != first && ok {
						t.Errorf("%s at %dx%d: view was %d rows and is now %d",
							sc.name, width, height, first, rows)
						ok = false
					}
				})
			}
		}
	}
}

// The hardware configuration menu had the same fault and its own copy of the
// layout code, so it needed the same fix rather than inheriting one.
func TestTheHardwareMenuIsTheSameHeightToo(t *testing.T) {
	for _, height := range []int{8, 10, 12, 16, 20, 24, 40} {
		for _, width := range []int{32, 40, 60, 80, 120} {
			for _, deep := range []bool{false, true} {
				m := hwModelIn(t)
				m.height, m.width = height, width
				m.robot = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
				m.rebuildEntries()

				if deep {
					m.sel, m.screen, m.cursor = "comp", hwScreenDevices, 0
					for i := 0; i < 14; i++ {
						m.rows = append(m.rows, hwRow{Kind: hwRowDevice, Label: "motor", Detail: "port 0"})
					}
				}

				first, ok := -1, true
				walkHW(m, func() {
					rows := renderedRows(m.View(), width)

					if rows >= height {
						t.Errorf("hwconfig deep=%v at %dx%d: %d rows leaves no slack in a %d row window",
							deep, width, height, rows, height)
						ok = false
						return
					}
					if first < 0 {
						first = rows
						return
					}
					if rows != first && ok {
						t.Errorf("hwconfig deep=%v at %dx%d: view was %d rows and is now %d",
							deep, width, height, first, rows)
						ok = false
					}
				})
			}
		}
	}
}

// A status message must not move the menu either: the room for one is always
// reserved, so toggling a setting changes the message and nothing else.
func TestAStatusMessageDoesNotMoveTheMenu(t *testing.T) {
	for _, width := range []int{32, 60, 80} {
		m := &SettingsModel{height: 20, width: width, confirmDeleteIndex: -1}
		m.refreshProfiles()
		m.screen = screenDeploy

		quiet := renderedRows(m.View(), width)

		m.status = "On: pushes will package arm64-v8a only"
		withStatus := renderedRows(m.View(), width)

		m.status = ""
		m.err = errLong{}
		withError := renderedRows(m.View(), width)

		if quiet != withStatus || quiet != withError {
			t.Errorf("at width %d the menu is %d rows quiet, %d with a status and %d with an error",
				width, quiet, withStatus, withError)
		}
	}
}

type errLong struct{}

func (errLong) Error() string {
	return "a long message about something that went wrong, long enough to need cutting on a narrow terminal"
}

func settingsScreens() []struct {
	name  string
	enter func(*SettingsModel)
} {
	return []struct {
		name  string
		enter func(*SettingsModel)
	}{
		{"main", func(m *SettingsModel) { m.screen = screenMain }},
		{"profiles", func(m *SettingsModel) { m.screen = screenProfiles }},
		{"home network", func(m *SettingsModel) { m.screen = screenHomeNetwork }},
		{"deploy", func(m *SettingsModel) { m.screen = screenDeploy }},
		{"extreme", func(m *SettingsModel) { m.screen = screenExtreme }},
		{"blob", func(m *SettingsModel) { m.screen = screenBlob }},
		{"blob branches", func(m *SettingsModel) {
			m.screen = screenBlobBranch
			m.blob.branches = []string{"main", "RSTController", "feedforward", "a-branch-with-a-very-long-name"}
		}},
		{"adb relay", func(m *SettingsModel) {
			m.screen = screenRelay
			for i := 0; i < 8; i++ {
				m.relay.spots = append(m.relay.spots, config.Spot{Network: "net", Address: "10.0.0.42:5555"})
			}
		}},
		{"loop profiles", func(m *SettingsModel) {
			m.screen = screenProfile
			for i := 0; i < 12; i++ {
				m.profile.runs = append(m.profile.runs, profile.Recording{Name: "TeleOP.txt", OpMode: "TeleOP"})
			}
		}},
		{"power runs", func(m *SettingsModel) {
			m.screen = screenPower
			for i := 0; i < 12; i++ {
				m.power.runs = append(m.power.runs, power.Recording{Name: "TeleOP.txt", OpMode: "TeleOP"})
			}
		}},
	}
}

// walk goes all the way down a list and all the way back up, rendering at every
// stop.
func walk(m *SettingsModel, check func()) {
	for i := 0; i < 14; i++ {
		check()
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	for i := 0; i < 14; i++ {
		check()
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
}

func walkHW(m *hwModel, check func()) {
	for i := 0; i < 14; i++ {
		check()
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	for i := 0; i < 14; i++ {
		check()
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
}

// Bubbletea draws one frame before it handles the first resize. A model that
// starts out believing it has twenty four rows draws twenty four rows into
// whatever window it actually has, and in a fourteen row panel that scrolls the
// terminal ten rows out from under the renderer, which has been counting from
// where it started. Every repaint after that lands at the wrong height and
// leaves rows of old frames behind, before a key has been pressed.
func TestNothingIsDrawnBeforeTheTerminalSaysHowBigItIs(t *testing.T) {
	settings := &SettingsModel{confirmDeleteIndex: -1}
	settings.refreshProfiles()

	if got := settings.View(); got != "" {
		t.Errorf("the settings menu drew %d rows before it knew the window:\n%s",
			renderedRows(got, defaultWidth), got)
	}

	// And it draws once it is told.
	settings.Update(tea.WindowSizeMsg{Width: 60, Height: 14})
	if settings.View() == "" {
		t.Error("the settings menu drew nothing after being given a size")
	}
	if got := renderedRows(settings.View(), 60); got >= 14 {
		t.Errorf("the first sized frame is %d rows in a 14 row window", got)
	}

	hw := &hwModel{}
	if got := hw.View(); got != "" {
		t.Errorf("the hardware menu drew before it knew the window:\n%s", got)
	}

	dev := &devModel{}
	if got := dev.View(); got != "" {
		t.Errorf("the developer menu drew before it knew the window:\n%s", got)
	}
}

// The models must not carry a size they were never given, or the guard above is
// dead code and the first frame is drawn to a window that does not exist.
func TestAMenuStartsWithNoSize(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height func() int
	}{
		// Through the constructors the commands actually use, not through a
		// literal: a default put back there is exactly the bug, and a test that
		// builds its own model would never see it.
		{"settings", func() int {
			m, err := NewSettingsModel()
			if err != nil {
				t.Fatal(err)
			}
			return m.height
		}},
		{"hardware", func() int { return newHWModel(t.TempDir()).height }},
		{"developer", func() int { return newDevModel("", "", nil).height }},
	} {
		if got := tc.height(); got != 0 {
			t.Errorf("the %s menu starts out believing it has %d rows", tc.name, got)
		}
	}
}
