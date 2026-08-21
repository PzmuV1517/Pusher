package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/blobdep"
	"github.com/andreibanu/pusher/internal/blobrel"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/ghauth"
	"github.com/andreibanu/pusher/internal/pathtrace"
	"github.com/andreibanu/pusher/internal/visual"
	tea "github.com/charmbracelet/bubbletea"
)

type blobState struct {
	pickerOnly bool

	auth  ghauth.Status
	creds ghauth.Credentials

	checking bool
	busy     bool

	dep    *blobdep.Dep
	latest string

	branches   []string
	branchErr  error
	branchBusy bool

	traces  []adb.RemoteTrace
	serial  string
	tracErr error

	limits pathtrace.Limits
}

var (
	blobItems        = []string{"Build variant", "Release branch", "Version", "GitHub token", "Recorded runs", "Back"}
	blobMissingItems = []string{"Add blob to the project", "GitHub token", "Back"}
	blobLockedItems  = []string{"GitHub token", "Back"}
)

type blobAuthMsg struct {
	status ghauth.Status
	creds  ghauth.Credentials
}

type blobOpMsg struct {
	status string
	err    error
}

func checkBlobAuth() tea.Msg {
	status, creds := ghauth.Resolve()
	return blobAuthMsg{status: status, creds: creds}
}

func blobOp(run func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		status, err := run()
		return blobOpMsg{status: status, err: err}
	}
}

// RunTracePicker opens the menu for choosing a run to render.
func RunTracePicker(projectRoot string, lim pathtrace.Limits) error {
	m, err := NewSettingsModel()
	if err != nil {
		return err
	}
	if projectRoot != "" {
		m.root = projectRoot
	}
	m.blob.limits = lim

	m.loadTraces()
	m.blob.pickerOnly = true
	m.screen = screenBlobRuns

	_, err = tea.NewProgram(m).Run()
	return err
}

func (m *SettingsModel) enterBlob() tea.Cmd {
	m.refreshBlob()
	m.blob.latest = ""
	m.blob.checking = true
	m.goTo(screenBlob, 0)
	return checkBlobAuth
}

func (m *SettingsModel) refreshBlob() {
	dep, err := blobdep.Detect(m.projectRoot())
	if err != nil {

		m.blob.dep = nil
		return
	}
	m.blob.dep = dep
}

func (m *SettingsModel) blobMenuItems() []string {
	switch {
	case !m.blob.auth.OK():
		return blobLockedItems
	case m.blob.dep == nil:
		return blobMissingItems
	}
	return blobItems
}

func (m *SettingsModel) blobLabel() string {
	if m.blob.dep == nil {
		return "not installed"
	}
	variant := "comp"
	if m.blob.dep.IsDev() {
		variant = "dev"
	}
	return m.blob.dep.Version + " (" + variant + ")"
}

func (m *SettingsModel) tokenLabel() string {
	if m.blob.creds.Login == "" || !m.blob.auth.OK() {
		return m.blob.auth.String()
	}

	who := m.blob.creds.Login

	if m.blob.creds.Discovered() {
		if from := ghauth.SourceLabel(m.blob.creds.Source); from != "" {
			who += " via " + from
		}
	}
	return m.blob.auth.String() + " (" + who + ")"
}

func (m *SettingsModel) updateBlob(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.blobMenuItems()

	switch key.String() {
	case "esc", "q", "left", "h":
		if m.blob.busy {
			return m, nil
		}
		m.goTo(screenMain, 0)
		m.status = ""

	case "up", "k":
		m.moveCursor(-1, len(items))
	case "down", "j":
		m.moveCursor(1, len(items))

	case "enter", " ", "right", "l":
		if m.blob.busy || m.blob.checking {
			return m, nil
		}
		m.status = ""
		m.err = nil

		return m.chooseBlob(items[m.cursor])
	}

	return m, nil
}

func (m *SettingsModel) chooseBlob(item string) (tea.Model, tea.Cmd) {
	switch item {
	case "GitHub token":
		m.input = ""
		m.maskInput = true
		m.goTo(screenBlobToken, 0)

	case "Back":
		m.goTo(screenMain, 0)

	case "Build variant":
		return m, m.switchVariant()

	case "Release branch":
		return m, m.enterBranches()

	case "Version":
		return m, m.bumpVersion()

	case "Add blob to the project":
		return m, m.addBlob()

	case "Recorded runs":
		m.loadTraces()
		m.goTo(screenBlobRuns, 0)
	}

	return m, nil
}

