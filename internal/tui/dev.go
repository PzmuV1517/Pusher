package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/bench"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/andreibanu/pusher/internal/feature"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/hotreload"
	"github.com/andreibanu/pusher/internal/pathtrace"
	"github.com/andreibanu/pusher/internal/visual"
	tea "github.com/charmbracelet/bubbletea"
)

type devScreen int

const (
	devScreenMain devScreen = iota
	devScreenReport
	devScreenReload
)

// benchRepeats is how many times each configuration is measured. One sample
// cannot tell a real difference from run-to-run variance, and a deploy varies
// by seconds.
const benchRepeats = 3

// devBanner is the warning above the tools.
const devBanner = "These measure by deploying to the robot over and over. " +
	"If you do not already know why you want this, you do not want it."

var devItems = []string{
	"Benchmark the deploy",
	"Hot reload feasibility",
	"Both, with a full report",
	"Hot reload an OpMode",
	"Benchmark Pusher Extreme",
	"Collect the robot's own logs",
	"Remove the hot reload proof",
	"Exit",
	"Preview the path visualiser",
}

// devVisualise is the position in devItems of the entry that only appears once
// the visualiser has been turned on.
const devVisualise = 8

// devSections group the tools by what they are for. The numbers are positions
// in devItems, so regrouping cannot change what an entry does.
var devSections = []menuSection{
	{"Measuring", []int{0, 4, 2, 1}},
	{"Trying things out", []int{3, devVisualise, 6}},
	{"When something went wrong", []int{5}},
	{"", []int{7}},
}

func (m *devModel) layout() menuLayout {
	return arrange(devSections, func(i int) bool {
		return i != devVisualise || feature.Revealed()
	})
}

// previewVisualiser draws a made up run and opens it.
//
// Nothing is read off the robot, so this works with no hub attached and no
// recorded trace, which is the point of it.
func (m *devModel) previewVisualiser() tea.Cmd {
	out, err := visual.RenderDemo("", pathtrace.DefaultLimits())
	if err != nil {
		m.err = err
		return nil
	}

	visual.Open(out)
	m.status = "Opened a sample path: " + out
	return nil
}

var devHelp = []string{
	"Deploys the current build with different settings and times each one,\n" +
		"three times over so a difference can be told from noise.\n" +
		"Reinstalls the app repeatedly. Takes about fifteen minutes.",

	"Times pushing a team-code-sized dex to the hub and compiling it there,\n" +
		"to see what a hot reload would have to beat. Installs nothing.",

	"Runs both and writes a report covering what each setting does, plus\n" +
		"Sloth's published figures for context.",

	"Compiles an OpMode here, pushes it to the hub and tells the robot\n" +
		"controller to rescan. Binds a motor by name and shows its encoder,\n" +
		"alternating m1 and m2 so a reload is proved by the binding changing.\n" +
		"Replaces anything Pusher Extreme reloaded: deploy again afterwards.",

	"Compiles and reloads your own team code several times over and times\n" +
		"each stage. Needs Pusher Extreme set up. Writes a report you can\n" +
		"paste numbers out of.",

	"Pulls the robot controller's own log files, which survive the app dying\n" +
		"and the hub rebooting, and shows the most recent crash in them. Use\n" +
		"this when adb logcat comes back empty.",

	"Deletes the pushed dex and tells the robot controller to rescan.",

	"",

	"Draws a made up autonomous and opens it, so the visualiser can be\n" +
		"looked at without a robot, a recorded run, or a blob-dev build.\n" +
		"Nothing is read off the robot and nothing is deployed.",
}

type devModel struct {
	screen devScreen
	cursor int
	height int
	width  int

	project string
	apk     string
	splits  []string
	serial  string

	busy    string
	reload  *hotreload.Result
	started time.Time
	elapsed time.Duration
	report  string
	saved   string
	summary string

	status string
	err    error
	quit   bool
}

// reloadDoneMsg carries the hot reload attempt back to the menu.
type reloadDoneMsg struct {
	result *hotreload.Result
	err    error
}

type devDoneMsg struct {
	report  string
	summary string
	saved   string
	err     error
}

type devProgressMsg struct{ what string }

// devTickMsg keeps the elapsed counter moving, so a benchmark that takes a
// quarter of an hour does not look like a freeze.
type devTickMsg time.Time

