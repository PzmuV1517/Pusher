package tui

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/hotreload"
	tea "github.com/charmbracelet/bubbletea"
)

// Pusher Extreme changes the shape of a team's project, so the menu has to be
// honest about what it did and how to get out. Everything reversible is said to
// be reversible, in the place where somebody would look for it.

var extremeItems = []string{
	"Set up this project",
	"Undo the setup",
	"Use it when deploying",
	"Back",
}

// Every entry is the same number of lines. A view that changes height between
// frames leaves the taller one's leftovers on screen, which is what "the menu
// is broken when scrolling" looks like.
var extremeHelp = []string{
	"Adds one marked block to TeamCode/build.gradle so team code is\n" +
		"reloaded rather than packaged. Nothing else is touched.",

	"Puts team code back in the APK and removes the block.\n" +
		"Deploy once afterwards.",

	"Reloads instead of installing when that is equivalent,\n" +
		"and installs normally when it is not.",

	"",
}

// extremeWarning is the one thing somebody has to know before turning this on.
const extremeWarning = "While set up, team code is not in the APK: a teammate " +
	"deploying from Android Studio gets a robot with no OpModes."

type extremeState struct {
	root      string
	set       bool
	status    extreme.State
	haveRoot  bool
	kept      []string
	drivers   []string
	reflected extreme.Reflection
}

func (m *SettingsModel) refreshExtreme() {
	m.extreme = extremeState{}

	project, err := extreme.FindProject()
	if err != nil {
		return
	}

	m.extreme.haveRoot = true
	m.extreme.root = project.Root
	m.extreme.set = extreme.Excluded(project.Root)
	m.extreme.kept = extreme.Kept(project.Root)
	m.extreme.drivers = extreme.FindDrivers(project.Root)
	m.extreme.reflected = extreme.FindReflected(project.Root)

	serial := ""
	if s, err := adb.Target(); err == nil {
		serial = s
	}

	apk, _ := gradle.FindApk(project.Root)
	m.extreme.status = extreme.Status(project.Root, serial, apk)
}

// extremeLabel says what will actually happen on the next deploy, which is not
// what the setting alone says.
//
// Off while the project is still set up is the state worth naming: the gradle
// block is what the build obeys, so the APK still comes out with no team code
// in it. Reading that as a plain "off" is how somebody concludes their project
// is back to normal when it is not.
func (m *SettingsModel) extremeLabel() string {
	if !config.GetExtreme() {
		if m.extreme.set {
			return "off (project still set up)"
		}
		return "off"
	}
	if m.extreme.set {
		return "on"
	}
	return "on (project not set up)"
}

func (m *SettingsModel) updateExtreme(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q":
		m.quit = true
		return m, tea.Quit

	case "esc", "left", "h":
		m.goTo(screenMain, 9)
		return m, nil

	case "up", "k":
		m.moveCursor(-1, len(extremeItems))
	case "down", "j":
		m.moveCursor(1, len(extremeItems))

	case "enter", " ", "right", "l":
		m.err = nil
		m.status = ""

		switch m.cursor {
		case 0:
			m.setUpExtreme()
		case 1:
			m.undoExtreme()
		case 2:
			m.setStatus(config.SetExtreme(!config.GetExtreme()), "Pusher Extreme updated")
		case 3:
			m.goTo(screenMain, 9)
		}
	}

	return m, nil
}

func (m *SettingsModel) setUpExtreme() {
	if !m.extreme.haveRoot {
		m.err = fmt.Errorf("run pusher from your FTC project to set this up")
		return
	}

	// Checked before anything is written, so a project this cannot reload is
	// told so rather than left with team code out of its APK.
	if err := extreme.Supported(m.extreme.root); err != nil {
		m.err = err
		return
	}

	// Hardware device drivers are kept whatever anyone would prefer. Every
	// reload builds a new classloader, so a reloaded driver is a different
	// class each time while the device in the hardware map was built under an
	// earlier one, and the robot then cannot find its own hardware.
	drivers := extreme.FindDrivers(m.extreme.root)

	if err := extreme.Exclude(m.extreme.root, drivers...); err != nil {
		m.err = err
		return
	}
	if err := config.SetExtreme(true); err != nil {
		m.err = err
		return
	}

	m.refreshExtreme()

	m.status = "Set up. Deploy once to install the APK without team code."

	// Exclude keeps whatever the drivers need to compile as well, and every one
	// of those stops being reloadable. Reporting only the drivers would
	// understate what was taken out of the reload.
	if kept := extreme.Kept(m.extreme.root); len(kept) > 0 {
		m.status = fmt.Sprintf("Set up. %d hardware driver(s) stay in the APK. Deploy once.",
			len(drivers))
		if extra := len(kept) - len(drivers); extra > 0 {
			m.status = fmt.Sprintf("Set up. %d driver(s) plus %d they need stay in the APK. Deploy once.",
				len(drivers), extra)
		}
	}
}

