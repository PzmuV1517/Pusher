package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestMainMenuValueColumnClearsLongestLabel(t *testing.T) {
	const mainMenuWidth = 29

	longest := 0
	for _, item := range mainItems {
		if n := len(item); n > longest {
			longest = n
		}
	}

	if longest >= mainMenuWidth {
		t.Fatalf("longest menu label is %d chars but the value column starts at %d; widen it", longest, mainMenuWidth)
	}
}

func TestRenderRowAlignsValuesRegardlessOfSelection(t *testing.T) {
	plain := renderRow(false, "Gradle threads", "8", 29, defaultWidth)
	selected := renderRow(true, "Slim APK before every push", "off", 29, defaultWidth)

	valueColumn := func(row string) int {
		return lipgloss.Width(row[:strings.LastIndex(row, "\n")]) - visibleValueLen(row)
	}

	if got := valueColumn(plain); got != valueColumn(selected) {
		t.Errorf("value column moved between rows: %d vs %d", got, valueColumn(selected))
	}
}

func visibleValueLen(row string) int {
	fields := strings.Fields(stripANSI(row))
	if len(fields) == 0 {
		return 0
	}
	return len(fields[len(fields)-1])
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false

	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && (r == 'm' || r == 'K'):
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func TestRenderRowOmitsPaddingWhenThereIsNoValue(t *testing.T) {
	row := stripANSI(renderRow(false, "Exit", "", 29, defaultWidth))
	if strings.TrimRight(row, "\n") != "   Exit" {
		t.Errorf("a valueless row should not be padded, got %q", row)
	}
}

func TestClampOffsetScrollsOneRowAtATime(t *testing.T) {
	const (
		total   = 20
		visible = 8
	)

	tests := []struct {
		name       string
		offset     int
		cursor     int
		wantOffset int
	}{
		{"opens at the top", 0, 0, 0},
		{"moving inside the window does not scroll", 0, 5, 0},
		{"last visible row still does not scroll", 0, 7, 0},
		{"stepping past the bottom shifts by one", 0, 8, 1},
		{"and again by one", 1, 9, 2},
		{"scrolling back up above the window", 5, 4, 4},
		{"wrapping to the first row returns to the top", 12, 0, 0},
		{"wrapping to the last row shows the final screenful", 0, 19, 12},
		{"offset never runs past the end", 18, 19, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampOffset(tt.offset, tt.cursor, visible, total)
			if got != tt.wantOffset {
				t.Errorf("clampOffset(%d, %d, %d, %d) = %d, want %d",
					tt.offset, tt.cursor, visible, total, got, tt.wantOffset)
			}
		})
	}
}

func TestClampOffsetNeverScrollsAListThatFits(t *testing.T) {
	for cursor := 0; cursor < 5; cursor++ {
		if got := clampOffset(0, cursor, 8, 5); got != 0 {
			t.Errorf("a 5-row list in an 8-row window must not scroll, got offset %d", got)
		}
	}
}

func TestClampOffsetAlwaysKeepsCursorVisible(t *testing.T) {
	const total, visible = 20, 6

	offset := 0
	for _, cursor := range []int{0, 3, 9, 19, 2, 15, 0, 19} {
		offset = clampOffset(offset, cursor, visible, total)
		if cursor < offset || cursor >= offset+visible {
			t.Fatalf("cursor %d fell outside window [%d,%d)", cursor, offset, offset+visible)
		}
	}
}

// A windowed list has to say that it is windowed, or the entries below the fold
// simply do not exist as far as anybody can tell.
func TestAWindowedListSaysItContinues(t *testing.T) {
	m := &SettingsModel{height: 14, width: 80, screen: screenHomeNetwork}
	row := func(int) string { return "row\n" }

	top := stripANSI(m.fill("", "", 40, row))
	if strings.Contains(top, "more above") {
		t.Error("a list at the top should not claim rows above it")
	}
	if !strings.Contains(top, "more below") {
		t.Error("a truncated list must show that it continues below")
	}

	m.cursor = 20
	middle := stripANSI(m.fill("", "", 40, row))
	if !strings.Contains(middle, "more above") || !strings.Contains(middle, "more below") {
		t.Errorf("a mid-list window needs both markers, got:\n%s", middle)
	}

	m.cursor = 0
	short := stripANSI(m.fill("", "", 3, row))
	if strings.Contains(short, "more above") || strings.Contains(short, "more below") {
		t.Errorf("a list that fits should have no markers, got:\n%s", short)
	}
}

