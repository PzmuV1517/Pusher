package ftcproject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const backupSuffix = ".pusher-bak"

var abiFiltersRe = regexp.MustCompile(`(?m)^([ \t]*)abiFilters\s+(.+)$`)

var quotedRe = regexp.MustCompile(`["']([^"']+)["']`)

var sourceMapPatternRe = regexp.MustCompile(`ignoreAssetsPatterns`)

// Project is an FTC project and the gradle files pusher may patch.
type Project struct {
	Root string

	CommonGradle string

	TeamCodeGradle string
}

// Supported reports why this project cannot be slimmed, or nil when it can.
//
// Every patch slim makes is Groovy: `abiFilters "x"` and
// `jniLibs.useLegacyPackaging true`. The Kotlin DSL writes both differently, so
// the patterns match nothing, and a deploy would go on packaging everything it
// always did while reporting that it had been slimmed.
//
// This is settled rather than pending. Pusher Extreme learned the Kotlin DSL
// because one generated block covers every project; slim instead edits lines a
// team wrote themselves, in whichever of several shapes they wrote them, and
// replacing a `+=` on a collection is not the same edit as replacing a literal.
// Guessing at that on a repository nobody can test against is how a deploy ends
// up silently packaging everything.
func Supported(root string) error {
	if _, err := os.Stat(filepath.Join(root, "build.common.gradle")); err == nil {
		return nil
	}

	for _, name := range []string{
		filepath.Join(root, "build.common.gradle.kts"),
		filepath.Join(root, "build.gradle.kts"),
		filepath.Join(root, "TeamCode", "build.gradle.kts"),
	} {
		if _, err := os.Stat(name); err == nil {
			return fmt.Errorf("`pusher slim` WILL NOT WORK on this project: it is " +
				"configured with the Kotlin DSL, which slim does not support and is " +
				"not going to")
		}
	}

	return fmt.Errorf("no build.common.gradle here, so there is nothing for " +
		"`pusher slim` to edit")
}

// Detect confirms a directory is an FTC project and locates its gradle files.
func Detect(root string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", root, err)
	}

	proj := &Project{
		Root:           abs,
		CommonGradle:   filepath.Join(abs, "build.common.gradle"),
		TeamCodeGradle: filepath.Join(abs, "TeamCode", "build.gradle"),
	}

	if _, err := os.Stat(proj.CommonGradle); err != nil {
		return nil, fmt.Errorf("this does not look like an FTC project: no build.common.gradle in %s", abs)
	}

	return proj, nil
}

// Analysis is what the project currently builds.
type Analysis struct {
	ABIs []string

	StripsSourceMaps bool

	HasBackups bool

	CompressesLibs bool
}

// Analyze reads what the project currently builds.
func (p *Project) Analyze() (*Analysis, error) {
	common, err := os.ReadFile(p.CommonGradle)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", p.CommonGradle, err)
	}

	analysis := &Analysis{HasBackups: p.HasBackups(), CompressesLibs: p.LegacyPackaging()}

	seen := map[string]bool{}
	for _, match := range abiFiltersRe.FindAllStringSubmatch(string(common), -1) {
		for _, abi := range quotedRe.FindAllStringSubmatch(match[2], -1) {
			if !seen[abi[1]] {
				seen[abi[1]] = true
				analysis.ABIs = append(analysis.ABIs, abi[1])
			}
		}
	}
	sort.Strings(analysis.ABIs)

	if teamCode, err := os.ReadFile(p.TeamCodeGradle); err == nil {
		analysis.StripsSourceMaps = sourceMapPatternRe.Match(teamCode)
	}

	return analysis, nil
}

// build.common.gradle, not TeamCode: AGP unions abiFilters from defaultConfig
// with the build type's, so a narrower list elsewhere merges straight back.
func (p *Project) SetABI(abi string) (bool, error) {
	original, err := os.ReadFile(p.CommonGradle)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", p.CommonGradle, err)
	}

	replacement := fmt.Sprintf(`abiFilters %q`, abi)
	patched := abiFiltersRe.ReplaceAllStringFunc(string(original), func(line string) string {
		match := abiFiltersRe.FindStringSubmatch(line)
		return match[1] + replacement
	})

	if patched == string(original) {
		return false, nil
	}

	if err := p.backup(p.CommonGradle, original); err != nil {
		return false, err
	}

	if err := os.WriteFile(p.CommonGradle, []byte(patched), 0644); err != nil {
		return false, fmt.Errorf("cannot write %s: %w", p.CommonGradle, err)
	}

	return true, nil
}

