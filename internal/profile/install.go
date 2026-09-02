// Package profile samples what a robot is in the middle of while an OpMode
// runs, so "what is eating my loop time" is a measurement rather than a guess.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
)

// DefaultPeriod is how often the OpMode's thread is sampled, in milliseconds.
//
// Ten a second is too coarse to see inside a twenty millisecond loop, and one a
// millisecond costs more than the thing being measured. A hundred a second
// resolves a loop into its parts and stops the robot for a stack walk one time
// in two hundred.
const DefaultPeriod = 10

// Dir is where the generated profiler lives inside the team's project.
func Dir(root string) string {
	return filepath.Join(root, extreme.SourceRoot,
		filepath.FromSlash(extreme.TeamPackage), Package)
}

// File is the generated profiler itself.
func File(root string) string {
	return filepath.Join(Dir(root), Class+".java")
}

// Installed reports whether the profiler is in the project.
func Installed(root string) bool {
	generated, err := isGenerated(File(root))
	return err == nil && generated
}

// isGenerated reports whether a file is pusher's rather than somebody's.
//
// Checked before writing and before deleting. A team that has written their own
// class at this path is not going to have it silently replaced, and a marker in
// the file is the only way to tell the difference.
func isGenerated(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(string(content), Marker), nil
}

// Install writes the profiler into the project.
func Install(root string) error {
	if _, err := os.Stat(filepath.Join(root, extreme.SourceRoot)); err != nil {
		return fmt.Errorf("no %s here, so this is not an FTC project", extreme.SourceRoot)
	}

	path := File(root)

	if _, err := os.Stat(path); err == nil {
		generated, err := isGenerated(path)
		if err != nil {
			return err
		}
		if !generated {
			return fmt.Errorf("%s already exists and pusher did not write it, so it is not going to overwrite it", path)
		}
	}

	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(sourceFor(config.GetProfilePeriod())), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}

	// A profiler that is reloaded rather than packaged never runs: the robot
	// controller collects the startup hook while scanning, but calls the ones it
	// collected when the web server comes up, which has already happened by the
	// time anything is reloaded. Keeping it in the APK is the whole fix, and
	// re-running the exclusion is what puts it in the keep list.
	if extreme.Excluded(root) {
		if err := extreme.Exclude(root, extreme.Kept(root)...); err != nil {
			return fmt.Errorf("the profiler is installed, but Pusher Extreme could not be told to keep it in the APK: %w", err)
		}
	}

	return nil
}

// Remove takes the profiler back out.
func Remove(root string) error {
	path := File(root)

	generated, err := isGenerated(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !generated {
		return fmt.Errorf("%s was not written by pusher, so it is being left alone", path)
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	// Leaves the directory only if it is now empty, which it is unless somebody
	// put something of their own beside the profiler.
	os.Remove(Dir(root))

	if extreme.Excluded(root) {
		if err := extreme.Exclude(root, extreme.Kept(root)...); err != nil {
			return fmt.Errorf("the profiler is removed, but the gradle block still names it: %w", err)
		}
	}

	return nil
}
