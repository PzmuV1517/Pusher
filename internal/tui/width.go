package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Every note in these menus was written with its line breaks in the source, at
// about seventy columns. On a narrower terminal they wrap, and because the
// views are built to be a fixed number of lines, a wrapped line adds one and
// the taller frame's leftovers stay on screen. That is what "the menu is broken"
// looks like.
//
// So the breaks in the source are treated as spaces and the text is wrapped to
// whatever room there is. The block still has to be the same height whichever
// entry the cursor is on, so its height is the tallest entry at this width
// rather than a number written down next to it.

// defaultWidth is used until the terminal says otherwise.
const defaultWidth = 80

// minWidth is the narrowest layout worth attempting.
const minWidth = 32

// indent is the left margin every line carries.
const indent = 2

// textWidth is the room a note has, given the terminal.
func textWidth(width int) int {
	// Only an unset width falls back. A terminal narrower than minWidth is
	// still a real terminal, and pretending it is eighty columns is what makes
	// every line on it wrap.
	if width <= 0 {
		width = defaultWidth
	}

	room := width - indent
	if room < 1 {
		room = 1
	}
	return room
}

// wrap breaks text to fit, treating the breaks already in it as spaces.
func wrap(text string, width int) []string {
	if text == "" {
		return nil
	}

	var out []string

	for _, para := range strings.Split(text, "\n\n") {
		line := ""

		for _, word := range strings.Fields(para) {
			switch {
			case line == "":
				line = word
			case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}

		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

// noteHeight is how many lines the tallest note needs at this width, which is
// the height the block has to be for every one of them.
func noteHeight(notes []string, width int) int {
	room := textWidth(width)

	tallest := 1
	for _, note := range notes {
		if n := len(wrap(note, room)); n > tallest {
			tallest = n
		}
	}
	return tallest
}

// note renders one entry's text, padded to the height the block always is.
func note(notes []string, index, width int) string {
	room := textWidth(width)
	height := noteHeight(notes, width)

	var lines []string
	if index >= 0 && index < len(notes) {
		lines = wrap(notes[index], room)
	}

	var b strings.Builder
	b.WriteString("\n")

	for i := 0; i < height; i++ {
		if i < len(lines) {
			b.WriteString(strings.Repeat(" ", indent) + helpStyle.Render(lines[i]) + "\n")
			continue
		}
		b.WriteString("\n")
	}

	return b.String()
}

// fit truncates one line so it cannot wrap, which would cost the view a line it
// did not budget for.
func fit(line string, width int) string {
	// A small budget is a small budget. This used to treat anything under
	// minWidth as unset and truncate to eighty instead, so on a narrow terminal
	// nothing was ever cut and every row wrapped.
	if width < 1 {
		width = 1
	}
	if lipgloss.Width(line) <= width {
		return line
	}

	// Trimmed by runes, since a byte count is not a column count.
	runes := []rune(line)
	for len(runes) > 1 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// labelWidth is the column the values line up in, narrowed when the terminal
// cannot spare it.
//
// A row is the three column cursor, the label out to this column, at least one
// space, and then the value. All of that has to fit.
func labelWidth(width, preferred, value int) int {
	if width <= 0 {
		width = defaultWidth
	}

	if room := width - rowPrefix - 1 - value; room < preferred {
		preferred = room
	}
	if preferred < 1 {
		preferred = 1
	}
	return preferred
}

// rowPrefix is the width of the cursor column every row carries.
const rowPrefix = 3

// lines counts the rendered height of a fragment.
func lines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n")
}

// height is how many rows a block takes on a terminal this wide.
//
// Not the same as counting newlines. A line wider than the terminal is wrapped
// by the terminal itself, onto rows nothing here budgeted for, and the view
// then runs past the bottom of the screen leaving the frame under it on
// display. Budgeting by newlines is what made scrolling a narrow terminal show
// duplicates.
func height(block string, width int) int {
	if block == "" {
		return 0
	}
	if width < 1 {
		width = defaultWidth
	}

	rows := 0
	for _, line := range strings.Split(strings.TrimSuffix(block, "\n"), "\n") {
		w := lipgloss.Width(line)
		if w <= width {
			rows++
			continue
		}
		rows += (w + width - 1) / width
	}
	return rows
}

// clamp drops whatever will not fit on the screen.
//
// A last resort rather than a layout. Screens that lay themselves out to the
// terminal never reach it, but some are a fixed block of text, and on a very
// short terminal that block is simply taller than the screen. Losing the bottom
// of one screen is better than the terminal showing the bottom of the last one
// underneath it, which is what a view taller than the window leaves behind.
func clamp(view string, width, tall int) string {
	if tall < 1 || height(view, width) <= tall {
		return view
	}

	kept, used := make([]string, 0, tall), 0
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		rows := height(line+"\n", width)
		if used+rows > tall {
			break
		}
		kept = append(kept, line)
		used += rows
	}

	return strings.Join(kept, "\n") + "\n"
}