func (m *SettingsModel) undoExtreme() {
	if !m.extreme.haveRoot {
		m.err = fmt.Errorf("run pusher from your FTC project to undo this")
		return
	}

	if err := extreme.Include(m.extreme.root); err != nil {
		m.err = err
		return
	}
	if err := config.SetExtreme(false); err != nil {
		m.err = err
		return
	}

	// A hub left holding a reload has those classes named twice once team code
	// is packaged again, and the SDK then registers no OpMode at all.
	m.status = "Undone. Deploy once so the robot gets an APK with team code in it."
	if serial, err := adb.Target(); err == nil {
		if err := hotreload.Clean(serial); err != nil {
			m.status = "Undone, but the robot still holds a reload: connect it and undo again."
		}
		extreme.ForgetSignature(serial)
	} else {
		m.status = "Undone. Connect the robot and undo again to clear the reload off it, then deploy."
	}

	m.refreshExtreme()
}

func (m *SettingsModel) viewExtreme() string {
	var b strings.Builder

	b.WriteString(helpStyle.Render("  "+fit(
		"Pusher Extreme   reload OpModes instead of installing an APK",
		textWidth(m.width))) + "\n\n")

	b.WriteString(m.extremeStatusLines())

	values := []string{
		onOff(m.extreme.set),
		"",
		onOff(config.GetExtreme()),
		"",
	}

	b.WriteString(m.renderList(len(extremeItems), func(i int) string {
		return renderRow(i == m.cursor, extremeItems[i], values[i], 24, m.width)
	}))

	b.WriteString(note(extremeHelp, m.cursor, m.width))

	b.WriteString("\n")
	for _, line := range wrap(extremeWarning, textWidth(m.width)) {
		b.WriteString("  " + errStyle.Render(line) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter choose · esc back", textWidth(m.width))) + "\n")
	return b.String()
}

// extremeStatusLines says what would happen on the next deploy, which is the
// question somebody actually has.
func (m *SettingsModel) extremeStatusLines() string {
	var b strings.Builder

	// Two lines, always. The list below must not move when this changes.
	if !m.extreme.haveRoot {
		b.WriteString("  " + unsetStyle.Render(fit("No FTC project here, so there is nothing to set up.", textWidth(m.width))) + "\n\n")
		return b.String()
	}

	switch {
	case !m.extreme.set:
		b.WriteString("  " + unsetStyle.Render(fit("Not set up: team code is packaged in the APK as usual.", textWidth(m.width))) + "\n")
	case m.extreme.status.Usable():
		b.WriteString("  " + okStyle.Render(fit("Ready: the next deploy reloads instead of installing.", textWidth(m.width))) + "\n")
	default:
		fmt.Fprintf(&b, "  %s\n", scrollStyle.Render(fit("Next deploy installs: "+m.extreme.status.Reason, textWidth(m.width))))
	}

	extras := ""
	if n := len(m.extreme.reflected.Classes); n > 0 {
		extras = fmt.Sprintf("%d @Config bridged", n)
	}
	if n := len(m.extreme.kept); n > 0 {
		if extras != "" {
			extras += ", "
		}
		extras += fmt.Sprintf("%d kept in the APK (hardware drivers)", n)
	}
	if n := len(m.extreme.drivers); n > 0 && !m.extreme.set {
		extras = fmt.Sprintf("%d hardware driver(s) will stay in the APK", n)
	}
	fmt.Fprintf(&b, "  %s\n\n", helpStyle.Render(fit(extras, textWidth(m.width))))

	return b.String()
}
