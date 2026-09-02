package tui

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/robotcfg"
)

// View satisfies tea.Model.
func (m *hwModel) View() string {
	if m.quit {
		return ""
	}

	// Nothing is drawn until the terminal has said how big it is.
	//
	// Bubbletea renders one frame before it handles the first resize, so a
	// model that starts with a height renders that height into whatever window
	// it actually has. Twenty four rows into a fourteen row panel scrolls the
	// terminal ten rows, and the renderer has been counting from where it
	// started: from then on it repaints at the wrong height and leaves rows of
	// old frames on screen. That is the duplicated entry, and it happens before
	// a key is ever pressed.
	if m.height <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Hardware configurations"))
	b.WriteString("\n\n")

	switch m.screen {
	case hwScreenList:
		b.WriteString(m.viewHWList())
	case hwScreenActions:
		b.WriteString(m.viewHWActions())
	case hwScreenDevices:
		b.WriteString(m.viewHWDevices())
	case hwScreenDevice:
		b.WriteString(m.viewHWDevice())
	case hwScreenPrompt:
		b.WriteString(m.viewHWPrompt())
	case hwScreenConfirm:
		b.WriteString(m.viewHWConfirm())
	case hwScreenSummary:
		b.WriteString(m.viewHWSummary())
	}

	// The offer outranks the error it came with, and is not a success: a tick
	// next to "no robot connected" reads as pusher being pleased about it.
	b.WriteString(m.statusArea())

	return clamp(b.String(), m.width, usable(m.height))
}

// hwStatusRows is the room the message under the menu always takes: a blank
// line and two rows, whether or not there is anything to put in them.
//
// Always, because a menu that grows a row when it has something to say moves
// every entry on it the moment an action reports back.
const hwStatusRows = 3

// statusArea is the message under the menu, or the space it would have taken.
//
// The offer outranks the error it came with, and is not a success: a tick next
// to "no robot connected" reads as pusher being pleased about it.
func (m *hwModel) statusArea() string {
	room := textWidth(m.width) - 2

	var lines []string
	switch {
	case m.busy != "":
		lines = []string{scrollStyle.Render("  … " + fit(m.busy, room))}

	case m.connect.open:
		if m.err != nil {
			lines = append(lines, errStyle.Render("  ! "+fit(m.err.Error(), room)))
		}
		lines = append(lines, helpStyle.Render("  "+fit(m.connect.hint(), room)))

	case m.err != nil:
		lines = []string{errStyle.Render("  ! " + fit(m.err.Error(), room))}
	case m.status != "":
		lines = []string{okStyle.Render("  ✓ " + fit(m.status, room))}
	}

	out := "\n"
	for i := 0; i < hwStatusRows-1; i++ {
		if i < len(lines) {
			out += lines[i] + "\n"
			continue
		}
		out += "\n"
	}
	return out
}

// budget is how many rows a list may use: the terminal, less the title, less
// the status area, less whatever the screen puts above and below it.
func (m *hwModel) budget(before, after string) int {
	chrome := 2 + hwStatusRows + height(before, m.width) + height(after, m.width)

	if budget := usable(m.height) - chrome; budget >= minVisibleRows {
		return budget
	}
	return minVisibleRows
}

// fill lays a list out into the room left over, in a block that is the same
// height wherever the cursor is.
func (m *hwModel) fill(before, after string, rendered []string) string {
	block, start := pane(rendered, m.width, m.budget(before, after), m.offset, m.cursor)
	m.offset = start

	return before + block + after
}

func (m *hwModel) viewHWDevices() string {
	title := m.sel
	if m.dirty {
		title += " *"
	}

	head := fmt.Sprintf("  %s   %s\n\n", titleStyle.Render(title), m.problemSummary())

	tail := m.problemList() +
		"\n" + helpStyle.Render("  "+fit("enter edit   a add   d remove   s save   p save and push   esc back", textWidth(m.width))) + "\n"

	rendered := make([]string, len(m.rows))
	for i, row := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorOn.Render("> ")
		}

		mark := " "
		if row.HasIss {
			mark = okStyle.Render("!")
			if row.Issue == robotcfg.Error {
				mark = errStyle.Render("x")
			}
		}

		switch row.Kind {
		case hwRowPortal:
			rendered[i] = fmt.Sprintf("%s%s %s\n", cursor, mark, titleStyle.Render(row.Label))
		case hwRowModule:
			rendered[i] = fmt.Sprintf("%s%s   %s  %s\n", cursor, mark,
				valueStyle.Render(row.Label), helpStyle.Render(row.Detail))
		case hwRowDevice:
			rendered[i] = fmt.Sprintf("%s%s     %-34s %s\n", cursor, mark,
				row.Label, helpStyle.Render(row.Detail))
		default:
			rendered[i] = fmt.Sprintf("%s%s     %s\n", cursor, mark, unsetStyle.Render(row.Label))
		}
	}

	return m.fill(head, tail, rendered)
}

func (m *hwModel) problemSummary() string {
	errors := m.issues.Count(robotcfg.Error)
	warnings := m.issues.Count(robotcfg.Warning)

	switch {
	case errors > 0:
		return errStyle.Render(fmt.Sprintf("%d problem(s) the robot would reject", errors))
	case warnings > 0:
		return okStyle.Render(fmt.Sprintf("%d thing(s) worth a look", warnings))
	default:
		return valueStyle.Render("no problems")
	}
}