// StripSourceMaps excludes JavaScript source maps, which the robot never reads.
func (p *Project) StripSourceMaps() (bool, error) {
	original, err := os.ReadFile(p.TeamCodeGradle)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", p.TeamCodeGradle, err)
	}

	if sourceMapPatternRe.Match(original) {
		return false, nil
	}

	block := `// pusher: source maps are debugger-only and never read on the robot.
androidResources {
    ignoreAssetsPatterns.add('*.map')
}`

	patched, err := appendToAndroidBlock(string(original), block)
	if err != nil {
		return false, err
	}

	if err := p.backup(p.TeamCodeGradle, original); err != nil {
		return false, err
	}

	if err := os.WriteFile(p.TeamCodeGradle, []byte(patched), 0644); err != nil {
		return false, fmt.Errorf("cannot write %s: %w", p.TeamCodeGradle, err)
	}

	return true, nil
}

var legacyPackagingRe = regexp.MustCompile(`(?m)^([ \t]*)jniLibs\.useLegacyPackaging[ \t]+(true|false)[ \t]*$`)

// StoreLibs stops native libraries being compressed, so the install does not extract them.
func (p *Project) StoreLibs(enable bool) (bool, error) {
	original, err := os.ReadFile(p.TeamCodeGradle)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", p.TeamCodeGradle, err)
	}

	want := "false"
	if enable {
		want = "true"
	}

	patched := legacyPackagingRe.ReplaceAllString(string(original), "${1}jniLibs.useLegacyPackaging "+want)

	if patched == string(original) {
		if legacyPackagingRe.Match(original) {

			return false, nil
		}
		return false, fmt.Errorf("no jniLibs.useLegacyPackaging line in %s", p.TeamCodeGradle)
	}

	if err := p.backup(p.TeamCodeGradle, original); err != nil {
		return false, err
	}

	if err := os.WriteFile(p.TeamCodeGradle, []byte(patched), 0644); err != nil {
		return false, fmt.Errorf("cannot write %s: %w", p.TeamCodeGradle, err)
	}

	return true, nil
}

// LegacyPackaging reports whether native libraries are still compressed.
func (p *Project) LegacyPackaging() bool {
	content, err := os.ReadFile(p.TeamCodeGradle)
	if err != nil {
		return true
	}

	match := legacyPackagingRe.FindSubmatch(content)
	if match == nil {
		return true
	}
	return string(match[2]) == "true"
}

func appendToAndroidBlock(content, block string) (string, error) {
	start := regexp.MustCompile(`(?m)^android\s*\{`).FindStringIndex(content)
	if start == nil {
		return "", fmt.Errorf("no top-level android { } block found")
	}

	depth := 0
	for i := start[1] - 1; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[:i] + indentBlock(block) + content[i:], nil
			}
		}
	}

	return "", fmt.Errorf("unterminated android { } block")
}

func indentBlock(block string) string {
	var b strings.Builder
	b.WriteString("\n")

	for _, line := range strings.Split(strings.Trim(block, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("    " + line + "\n")
	}

	return b.String()
}

// Never overwrites an existing backup: the first is the only pristine copy.
func (p *Project) backup(path string, content []byte) error {
	backupPath := path + backupSuffix
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("cannot write backup %s: %w", backupPath, err)
	}

	return nil
}

func (p *Project) backupTargets() []string {
	return []string{p.CommonGradle, p.TeamCodeGradle}
}

// HasBackups reports whether pusher has patched anything.
func (p *Project) HasBackups() bool {
	for _, path := range p.backupTargets() {
		if _, err := os.Stat(path + backupSuffix); err == nil {
			return true
		}
	}
	return false
}

// Undo restores every gradle file pusher patched.
func (p *Project) Undo() ([]string, error) {
	var restored []string

	for _, path := range p.backupTargets() {
		backupPath := path + backupSuffix

		content, err := os.ReadFile(backupPath)
		if err != nil {
			continue
		}

		if err := os.WriteFile(path, content, 0644); err != nil {
			return restored, fmt.Errorf("cannot restore %s: %w", path, err)
		}
		if err := os.Remove(backupPath); err != nil {
			return restored, fmt.Errorf("cannot remove backup %s: %w", backupPath, err)
		}

		restored = append(restored, filepath.Base(path))
	}

	if len(restored) == 0 {
		return nil, fmt.Errorf("nothing to undo: no pusher backups found in %s", p.Root)
	}

	return restored, nil
}

// PickABI chooses the architecture the hub runs that the project also packages.
func PickABI(deviceABIs, projectABIs []string) (string, error) {
	if len(deviceABIs) == 0 {
		return "", fmt.Errorf("device reported no ABIs")
	}

	if len(projectABIs) == 0 {
		return deviceABIs[0], nil
	}

	available := map[string]bool{}
	for _, abi := range projectABIs {
		available[abi] = true
	}

	for _, abi := range deviceABIs {
		if available[abi] {
			return abi, nil
		}
	}

	return "", fmt.Errorf("device supports %s but the project only packages %s",
		strings.Join(deviceABIs, ", "), strings.Join(projectABIs, ", "))
}