// The block is the same height wherever the cursor is, markers or no markers.
// A menu that changes height as you move through it walks its own footer up and
// down the screen and leaves the taller frame's bottom rows behind, which is
// what going down a list and back up used to do to it.
func TestAListIsTheSameHeightWhereverTheCursorIs(t *testing.T) {
	const width, tall = 80, 14

	m := &SettingsModel{height: tall, width: width, screen: screenHomeNetwork}
	row := func(int) string { return "row\n" }

	want := -1
	for _, cursor := range []int{0, 1, 5, 20, 39, 20, 5, 1, 0} {
		m.cursor = cursor

		got := height(m.fill("", "", 40, row), width)
		if want < 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("cursor %d: block is %d rows, was %d", cursor, got, want)
		}
	}
}

// A terminal too short for the list still has to show some of it.
func TestAShortTerminalStillShowsRows(t *testing.T) {
	m := &SettingsModel{height: 5, width: 80, screen: screenHomeNetwork}

	block := stripANSI(m.fill("", "", 40, func(int) string { return "row\n" }))
	if strings.Count(block, "row") < 1 {
		t.Errorf("a five row terminal showed no entries at all:\n%s", block)
	}
}

func TestListLengthCountsTheHomeNetworkOptOutRow(t *testing.T) {
	m := &SettingsModel{screen: screenHomeNetwork, networks: []string{"a", "b", "c"}}
	if got := m.listLength(); got != 4 {
		t.Errorf("home network list should be networks+1, got %d", got)
	}
}

func TestOnOffAndOrUnset(t *testing.T) {
	if onOff(true) != "on" || onOff(false) != "off" {
		t.Error("onOff should render on/off")
	}
	if orUnset("", "not set") != "not set" {
		t.Error("orUnset should fall back when empty")
	}
	if orUnset("ASUS_5G", "not set") != "ASUS_5G" {
		t.Error("orUnset should pass through a real value")
	}
}

// A screen whose height changes as the cursor moves leaves the taller frame's
// leftovers on screen, which is what a menu breaking while scrolling looks
// like. The Pusher Extreme screen did it worst, being both variable and taller
// than a default terminal, but every screen with a note under the cursor had
// the same fault.
func TestMenuHeightDoesNotChangeAsTheCursorMoves(t *testing.T) {
	m := &SettingsModel{height: defaultHeight, confirmDeleteIndex: -1}
	m.refreshProfiles()

	for _, screen := range []struct {
		name  string
		items int
		view  func(int) string
	}{
		{"deploy", len(deployItems), func(i int) string {
			m.screen, m.cursor = screenDeploy, i
			return m.viewDeploy()
		}},
		{"extreme", len(extremeItems), func(i int) string {
			m.screen, m.cursor = screenExtreme, i
			return m.viewExtreme()
		}},
	} {
		first := lineCount(screen.view(0))
		for i := 1; i < screen.items; i++ {
			if got := lineCount(screen.view(i)); got != first {
				t.Errorf("%s is %d lines at row 0 and %d at row %d", screen.name, first, got, i)
			}
		}

		// And the whole view has to fit the terminal, with a status line and
		// without one: the body claims the room a status is not using, and has
		// to give it back when one appears.
		for _, status := range []string{"", "Saved"} {
			m.status = status
			m.cursor = 0

			if got := renderedRows(m.View(), defaultWidth); got >= defaultHeight {
				t.Errorf("%s with status %q is %d rows of a %d row terminal",
					screen.name, status, got, defaultHeight)
			}
		}
		m.status = ""
	}
}

func TestDevMenuHeightDoesNotChange(t *testing.T) {
	d := &devModel{height: defaultHeight, screen: devScreenMain}

	first := lineCount(d.viewDevMain())
	for i := 1; i < len(d.layout().Rows); i++ {
		d.cursor = i
		if got := lineCount(d.viewDevMain()); got != first {
			t.Errorf("the dev menu is %d lines at row 0 and %d at row %d", first, got, i)
		}
	}
}