func (m *hwModel) problemList() string {
	if len(m.issues) == 0 {
		return ""
	}

	const most = 3

	var b strings.Builder
	b.WriteString("\n")

	for i, issue := range m.issues {
		if i == most {
			fmt.Fprintf(&b, "  %s\n", helpStyle.Render(
				fmt.Sprintf("... and %d more", len(m.issues)-most)))
			break
		}

		style := okStyle
		if issue.Level == robotcfg.Error {
			style = errStyle
		}
		fmt.Fprintf(&b, "  %s\n", style.Render("  "+issue.Msg))
	}

	return b.String()
}

func (m *hwModel) viewHWDevice() string {
	var b strings.Builder

	what := "Edit device"
	if m.form.adding {
		what = "Add a device"
	}

	where := ""
	if m.form.portal < len(m.cfg.Portals) && m.form.module >= 0 &&
		m.form.module < len(m.cfg.Portals[m.form.portal].Modules) {
		where = " on " + m.cfg.Portals[m.form.portal].Modules[m.form.module].Name
	}

	fmt.Fprintf(&b, "  %s%s\n\n", titleStyle.Render(what), helpStyle.Render(where))

	b.WriteString(m.field("Type", m.form.typed, hwFieldType))

	if m.form.field == hwFieldType {
		b.WriteString(m.suggestions())
	} else if flavor := robotcfg.FlavorOf(m.form.typed); flavor != robotcfg.Unclassified {
		fmt.Fprintf(&b, "         %s\n", helpStyle.Render("uses a "+flavor.String()+" port"))
	} else if m.form.typed != "" {
		fmt.Fprintf(&b, "         %s\n", helpStyle.Render("not a type pusher knows - its ports are not checked"))
	}

	b.WriteString(m.field("Name", m.form.name, hwFieldName))
	b.WriteString(m.field("Port", m.form.port, hwFieldPort))

	if robotcfg.FlavorOf(m.form.typed) == robotcfg.I2C {
		b.WriteString(m.field("Bus", m.form.bus, hwFieldBus))
	}

	b.WriteString("\n")
	if m.form.problem != "" {
		b.WriteString("  " + errStyle.Render("! "+m.form.problem) + "\n")
	} else {
		b.WriteString("  " + okStyle.Render("✓ looks fine") + "\n")
	}

	help := "  tab next field   enter save   esc cancel"
	if m.form.field == hwFieldType {
		help = "  type to filter   up/down pick   enter choose   tab next field   esc cancel"
	}
	b.WriteString("\n" + helpStyle.Render(help) + "\n")

	return b.String()
}

func (m *hwModel) field(label, value string, which hwField) string {
	shown := value
	if m.form.field == which {
		shown += "▏"
	} else if value == "" {
		shown = unsetStyle.Render("-")
	}

	marker := "  "
	if m.form.field == which {
		marker = cursorOn.Render("> ")
	}

	return fmt.Sprintf("%s%-7s %s\n", marker, label, shown)
}

func (m *hwModel) suggestions() string {
	if len(m.form.suggest) == 0 {
		return "         " + unsetStyle.Render("no device type matches") + "\n"
	}

	const window = 5

	start := m.form.pick - window/2
	if start < 0 {
		start = 0
	}
	if start+window > len(m.form.suggest) {
		start = len(m.form.suggest) - window
	}
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	for i := start; i < len(m.form.suggest) && i < start+window; i++ {
		if i == m.form.pick {
			fmt.Fprintf(&b, "         %s\n", cursorOn.Render("> "+m.form.suggest[i]))
			continue
		}
		fmt.Fprintf(&b, "           %s\n", helpStyle.Render(m.form.suggest[i]))
	}

	if len(m.form.suggest) > window {
		fmt.Fprintf(&b, "           %s\n", scrollStyle.Render(
			fmt.Sprintf("%d of %d", m.form.pick+1, len(m.form.suggest))))
	}

	return b.String()
}

func (m *hwModel) viewHWPrompt() string {
	var b strings.Builder

	fmt.Fprintf(&b, "  %s\n\n", titleStyle.Render(m.prompt.title))
	fmt.Fprintf(&b, "  %s▏\n", m.prompt.value)

	b.WriteString("\n" + helpStyle.Render("  enter confirm   esc cancel") + "\n")
	return b.String()
}

func (m *hwModel) viewHWConfirm() string {
	var b strings.Builder

	fmt.Fprintf(&b, "  %s\n", titleStyle.Render(m.confirm.title))
	if m.confirm.detail != "" {
		fmt.Fprintf(&b, "  %s\n", helpStyle.Render(m.confirm.detail))
	}

	b.WriteString("\n" + helpStyle.Render("  y to confirm   anything else cancels") + "\n")
	return b.String()
}

func (m *hwModel) viewHWSummary() string {
	var b strings.Builder

	fmt.Fprintf(&b, "  %s\n\n", titleStyle.Render(m.sel))

	if m.cfg == nil {
		return b.String()
	}

	for _, line := range strings.Split(robotcfg.Summary(m.cfg), "\n") {
		if line != "" {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}

	names := m.cfg.Names()
	fmt.Fprintf(&b, "\n  %s\n", helpStyle.Render(
		fmt.Sprintf("%d device(s) an OpMode can look up", len(names))))

	b.WriteString(m.problemList())
	b.WriteString("\n" + helpStyle.Render("  esc back") + "\n")

	return b.String()
}