// devProgress carries what the worker is doing back to the menu. Buffered and
// dropped on the floor when full: progress must never block the measurement.
var devProgress = make(chan string, 64)

func waitForProgress() tea.Msg { return devProgressMsg{what: <-devProgress} }

func devTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return devTickMsg(t) })
}

// RunDev opens the developer menu.
func RunDev(projectRoot, apk string, splits []string) error {
	m := &devModel{
		height:  defaultHeight,
		width:   defaultWidth,
		project: projectRoot,
		apk:     apk,
		splits:  splits,
	}

	if serial, err := adb.Target(); err == nil {
		m.serial = serial
	}

	_, err := tea.NewProgram(m).Run()
	return err
}

// Init satisfies tea.Model.
func (m *devModel) Init() tea.Cmd { return nil }

// Update satisfies tea.Model.
func (m *devModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case devProgressMsg:
		if m.busy == "" {
			return m, nil
		}
		m.busy = msg.what
		return m, waitForProgress

	case devTickMsg:
		if m.busy == "" {
			return m, nil
		}
		m.elapsed = time.Since(m.started)
		return m, devTick()

	case reloadDoneMsg:
		m.busy = ""
		m.err = msg.err
		m.reload = msg.result
		if msg.err == nil && msg.result != nil && msg.result.Err == nil {
			m.screen = devScreenReload
		}
		return m, nil

	case devDoneMsg:
		m.busy = ""
		m.err = msg.err
		m.report = msg.report
		m.summary = msg.summary
		m.saved = msg.saved
		if msg.err == nil {
			m.screen = devScreenReport
		}
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if key.Type == tea.KeyCtrlC {
		m.quit = true
		return m, tea.Quit
	}

	if m.busy != "" {
		return m, nil
	}

	if m.screen == devScreenReport || m.screen == devScreenReload {
		switch key.String() {
		case "esc", "q", "left", "h", "enter":
			m.screen = devScreenMain
		}
		return m, nil
	}

	switch key.String() {
	case "q", "esc":
		m.quit = true
		return m, tea.Quit

	case "up", "k":
		rows := m.layout().Rows
		m.cursor = (m.cursor - 1 + len(rows)) % len(rows)
	case "down", "j":
		rows := m.layout().Rows
		m.cursor = (m.cursor + 1) % len(rows)

	case "enter", " ", "right", "l":
		m.err = nil
		m.status = ""

		switch m.layout().Rows[m.cursor] {
		case 0:
			return m, m.run(true, false)
		case 1:
			return m, m.run(false, true)
		case 2:
			return m, m.run(true, true)
		case 3:
			return m, m.tryReload()
		case 4:
			return m, m.benchExtreme()
		case 5:
			return m, m.collectLogs()
		case 6:
			return m, m.cleanReload()
		case 7:
			m.quit = true
			return m, tea.Quit
		case devVisualise:
			return m, m.previewVisualiser()
		}
	}

	return m, nil
}

func (m *devModel) run(deploy, reload bool) tea.Cmd {
	if m.serial == "" {
		m.err = fmt.Errorf("no robot connected - plug in USB or run `pusher connect`")
		return nil
	}
	if m.apk == "" {
		m.err = fmt.Errorf("no APK built yet - run `pusher` once first")
		return nil
	}

	m.busy = "starting"
	m.started = time.Now()
	m.elapsed = 0

	serial, apk, splits, project := m.serial, m.apk, m.splits, m.project

	work := func() tea.Msg {
		info, err := bench.Inspect(apk)
		if err != nil {
			return devDoneMsg{err: err}
		}

		var runs []bench.Run
		if deploy {
			runs = bench.Deploy(bench.Options{
				Serial:   serial,
				APK:      apk,
				Splits:   splits,
				Repeat:   benchRepeats,
				Progress: post,
			})
		}

		var floor bench.Reload
		if reload {
			post("timing a reload on the hub")
			floor = bench.MeasureReload(serial, apk)
		}

		settings := map[string]bool{
			"delta":     config.GetDeltaTransfer(),
			"skip":      config.GetSkipUnchanged(),
			"stream":    config.GetStreamInstall(),
			"storeLibs": config.GetStoreLibs(),
			"split":     config.GetSplitInstall(),
		}

		body := bench.Report(info, runs, floor, settings)

		saved, err := bench.SaveReport(project, body)
		if err != nil {
			saved = ""
		}

		return devDoneMsg{
			report:  body,
			summary: bench.Summary(runs),
			saved:   saved,
		}
	}

	return tea.Batch(work, waitForProgress, devTick())
}