// Every entry needs its note, or moving onto the one without it changes the
// screen height and leaves the taller frame's leftovers behind.
func TestEveryDevEntryHasANote(t *testing.T) {
	if len(devHelp) != len(devItems) {
		t.Fatalf("%d entries and %d notes", len(devItems), len(devHelp))
	}
}

// Nothing may be wider than the terminal. A line that wraps costs the view a
// line it did not budget for, and the taller frame's leftovers stay on screen.
func TestNoScreenIsWiderThanTheTerminal(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120} {
		m := &SettingsModel{height: 30, width: width, confirmDeleteIndex: -1}
		m.refreshProfiles()

		d := &devModel{height: 30, width: width, screen: devScreenMain}

		screens := map[string]func() string{
			"settings": func() string { m.screen = screenMain; return m.viewMain() },
			"deploy":   func() string { m.screen = screenDeploy; return m.viewDeploy() },
			"extreme":  func() string { m.screen = screenExtreme; return m.viewExtreme() },
			"dev":      d.viewDevMain,
		}

		for name, view := range screens {
			for cursor := 0; cursor < 4; cursor++ {
				m.cursor, d.cursor = cursor, cursor

				for _, line := range strings.Split(view(), "\n") {
					if got := lipgloss.Width(line); got > width {
						t.Errorf("%s at width %d, row %d: a line is %d wide: %q",
							name, width, cursor, got, stripANSI(line))
					}
				}
			}
		}
	}
}

// A screen has to be the same height whichever entry the cursor is on, at every
// width, or resizing the terminal is what breaks it rather than scrolling.
func TestScreenHeightIsStableAtEveryWidth(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120} {
		m := &SettingsModel{height: 30, width: width, confirmDeleteIndex: -1}
		m.refreshProfiles()

		for _, s := range []struct {
			name  string
			items int
			view  func(int) string
		}{
			{"deploy", len(deployItems), func(i int) string {
				m.screen, m.cursor = screenDeploy, i
				return m.viewDeploy()
			}},
			{"extreme", len(extremeItems), func(i int) string {
				m.screen, m.cursor = screenExtreme, i
				return m.viewExtreme()
			}},
		} {
			first := lineCount(s.view(0))
			for i := 1; i < s.items; i++ {
				if got := lineCount(s.view(i)); got != first {
					t.Errorf("%s at width %d is %d lines at row 0 and %d at row %d",
						s.name, width, first, got, i)
				}
			}
		}
	}
}

// The room a list has was a constant subtracted from the terminal height, which
// scrolled lists that would have fitted while leaving blank rows under them.
// Neither the overflow nor the waste is something a person should work around
// by resizing.
func TestListsFillTheTerminalWithoutOverflowing(t *testing.T) {
	for _, height := range []int{16, 20, 24, 30, 40, 60} {
		m := &SettingsModel{height: height, width: 100, confirmDeleteIndex: -1}
		m.refreshProfiles()
		m.networks = make([]string, 40)
		for i := range m.networks {
			m.networks[i] = fmt.Sprintf("net-%02d", i)
		}

		for _, s := range []struct {
			name  string
			rows  int
			enter func()
		}{
			{"main", len(m.rows()), func() { m.screen = screenMain }},
			{"home network", len(m.networks) + 1, func() { m.screen = screenHomeNetwork }},
		} {
			s.enter()
			m.cursor, m.offset = 0, 0

			view := m.View()
			got := lineCount(view)

			if got > height {
				t.Errorf("%s at height %d renders %d lines", s.name, height, got)
			}

			// A list too long to fit must use the room it has. Not scrolling
			// means every row is on screen, which is the good outcome however
			// few lines that took.
			if strings.Contains(stripANSI(view), "more below") && got < height-1 {
				t.Errorf("%s at height %d scrolls %d rows away while using only %d lines",
					s.name, height, s.rows, got)
			}
		}
	}
}