func ensureLibrary(root, token, artifact, version string) error {
	if tracked := blobdep.TrackedAARs(root); len(tracked) > 0 {
		return fmt.Errorf("git is already tracking %s.\n"+
			"blob is private and FTC team repositories are usually public, so this\n"+
			"would publish it. Untrack it first:\n"+
			"    git rm --cached %s",
			strings.Join(tracked, ", "), strings.Join(tracked, " "))
	}

	if _, err := blobdep.EnsureIgnored(root); err != nil {
		return err
	}

	if _, err := os.Stat(blobdep.AARPath(root, artifact, version)); err == nil {
		return nil
	}

	data, err := blobrel.Fetch(token, blobrel.Variant(artifact), version)
	if err != nil {
		return err
	}
	return blobdep.Place(root, artifact, version, data)
}

func (m *SettingsModel) switchVariant() tea.Cmd {
	root, token := m.projectRoot(), m.blob.creds.Secret()
	version := m.blob.dep.Version

	target := blobdep.ArtifactDev
	if m.blob.dep.IsDev() {
		target = blobdep.ArtifactComp
	}

	m.blob.busy = true
	return blobOp(func() (string, error) {
		if err := ensureLibrary(root, token, target, version); err != nil {
			return "", err
		}
		if err := blobdep.SetArtifact(root, target); err != nil {
			return "", err
		}
		blobdep.Prune(root, target, version)

		if target == blobdep.ArtifactDev {
			return "Switched to dev. Records traces. Do not take this to a match. Gradle sync + redeploy.", nil
		}
		return "Switched to competition. No logging code in the APK. Gradle sync + redeploy.", nil
	})
}

func (m *SettingsModel) bumpVersion() tea.Cmd {
	root, token := m.projectRoot(), m.blob.creds.Secret()
	artifact, previous := m.blob.dep.Artifact, m.blob.dep.Version

	// Newest on the branch this project follows, not newest overall. Updating
	// somebody off their branch and onto main is not an update, it is undoing
	// the choice they made in the entry above this one.
	branch := config.GetBlobBranch()

	m.blob.busy = true
	return blobOp(func() (string, error) {
		latest, err := blobrel.LatestOn(token, branch)
		if err != nil {
			return "", err
		}
		if latest == previous {
			return "Already on " + latest, nil
		}

		if err := ensureLibrary(root, token, artifact, latest); err != nil {
			return "", err
		}
		if err := blobdep.SetVersion(root, latest); err != nil {
			return "", err
		}
		blobdep.Prune(root, artifact, latest)

		return fmt.Sprintf("Updated %s to %s. Gradle sync to pick it up.", previous, latest), nil
	})
}

func (m *SettingsModel) addBlob() tea.Cmd {
	root, token := m.projectRoot(), m.blob.creds.Secret()

	branch := config.GetBlobBranch()

	m.blob.busy = true
	return blobOp(func() (string, error) {
		version, err := blobrel.LatestOn(token, branch)
		if err != nil {

			return "", err
		}

		if err := ensureLibrary(root, token, blobdep.ArtifactComp, version); err != nil {
			return "", err
		}
		if err := blobdep.Add(root, blobdep.ArtifactComp, version); err != nil {
			return "", err
		}

		return fmt.Sprintf("Added blob %s (competition build) to TeamCode/libs. Gradle sync to pick it up.", version), nil
	})
}

func (m *SettingsModel) saveToken(token string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(token) == "" {
			if err := ghauth.Clear(); err != nil {
				return blobAuthMsg{status: ghauth.NoToken}
			}
			return blobAuthMsg{status: ghauth.NoToken}
		}

		creds, err := ghauth.SetToken(strings.TrimSpace(token))
		if err != nil {
			return blobOpMsg{err: err}
		}
		return blobAuthMsg{status: ghauth.Verified, creds: creds}
	}
}

func (m *SettingsModel) updateBlobToken(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.input = ""
		m.maskInput = false
		m.goTo(screenBlob, 0)
		m.status = "Cancelled"
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	case tea.KeyEnter:
		token := m.input

		m.input = ""
		m.maskInput = false
		m.blob.busy = true
		m.goTo(screenBlob, 0)
		return m, m.saveToken(token)

	case tea.KeyRunes:
		m.input += string(key.Runes)
		return m, nil
	}

	return m, nil
}

func (m *SettingsModel) loadTraces() {
	serial, traces, err := visual.List()
	m.blob.serial = serial
	m.blob.traces = traces
	m.blob.tracErr = err
}

func (m *SettingsModel) updateBlobRuns(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		if m.blob.pickerOnly {
			m.quit = true
			return m, tea.Quit
		}
		m.goTo(screenBlob, 3)
		m.status = ""

	case "r":
		m.loadTraces()
		m.cursor = 0

	case "up", "k":
		m.moveCursor(-1, len(m.blob.traces))
	case "down", "j":
		m.moveCursor(1, len(m.blob.traces))

	case "enter", " ":
		if len(m.blob.traces) == 0 {
			return m, nil
		}

		trace := m.blob.traces[m.cursor]
		out, err := visual.Render(m.blob.serial, trace, m.projectRoot(), "", m.blob.limits)
		if err != nil {
			m.err = err
			return m, nil
		}
		visual.Open(out)
		m.status = "Opened " + out
	}

	return m, nil
}