// post reports progress without ever blocking the measurement.
func post(what string) {
	select {
	case devProgress <- what:
	default:
	}
}

// View satisfies tea.Model.
func (m *devModel) View() string {
	if m.quit {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Pusher developer tools"))
	b.WriteString("\n\n")

	switch m.screen {
	case devScreenReport:
		b.WriteString(m.viewDevReport())
	case devScreenReload:
		b.WriteString(m.viewDevReload())
	default:
		b.WriteString(m.viewDevMain())
	}

	switch {
	case m.busy != "":
		status := "  … " + m.busy
		if m.elapsed >= time.Second {
			status += fmt.Sprintf("   %s elapsed", m.elapsed.Round(time.Second))
		}
		b.WriteString("\n" + scrollStyle.Render(status) + "\n")
	case m.err != nil:
		b.WriteString("\n" + errStyle.Render("  ! "+m.err.Error()) + "\n")
	case m.status != "":
		b.WriteString("\n" + okStyle.Render("  ✓ "+m.status) + "\n")
	}

	return b.String()
}

func (m *devModel) viewDevMain() string {
	var b strings.Builder

	for _, line := range wrap(devBanner, textWidth(m.width)) {
		b.WriteString("  " + errStyle.Render(line) + "\n")
	}
	b.WriteString("\n")

	robot := "no robot connected"
	if m.serial != "" {
		robot = "robot: " + m.serial
	}
	fmt.Fprintf(&b, "  %s\n", helpStyle.Render(robot))

	apk := "no APK built yet"
	if m.apk != "" {
		apk = m.apk
		if len(m.splits) > 1 {
			apk += fmt.Sprintf("  (+%d split(s))", len(m.splits)-1)
		}
	}
	fmt.Fprintf(&b, "  %s\n\n", helpStyle.Render(apk))

	list := m.layout()
	for i, row := range list.Rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorOn.Render("> ")
		}
		b.WriteString(list.render(i, fmt.Sprintf("%s%s\n", cursor, devItems[row])))
	}

	b.WriteString(note(devHelp, list.Rows[m.cursor], m.width))

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter run · up/down move · q quit", textWidth(m.width))) + "\n")
	return b.String()
}

func (m *devModel) viewDevReport() string {
	var b strings.Builder

	b.WriteString("  " + titleStyle.Render("Results") + "\n\n")

	if m.summary != "" {
		b.WriteString(m.summary)
		b.WriteString("\n")
	}

	if m.saved != "" {
		fmt.Fprintf(&b, "  %s\n", okStyle.Render("Full report: "+m.saved))
	} else {
		b.WriteString("  " + helpStyle.Render("The report could not be saved to the project.") + "\n")
	}

	b.WriteString("\n  " + helpStyle.Render("Pusher is not a Sloth replacement. The report says why.") + "\n")
	b.WriteString("\n" + helpStyle.Render("  esc back") + "\n")

	return b.String()
}

// DevTargets resolves what the menu should measure against.
func DevTargets() (project, apk string, splits []string) {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return "", "", nil
	}

	project = gradle.ProjectDir(wrapper)
	if found, err := gradle.FindApk(project); err == nil {
		apk = found
	}

	return project, apk, gradle.FindSplits(project)
}

// tryReload runs the hot reload experiment.
//
// The attempt number goes into the OpMode's name, so a second run proves a
// reload rather than a first load: the entry on the Driver Station has to
// change, not merely be present.
func (m *devModel) tryReload() tea.Cmd {
	if m.serial == "" {
		m.err = fmt.Errorf("no robot connected - plug in USB or run `pusher connect`")
		return nil
	}

	m.busy = "compiling an OpMode"
	m.started = time.Now()
	m.elapsed = 0

	// The marker is the clock, not a counter: `pusher dev` is a fresh process
	// every launch, so a counter restarts at one and two runs look identical
	// on the Driver Station.
	serial, marker := m.serial, time.Now().Format("15:04:05")
	motor := hotreload.NextMotor(serial)

	work := func() tea.Msg {
		post("compiling an OpMode that binds " + motor)
		result := hotreload.Run(serial, marker, motor)
		if result.Err != nil {
			return reloadDoneMsg{result: result, err: result.Err}
		}
		return reloadDoneMsg{result: result}
	}

	return tea.Batch(work, waitForProgress, devTick())
}

