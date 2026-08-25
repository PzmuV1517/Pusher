// Package power measures what a robot's motors actually draw, so the answer to
// "what is flattening the battery" is a measurement rather than an argument.
package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
)

// Dir is where the generated monitor lives inside the team's project.
func Dir(root string) string {
	return filepath.Join(root, extreme.SourceRoot,
		filepath.FromSlash(extreme.TeamPackage), Package)
}

// File is the generated monitor itself.
func File(root string) string {
	return filepath.Join(Dir(root), Class+".java")
}

// Installed reports whether the monitor is in the project.
func Installed(root string) bool {
	generated, err := isGenerated(File(root))
	return err == nil && generated
}

// isGenerated reports whether a file is pusher's rather than somebody's.
//
// Checked before writing and before deleting. A team that has written their own
// class at this path is not going to have it silently replaced, and a marker in
// the file is the only way to know the difference.
func isGenerated(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(string(content), Marker), nil
}

// Install writes the monitor into the project.
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
	if err := os.WriteFile(path, []byte(sourceFor(config.GetPowerPeriod())), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}

	// A monitor that reloads instead of shipping in the APK never runs: the
	// robot controller collects the startup hook while scanning, but calls the
	// ones it collected when the web server starts, which has already happened
	// by the time anything is reloaded. Keeping it in the APK is the whole fix,
	// and re-running the exclusion is what puts it in the keep list.
	if extreme.Excluded(root) {
		if err := extreme.Exclude(root, extreme.Kept(root)...); err != nil {
			return fmt.Errorf("the monitor is installed, but Pusher Extreme could not be told to keep it in the APK: %w", err)
		}
	}

	return nil
}

// Remove takes the monitor back out.
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
	// put something of their own beside the monitor.
	os.Remove(Dir(root))

	if extreme.Excluded(root) {
		if err := extreme.Exclude(root, extreme.Kept(root)...); err != nil {
			return fmt.Errorf("the monitor is removed, but the gradle block still names it: %w", err)
		}
	}

	return nil
}
