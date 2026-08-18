package tui

import "github.com/charmbracelet/lipgloss"

// A dozen settings in one flat column is a wall to read, and nothing in it says
// which ones belong together or which one to reach for. Grouping is display
// only: entries keep their positions in the item list, so what each one does is
// still decided by its own index and adding a group cannot rewire the menu.

var sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("246"))

// menuSection is a named group of entries, named by their index in the item
// list. An empty title puts a gap in without a heading.
type menuSection struct {
	Title string
	Items []int
}

// menuLayout is the order to show entries in, and what heads each group.
type menuLayout struct {
	// Rows are indexes into the item list, in display order.
	Rows []int
	// Headers[i] heads Rows[i]. Empty for a group that only wants the gap.
	Headers []string
	// Starts[i] marks the first row of a group, which is what earns the gap.
	// A group with no title still gets one, or the last entry crowds the one
	// above it.
	Starts []bool
}

// arrange lays sections out, dropping hidden entries and any heading whose
// group has nothing left under it.
//
// shown may be nil, which shows everything.
func arrange(sections []menuSection, shown func(int) bool) menuLayout {
	var out menuLayout

	for _, section := range sections {
		first := true

		for _, item := range section.Items {
			if shown != nil && !shown(item) {
				continue
			}

			starts, header := first, ""
			if first {
				header = section.Title
				first = false
			}

			out.Rows = append(out.Rows, item)
			out.Headers = append(out.Headers, header)
			out.Starts = append(out.Starts, starts)
		}
	}

	return out
}

// Extra is how many lines the groups add, which the list has to leave room for
// or the bottom of the menu falls off a short terminal.
func (l menuLayout) Extra() int {
	extra := 0
	for i, starts := range l.Starts {
		if !starts {
			continue
		}
		if l.Headers[i] != "" {
			extra++
		}
		if i > 0 {
			extra++
		}
	}
	return extra
}

// render draws one row, under its heading when it starts a group.
func (l menuLayout) render(i int, body string, width int) string {
	if !l.Starts[i] {
		return body
	}

	out := ""
	if i > 0 {
		out = "\n"
	}
	if l.Headers[i] != "" {
		out += sectionStyle.Render("  "+fit(l.Headers[i], textWidth(width))) + "\n"
	}
	return out + body
}