func (m *devModel) cleanReload() tea.Cmd {
	if m.serial == "" {
		m.err = fmt.Errorf("no robot connected")
		return nil
	}

	m.busy = "removing the proof"
	m.started = time.Now()

	serial := m.serial

	return tea.Batch(func() tea.Msg {
		if err := hotreload.Clean(serial); err != nil {
			return reloadDoneMsg{err: err}
		}
		return reloadDoneMsg{result: nil}
	}, devTick())
}

func (m *devModel) viewDevReload() string {
	var b strings.Builder

	r := m.reload
	if r == nil {
		return "  nothing to show\n"
	}

	b.WriteString("  " + titleStyle.Render("Hot reload attempt") + "\n\n")

	for _, step := range r.Steps {
		fmt.Fprintf(&b, "  %s\n", helpStyle.Render(step))
	}

	if e := r.Diagnosis.Exception; e != "" {
		b.WriteString("\n  " + errStyle.Render("The robot threw while reloading:") + "\n")
		for _, line := range wrapAt(e, 92) {
			fmt.Fprintf(&b, "    %s\n", errStyle.Render(line))
		}
	}
	if p := r.Diagnosis.LogPath; p != "" {
		fmt.Fprintf(&b, "  %s\n", helpStyle.Render("full log: "+p))
	}

	if d := r.Diagnosis; !d.OK() {
		b.WriteString("\n  " + errStyle.Render("Something is wrong on the robot:") + "\n")
		for _, finding := range d.Findings {
			fmt.Fprintf(&b, "    %s\n", errStyle.Render(finding))
		}
		if d.Pointer != "" {
			fmt.Fprintf(&b, "\n  %s\n", helpStyle.Render("pointer file says: "+d.Pointer))
		}
		if d.OutputDir != "" {
			fmt.Fprintf(&b, "\n  %s\n", helpStyle.Render("directory the SDK reads: "+d.OutputDir))
		}
		for _, line := range d.Tree {
			fmt.Fprintf(&b, "    %s\n", helpStyle.Render(trim(line, 96)))
		}
		if d.Crash != "" {
			b.WriteString("\n  " + helpStyle.Render("Most recent crash:") + "\n")
			for _, line := range strings.Split(d.Crash, "\n") {
				fmt.Fprintf(&b, "    %s\n", helpStyle.Render(trim(line, 96)))
			}
		}
		b.WriteString("\n" + helpStyle.Render("  esc back") + "\n")
		return b.String()
	}

	b.WriteString("\n  " + okStyle.Render("Now look at the Driver Station.") + "\n\n")
	fmt.Fprintf(&b, "  Look for an OpMode called %s in the TeleOp list.\n",
		valueStyle.Render(`"`+r.OpModeName+`"`))
	fmt.Fprintf(&b, "  Run it: it binds the motor named %s and shows its encoder.\n",
		valueStyle.Render(r.Motor))

	if r.ColdStart {
		b.WriteString("\n  " + scrollStyle.Render("First run on this hub, so a restart is needed once.") + "\n")
		b.WriteString("  " + helpStyle.Render("The app attaches its watch when it starts, and the directory") + "\n")
		b.WriteString("  " + helpStyle.Render("it watches did not exist until just now. Restart the robot,") + "\n")
		b.WriteString("  " + helpStyle.Render("then run this again: from then on it should be live.") + "\n")
	} else {
		b.WriteString("  " + helpStyle.Render("No restart needed. The time in the name should change within") + "\n")
		b.WriteString("  " + helpStyle.Render("a second or two; reopen the list if it does not.") + "\n")
	}

	b.WriteString("\n  " + helpStyle.Render("Changed: a live reload, no install and no restart.") + "\n")
	b.WriteString("  " + helpStyle.Render("Only after a restart: the files are found but the watch is not firing.") + "\n")
	b.WriteString("  " + helpStyle.Render("Never: the files are not where the SDK reads them.") + "\n")
	b.WriteString("\n  " + helpStyle.Render("Run this again and it switches to the other motor, so the binding") + "\n")
	b.WriteString("  " + helpStyle.Render("changing is what proves the class was replaced.") + "\n")

	b.WriteString("\n" + helpStyle.Render("  esc back") + "\n")
	return b.String()
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// wrapAt breaks a long message so a narrow terminal does not scroll the start
// of it away.
func wrapAt(s string, width int) []string {
	var out []string
	for len(s) > width {
		cut := strings.LastIndex(s[:width], " ")
		if cut <= 0 {
			cut = width
		}
		out = append(out, s[:cut])
		s = strings.TrimSpace(s[cut:])
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// benchExtreme times a real reload of the project's own team code.
func (m *devModel) benchExtreme() tea.Cmd {
	if m.serial == "" {
		m.err = fmt.Errorf("no robot connected - plug in USB or run `pusher connect`")
		return nil
	}

	project, err := extreme.FindProject()
	if err != nil {
		m.err = fmt.Errorf("run this from your FTC project")
		return nil
	}
	if !extreme.Excluded(project.Root) {
		m.err = fmt.Errorf("set Pusher Extreme up first: `pusher settings` -> Pusher Extreme")
		return nil
	}

	// The same check a deploy makes. Reloading onto a robot whose APK does not
	// match this project leaves classes that were deliberately kept out of the
	// reload present in neither place, and the robot crashes on init resolving
	// them. A measuring tool must not be able to do that.
	apk, _ := gradle.FindApk(project.Root)
	if state := extreme.Status(project.Root, m.serial, apk); !state.Usable() {
		m.err = fmt.Errorf("cannot reload yet: %s\n    run `pusher` first", state.Reason)
		return nil
	}

	m.busy = "starting"
	m.started = time.Now()
	m.elapsed = 0

	serial, root := m.serial, project.Root

	work := func() tea.Msg {
		result := extreme.Benchmark(project, serial, extreme.Kept(root), extremeRuns, post)
		if result.Err != nil {
			return devDoneMsg{err: result.Err}
		}

		body := result.Report()
		saved, err := bench.SaveReport(root, body)
		if err != nil {
			saved = ""
		}

		return devDoneMsg{report: body, summary: extremeSummary(result), saved: saved}
	}

	return tea.Batch(work, waitForProgress, devTick())
}

// extremeRuns is how many reloads are timed. Enough to see the spread without
// making the menu sit there for a minute.
const extremeRuns = 5

func extremeSummary(r extreme.BenchResult) string {
	var b strings.Builder
	for _, phase := range []extreme.Phase{r.Classpath, r.Compile, r.Deliver, r.Total} {
		fmt.Fprintf(&b, "  %-22s %s\n", phase.Name, phase.Best().Round(time.Millisecond))
	}
	return b.String()
}

// collectLogs pulls the robot controller's own logs, which outlive the crash
// that adb logcat loses.
func (m *devModel) collectLogs() tea.Cmd {
	if m.serial == "" {
		m.err = fmt.Errorf("no robot connected - plug in USB or run `pusher connect`")
		return nil
	}

	m.busy = "pulling the robot's logs"
	m.started = time.Now()

	serial := m.serial

	return tea.Batch(func() tea.Msg {
		path, crash, err := hotreload.CollectRobotLog(serial)
		if err != nil {
			return devDoneMsg{err: err}
		}

		var b strings.Builder
		if crash == "" {
			b.WriteString("No crash found in the robot controller's logs.\n")
		} else {
			b.WriteString("Most recent crash:\n\n")
			b.WriteString(crash)
			b.WriteString("\n")
		}

		return devDoneMsg{
			report:  b.String(),
			summary: logSummary(crash),
			saved:   path,
		}
	}, devTick())
}

// logSummary keeps the menu to a few lines; the whole thing is in the file.
func logSummary(crash string) string {
	if crash == "" {
		return "  no crash found in the robot's own logs\n"
	}

	var b strings.Builder
	for i, line := range strings.Split(crash, "\n") {
		if i >= 6 {
			b.WriteString("  ...\n")
			break
		}
		fmt.Fprintf(&b, "  %s\n", trim(line, 96))
	}
	return b.String()
}
