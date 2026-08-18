package extreme

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/andreibanu/pusher/internal/javasrc"
)

// A reload can put every class on the robot and still register none of them,
// and pusher cannot see that from here: pushing files is not the same as the
// SDK accepting them. So what was sent is worked out from the source, and
// compared against what the robot says it has.

var (
	opModeRe = regexp.MustCompile(`(?m)^\s*@(TeleOp|Autonomous)\b\s*(\([^)]*\))?`)

	// name = "..." inside the annotation's arguments, in either language.
	opModeNameRe = regexp.MustCompile(`\bname\s*=\s*"([^"]*)"`)

	disabledRe = regexp.MustCompile(`(?m)^\s*@Disabled\b`)
)

// OpMode is one OpMode the team declared.
type OpMode struct {
	// Name is what the Driver Station shows, which is the annotation's name
	// when it has one and the class's own name when it does not.
	Name string

	// Class is the declaration the annotation was attached to.
	Class string

	// File is where it was found, for saying so.
	File string
}

// DeclaredOpModes reads the team's sources for OpModes the robot should end up
// with.
//
// Source rather than bytecode, for the same reason the rest of this reads
// source: it is the only description that exists before anything is compiled,
// and it is what somebody edits when an OpMode does not appear.
func DeclaredOpModes(root string) []OpMode {
	base := filepath.Join(root, SourceRoot)

	var out []OpMode

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isSource(path) {
			return nil
		}

		blob, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Found in the masked copy, so an annotation quoted in a comment does
		// not count as a declaration nobody wrote. Read out of the original,
		// because the name is a string literal and masking is what blanks
		// those. Mask keeps every byte where it was, so one set of offsets
		// addresses both.
		raw := string(blob)
		code := javasrc.Mask(raw)

		match := opModeRe.FindStringSubmatchIndex(code)
		if match == nil {
			return nil
		}

		// @Disabled is registered by nobody, so expecting it on the robot would
		// report a failure every time somebody parks an OpMode.
		if disabledRe.MatchString(code) {
			return nil
		}

		class := className(path, raw, match[1])

		name := class
		if match[4] >= 0 {
			if m := opModeNameRe.FindStringSubmatch(raw[match[4]:match[5]]); m != nil {
				name = m[1]
			}
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			rel = path
		}

		out = append(out, OpMode{Name: name, Class: class, File: filepath.ToSlash(rel)})
		return nil
	})

	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// MissingFrom is the OpModes the robot did not end up with.
//
// Compared by name, because the name is what both sides agree on: the robot
// reports what the Driver Station lists, and never mentions the class.
func MissingFrom(declared []OpMode, onRobot []string) []OpMode {
	has := make(map[string]bool, len(onRobot))
	for _, name := range onRobot {
		has[name] = true
	}

	var out []OpMode
	for _, mode := range declared {
		if !has[mode.Name] {
			out = append(out, mode)
		}
	}
	return out
}

// Summary names a few of them, and says how many it left out.
func Summary(modes []OpMode, limit int) string {
	var names []string
	for i, mode := range modes {
		if i == limit {
			names = append(names, "and "+plural(len(modes)-limit, "more"))
			break
		}
		names = append(names, mode.Name)
	}
	return strings.Join(names, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