func (m *SettingsModel) viewBlobToken() string {
	var b strings.Builder

	b.WriteString(helpStyle.Render("  A GitHub token with read access to the private blob repository.") + "\n")
	b.WriteString(helpStyle.Render("  Classic tokens need the repo scope. Fine-grained tokens need") + "\n")
	b.WriteString(helpStyle.Render("  Contents: Read on that repository.") + "\n\n")
	b.WriteString(helpStyle.Render("  Only needed if this machine has no GitHub login pusher can use.") + "\n")
	b.WriteString(helpStyle.Render("  It already tried GH_TOKEN, the gh CLI and git's credential helper.") + "\n\n")

	b.WriteString(fmt.Sprintf("  Token: %s\n", strings.Repeat("*", len(m.input))))

	b.WriteString("\n" + helpStyle.Render("  Stored in ~/.config/pusher/credentials, readable only by you,") + "\n")
	b.WriteString(helpStyle.Render("  and never written into the FTC project.") + "\n")
	b.WriteString("\n" + helpStyle.Render("  enter save · empty + enter removes · esc cancel") + "\n")
	return b.String()
}

func (m *SettingsModel) viewBlob() string {
	var b strings.Builder
	items := m.blobMenuItems()

	if m.blob.checking {
		b.WriteString(helpStyle.Render("  Checking access...") + "\n\n")
	}

	if !m.blob.auth.OK() && !m.blob.checking {
		b.WriteString(m.blobLockedNotice())
	}

	values := make([]string, len(items))
	for i, item := range items {
		switch item {
		case "Build variant":
			values[i] = m.blob.dep.VariantName()
		case "Release branch":
			values[i] = m.branchValue()
		case "Version":
			values[i] = m.versionValue()
		case "GitHub token":
			values[i] = m.tokenLabel()
		}
	}

	b.WriteString(m.renderList(len(items), func(i int) string {
		return renderRow(i == m.cursor, items[i], values[i], 29, m.width)
	}))

	b.WriteString("\n")
	if m.blob.busy {
		b.WriteString(okStyle.Render("  Working...") + "\n")
	} else if m.blob.dep != nil && !m.blob.dep.Present && m.blob.auth.OK() {
		b.WriteString(errStyle.Render("  The AAR this project references is missing from TeamCode/libs.") + "\n")
		b.WriteString(helpStyle.Render("  Switching variant or version downloads it again.") + "\n")
	}

	b.WriteString(helpStyle.Render("  "+fit("↑/↓ move · enter select · esc back", textWidth(m.width))) + "\n")
	return b.String()
}

// branchValue says which branch this project follows, and whether the version
// it is on came from somewhere else.
//
// Worth showing rather than assuming: a project can sit on a branch build long
// after somebody switched the setting back, and the two disagreeing is exactly
// when it matters.
func (m *SettingsModel) branchValue() string {
	branch := config.GetBlobBranch()

	if m.blob.dep == nil || m.blob.dep.Version == "" {
		return branch
	}

	if on := blobrel.Branch(m.blob.dep.Version); on != branch {
		return branch + " (on " + on + ")"
	}
	return branch
}

func (m *SettingsModel) versionValue() string {
	if m.blob.dep == nil {
		return ""
	}
	if m.blob.latest == "" {
		return m.blob.dep.Version
	}
	if m.blob.latest == m.blob.dep.Version {
		return m.blob.dep.Version + " (latest)"
	}
	return m.blob.dep.Version + " -> " + m.blob.latest
}