// The cursor has to stay on screen wherever it is, or moving down past the fold
// makes the selection invisible.
func TestTheCursorStaysOnScreenAtEveryHeight(t *testing.T) {
	for _, height := range []int{16, 24, 40} {
		m := &SettingsModel{height: height, width: 100, screen: screenHomeNetwork}
		m.networks = make([]string, 40)
		for i := range m.networks {
			m.networks[i] = fmt.Sprintf("net-%02d", i)
		}

		for _, cursor := range []int{0, 5, 20, 40, 12, 0} {
			m.cursor = cursor
			plain := stripANSI(m.View())

			if !strings.Contains(plain, fmt.Sprintf("net-%02d", max(0, cursor-1))) && cursor > 0 {
				t.Errorf("height %d: cursor %d is not on screen:\n%s", height, cursor, plain)
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Grouping is display only. An entry appearing in no group, or in two, silently
// removes it from the menu or runs the wrong thing.
func TestEveryMenuEntryIsInExactlyOneGroup(t *testing.T) {
	for _, c := range []struct {
		name     string
		sections []menuSection
		items    []string
	}{
		{"settings", mainSections, mainItems},
		{"dev", devSections, devItems},
	} {
		seen := map[int]int{}
		for _, section := range c.sections {
			for _, item := range section.Items {
				seen[item]++
			}
		}

		for i := range c.items {
			switch seen[i] {
			case 1:
			case 0:
				t.Errorf("%s: %q is in no group, so it cannot be reached", c.name, c.items[i])
			default:
				t.Errorf("%s: %q is in %d groups", c.name, c.items[i], seen[i])
			}
		}
	}
}

func lineCount(s string) int { return strings.Count(s, "\n") }

// renderedRows is how many rows a terminal of this width actually shows, which
// is not how many newlines the string has: a line wider than the terminal is
// wrapped by the terminal itself, onto rows the view never budgeted for.
// Counting newlines misses exactly the bug this is here to catch.
func renderedRows(view string, width int) int {
	rows := 0
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		w := lipgloss.Width(line)
		switch {
		case w == 0:
			rows++
		default:
			rows += (w + width - 1) / width
		}
	}
	return rows
}

// fill() budgets exactly two lines for the status, so the message has to be one
// line however narrow the terminal is. A wider one wraps onto a third, the view
// comes out taller than the screen, and the frame before it is left on screen
// underneath. Reported as toggling a setting breaking the menu and text writing
// over other text.
func TestAStatusNeverMakesTheViewTallerThanTheScreen(t *testing.T) {
	// The longest things a toggle actually says.
	messages := []string{
		"On: every push says what dashboard tuning it overwrote",
		"Off: `pusher dash diff` still compares on demand",
		"On: a random ID, the version and your OS, once a day",
		"This build has no counter to talk to, so nothing is sent",
		"`pusher slim` WILL NOT WORK on this project: it is configured with the " +
			"Kotlin DSL, which slim does not support and is not going to",
	}

	for _, height := range []int{16, 24, 40} {
		for _, width := range []int{40, 60, 72, 80, 100, 120} {
			for _, message := range messages {
				m := &SettingsModel{height: height, width: width, confirmDeleteIndex: -1}
				m.refreshProfiles()
				m.screen = screenMain
				m.status = message

				if got := renderedRows(m.View(), width); got > height {
					t.Errorf("status %q at %dx%d fills %d rows of %d",
						message[:20]+"...", width, height, got, height)
				}

				m.status = ""
				m.err = errors.New(message)

				if got := renderedRows(m.View(), width); got > height {
					t.Errorf("error %q at %dx%d fills %d rows of %d",
						message[:20]+"...", width, height, got, height)
				}
			}
		}
	}
}

// Nothing is gained by cutting a message that already fits.
func TestAShortStatusIsLeftAlone(t *testing.T) {
	m := &SettingsModel{height: 24, width: 100, confirmDeleteIndex: -1}
	m.refreshProfiles()
	m.screen = screenMain
	m.status = "Delta transfer updated"

	if !strings.Contains(stripANSI(m.View()), "Delta transfer updated") {
		t.Error("a status that fits was truncated anyway")
	}
}
