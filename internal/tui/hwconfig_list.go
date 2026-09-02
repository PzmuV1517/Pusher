package tui

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/robotcfg"
	tea "github.com/charmbracelet/bubbletea"
)

var hwListExtras = []string{
	"New configuration",
	"Pull everything from the robot",
	"Refresh",
	"Exit",
}

func (m *hwModel) updateHWList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.entries) + len(hwListExtras)

	switch key.String() {
	case "q", "esc":
		m.quit = true
		return m, tea.Quit

	case "up", "k":
		m.move(-1, total)
	case "down", "j":
		m.move(1, total)

	case "enter", " ", "right", "l":
		m.clear()

		if m.cursor < len(m.entries) {
			m.sel = m.entries[m.cursor].Name
			m.goTo(hwScreenActions, 0)
			return m, nil
		}

		switch hwListExtras[m.cursor-len(m.entries)] {
		case "New configuration":
			m.prompt = hwPrompt{title: "Name for the new configuration", action: "new"}
			m.goTo(hwScreenPrompt, 0)
		case "Pull everything from the robot":
			return m, m.pullAll()
		case "Refresh":
			m.loading = true
			return m, hwLoad
		case "Exit":
			m.quit = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *hwModel) entry(name string) hwEntry {
	for _, e := range m.entries {
		if e.Name == name {
			return e
		}
	}
	return hwEntry{Name: name}
}

func (m *hwModel) actionItems() []string {
	e := m.entry(m.sel)

	var items []string
	if e.InLocal {
		items = append(items, "Edit devices", "View summary")
	}
	if e.InLocal && m.serial != "" {
		items = append(items, "Push to the robot")
	}
	if e.OnRobot {
		items = append(items, "Pull from the robot")
	}
	if e.InLocal && e.OnRobot {
		items = append(items, "Compare with the robot")
	}
	if e.InLocal {
		items = append(items, "Duplicate", "Rename", "Delete from the project")
	}
	if e.OnRobot {
		items = append(items, "Delete from the robot")
	}

	return append(items, "Back")
}

func (m *hwModel) updateHWActions(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.actionItems()

	switch key.String() {
	case "q":
		m.quit = true
		return m, tea.Quit

	case "esc", "left", "h":
		m.goTo(hwScreenList, m.cursor)
		return m, nil

	case "up", "k":
		m.move(-1, len(items))
	case "down", "j":
		m.move(1, len(items))

	case "enter", " ", "right", "l":
		m.clear()

		switch items[m.cursor] {
		case "Edit devices":
			return m, m.openEditor()

		case "View summary":
			if err := m.loadConfig(); err != nil {
				m.err = err
				return m, nil
			}
			m.goTo(hwScreenSummary, 0)

		case "Push to the robot":
			return m, m.push(m.sel)

		case "Pull from the robot":
			return m, m.pull(m.sel)

		case "Compare with the robot":
			return m, m.compare(m.sel)

		case "Duplicate":
			m.prompt = hwPrompt{title: "Name for the copy", value: m.sel + " copy", action: "duplicate"}
			m.goTo(hwScreenPrompt, 0)

		case "Rename":
			m.prompt = hwPrompt{title: "New name", value: m.sel, action: "rename"}
			m.goTo(hwScreenPrompt, 0)

		case "Delete from the project":
			m.confirm = hwConfirm{
				title:  fmt.Sprintf("Delete %q from the project?", m.sel),
				detail: m.store.Path(m.sel),
				action: "delete-local",
				name:   m.sel,
			}
			m.goTo(hwScreenConfirm, 0)

		case "Delete from the robot":
			detail := robotcfg.RemotePath(m.sel)
			if m.entry(m.sel).Active {
				detail += "\n  This is the configuration the robot is running."
			}
			m.confirm = hwConfirm{
				title:  fmt.Sprintf("Delete %q from the robot?", m.sel),
				detail: detail,
				action: "delete-robot",
				name:   m.sel,
			}
			m.goTo(hwScreenConfirm, 0)

		case "Back":
			m.goTo(hwScreenList, 0)
		}
	}

	return m, nil
}

func (m *hwModel) viewHWList() string {
	var head strings.Builder

	fmt.Fprintf(&head, "  %s\n", helpStyle.Render(fit(m.store.Dir, textWidth(m.width))))
	if m.loading {
		fmt.Fprintf(&head, "  %s\n", helpStyle.Render("asking the robot..."))
	} else if m.serial != "" {
		robot := "robot: " + m.serial
		if m.active != "" {
			robot += "   active: " + m.active
		}
		fmt.Fprintf(&head, "  %s\n", helpStyle.Render(fit(robot, textWidth(m.width))))
	} else {
		head.WriteString("\n")
	}
	head.WriteString("\n")

	if len(m.entries) == 0 {
		head.WriteString("  " + unsetStyle.Render("No configurations yet.") + "\n\n")
	} else {
		fmt.Fprintf(&head, "  %-28s %-18s %s\n",
			helpStyle.Render("CONFIGURATION"), helpStyle.Render("WHERE"), helpStyle.Render("STATUS"))
	}

	var tail strings.Builder
	if m.active != "" {
		tail.WriteString("\n  " + helpStyle.Render("* is the configuration the robot is running") + "\n")
	}
	tail.WriteString("\n" + helpStyle.Render("  "+fit("enter open   up/down move   q quit", textWidth(m.width))) + "\n")

	total := len(m.entries) + len(hwListExtras)
	rendered := make([]string, total)

	for i := range rendered {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorOn.Render("> ")
		}

		if i >= len(m.entries) {
			rendered[i] = fmt.Sprintf("%s%s\n", cursor, hwListExtras[i-len(m.entries)])
			continue
		}

		e := m.entries[i]

		name := e.Name
		if e.Active {
			name += " *"
		}

		status := e.status()
		styled := valueStyle.Render(status)
		if status == "differs" {
			styled = scrollStyle.Render(status)
		} else if !e.OnRobot || !e.InLocal {
			styled = unsetStyle.Render(status)
		}

		rendered[i] = fmt.Sprintf("%s%-28s %-18s %s\n", cursor, name, e.where(), styled)
	}

	return m.fill(head.String(), tail.String(), rendered)
}

func (m *hwModel) viewHWActions() string {
	var b strings.Builder

	e := m.entry(m.sel)

	fmt.Fprintf(&b, "  %s\n", titleStyle.Render(m.sel))
	fmt.Fprintf(&b, "  %s\n\n", helpStyle.Render(e.where()+"   "+e.status()))

	for i, item := range m.actionItems() {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorOn.Render("> ")
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, item)
	}

	b.WriteString("\n" + helpStyle.Render("  enter choose   esc back") + "\n")
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