func (m *SettingsModel) viewBlobRuns() string {
	var b strings.Builder

	if m.blob.tracErr != nil {
		for _, line := range strings.Split(m.blob.tracErr.Error(), "\n") {
			b.WriteString(helpStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  r retry · esc back") + "\n")
		return b.String()
	}

	if len(m.blob.traces) == 0 {
		b.WriteString(helpStyle.Render("  No recorded runs on the robot.") + "\n")
		b.WriteString("\n" + helpStyle.Render("  r refresh · esc back") + "\n")
		return b.String()
	}

	b.WriteString(m.renderList(len(m.blob.traces), func(i int) string {
		t := m.blob.traces[i]
		note := ""
		if i == 0 {
			note = "newest"
		}
		return renderRow(i == m.cursor, t.OpMode, note, 29, m.width)
	}))

	b.WriteString("\n" + helpStyle.Render("  enter opens the visualiser · r refresh · esc back") + "\n")
	return b.String()
}

func (m *SettingsModel) blobLockedNotice() string {
	var b strings.Builder

	// Wrapped rather than fitted: this is a sentence somebody has to read to
	// know what to do, and cutting it at the terminal's edge would take the
	// instruction away with it.
	notice := "blob is a private library. Using it needs a GitHub token with read access to the repository."
	style := helpStyle

	if m.blob.auth == ghauth.Denied {
		notice = "This token cannot read the blob repository. Ask for access, or set a token that has it."
		style = errStyle
	}

	for _, line := range wrap(notice, textWidth(m.width)) {
		b.WriteString(style.Render("  "+line) + "\n")
	}
	b.WriteString("\n")

	return b.String()
}

// Switching branch is one action rather than two. Choosing a branch and being
// left on the version from the last one would be a setting that changed and a
// project that did not, which is the shape of every bug in this tool worth
// having: something reported as done that has not happened yet.

type blobBranchMsg struct {
	branches []string
	err      error
}

func (m *SettingsModel) enterBranches() tea.Cmd {
	m.blob.branches = nil
	m.blob.branchErr = nil
	m.blob.branchBusy = true
	m.goTo(screenBlobBranch, 0)

	token := m.blob.creds.Secret()
	return func() tea.Msg {
		branches, err := blobrel.Branches(token)
		return blobBranchMsg{branches: branches, err: err}
	}
}

func (m *SettingsModel) updateBlobBranch(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenBlob, 1)
		m.status = ""

	case "r":
		return m, m.enterBranches()

	case "up", "k":
		m.moveCursor(-1, len(m.blob.branches))
	case "down", "j":
		m.moveCursor(1, len(m.blob.branches))

	case "enter", " ":
		if m.blob.branchBusy || len(m.blob.branches) == 0 {
			return m, nil
		}
		return m, m.followBranch(m.blob.branches[m.cursor])
	}

	return m, nil
}

// followBranch records the choice and moves the project onto that branch's
// newest release.
func (m *SettingsModel) followBranch(branch string) tea.Cmd {
	root, token := m.projectRoot(), m.blob.creds.Secret()

	var artifact, previous string
	if m.blob.dep != nil {
		artifact, previous = m.blob.dep.Artifact, m.blob.dep.Version
	}

	m.blob.busy = true
	m.goTo(screenBlob, 1)

	return blobOp(func() (string, error) {
		if err := config.SetBlobBranch(branch); err != nil {
			return "", err
		}

		latest, err := blobrel.LatestOn(token, branch)
		if err != nil {
			return "", err
		}

		if artifact == "" {
			return "Following " + branch + ". Newest there is " + latest, nil
		}
		if latest == previous {
			return "Following " + branch + ", already on " + latest, nil
		}

		if err := ensureLibrary(root, token, artifact, latest); err != nil {
			return "", err
		}
		if err := blobdep.SetVersion(root, latest); err != nil {
			return "", err
		}
		blobdep.Prune(root, artifact, latest)

		return fmt.Sprintf("Following %s: %s to %s. Gradle sync to pick it up.",
			branch, previous, latest), nil
	})
}

func (m *SettingsModel) viewBlobBranch() string {
	var b strings.Builder

	b.WriteString(helpStyle.Render("  "+fit("Which of blob's branches this project follows.", textWidth(m.width))) + "\n")
	b.WriteString(helpStyle.Render("  "+fit("Choosing one moves the project to the newest release from it.", textWidth(m.width))) + "\n\n")

	switch {
	case m.blob.branchBusy:
		b.WriteString(helpStyle.Render("  Looking...") + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("esc back", textWidth(m.width))) + "\n")
		return b.String()

	case m.blob.branchErr != nil:
		for _, line := range strings.Split(m.blob.branchErr.Error(), "\n") {
			b.WriteString(errStyle.Render("  "+fit(line, textWidth(m.width))) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("  "+fit("r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()

	case len(m.blob.branches) == 0:
		b.WriteString(helpStyle.Render("  "+fit("No releases at all, so there is nothing to follow.", textWidth(m.width))) + "\n")
		b.WriteString("\n" + helpStyle.Render("  "+fit("r retry · esc back", textWidth(m.width))) + "\n")
		return b.String()
	}

	current := config.GetBlobBranch()

	b.WriteString(m.renderList(len(m.blob.branches), func(i int) string {
		name := m.blob.branches[i]

		value := ""
		if name == current {
			value = "following"
		} else if name != blobrel.MainBranch {
			value = "branch work"
		}

		return renderRow(i == m.cursor, name, value, 29, m.width)
	}))

	b.WriteString("\n" + helpStyle.Render("  "+fit("enter follow · r refresh · esc back", textWidth(m.width))) + "\n")
	return b.String()
}
