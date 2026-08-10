package extreme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Parent-first delegation means a class present in the APK always wins. So team
// code that is going to be reloaded cannot also be packaged, or the reload
// loads fine and the robot keeps running the old copy with nothing to show for
// it.
//
// This is the one change made to a team's repository, and it is marked rather
// than backed up. `pusher slim` already keeps a .pusher-bak of the same file,
// and two features sharing one backup means undoing either undoes both. A
// marked block can be added and removed exactly, whatever else edited the file
// in between.

const (
	beginMarker = "// pusher extreme: begin - team code is reloaded, not packaged"
	endMarker   = "// pusher extreme: end"
)

// TeamPackage is what gets excluded from the APK.
const TeamPackage = "org/firstinspires/ftc/teamcode"

var blockRe = regexp.MustCompile(`(?s)\n*` + regexp.QuoteMeta(beginMarker) + `.*?` + regexp.QuoteMeta(endMarker) + `\n*`)

var keptRe = regexp.MustCompile(`// Kept in the APK anyway: (.*)`)

// blockFor builds what gets appended to the module's gradle file.
//
// A second android { } is legal: it is a method call configuring the same
// extension, not a declaration, so this composes with whatever is above it
// instead of having to be spliced into it.
//
// The exclusion is a closure rather than a pattern because of the keep list.
// Ant patterns cannot say "everything under here except that", and generating
// one pattern per subpackage would stop covering a package added later.
//
// Directories are always let through. Excluding one prunes everything under it
// before any of it is considered, which drops the kept class along with its
// package and leaves the keep list silently dead.
func blockFor(root string, keep []string) string {
	// A keep entry is either a package, which ends in a slash once trimmed, or
	// a single class. Both are matched by prefix, with the class form pinned to
	// a dot so Foo does not also keep FooBar.
	var conditions strings.Builder
	for _, entry := range keep {
		trimmed := strings.TrimSuffix(entry, "/")
		if isClassEntry(root, trimmed) {
			fmt.Fprintf(&conditions, " &&\n                        !path.startsWith('%s.')", trimmed)
			continue
		}
		fmt.Fprintf(&conditions, " &&\n                        !path.startsWith('%s/')", trimmed)
	}

	kept := "nothing"
	if len(keep) > 0 {
		kept = strings.Join(keep, ", ")
	}

	return beginMarker + `
//
// Remove this block, or run the menu entry that added it, to go back to
// packaging team code in the APK.
//
// Kept in the APK anyway: ` + kept + `
android {
    sourceSets {
        main {
            java {
                exclude { details ->
                    def path = details.path
                    !details.directory &&
                        path.startsWith('` + TeamPackage + `/')` + conditions.String() + `
                }
            }
        }
    }
}
` + endMarker
}

// isClassEntry reports whether a keep entry names one class rather than a
// package.
//
// By looking, not by guessing at the capital letter. Convention is not
// reliable: real projects have packages named Hardware and Util alongside
// classes named velocityController, and getting this backwards emits a
// condition that matches nothing, so the entry is excluded despite being kept.
// Only a root with neither on disk falls back to the convention.
func isClassEntry(root, entry string) bool {
	if root != "" {
		if _, err := os.Stat(filepath.Join(root, SourceRoot, filepath.FromSlash(entry)+".java")); err == nil {
			return true
		}
		if info, err := os.Stat(filepath.Join(root, SourceRoot, filepath.FromSlash(entry))); err == nil {
			return !info.IsDir()
		}
	}

	base := entry
	if i := strings.LastIndex(entry, "/"); i >= 0 {
		base = entry[i+1:]
	}
	return base != "" && base[0] >= 'A' && base[0] <= 'Z'
}

// GradleFile is the module file the exclusion lives in.
func GradleFile(root string) string {
	return filepath.Join(root, Module, "build.gradle")
}

// KotlinDSL reports whether the module is configured with the Kotlin DSL.
//
// The block this writes is Groovy, and there is nowhere to put it in a
// build.gradle.kts. Saying so beats failing to read a file that was never
// going to be there.
func KotlinDSL(root string) bool {
	if _, err := os.Stat(GradleFile(root)); err == nil {
		return false
	}
	_, err := os.Stat(GradleFile(root) + ".kts")
	return err == nil
}

// Supported reports whether this project can be reloaded at all, and why not.
//
// Checked before anything is written. Setting up a project that cannot reload
// leaves team code out of the APK with nothing putting it back, which is a
// robot with no OpModes.
func Supported(root string) error {
	if KotlinDSL(root) {
		return fmt.Errorf("this project is configured with the Kotlin DSL " +
			"(TeamCode/build.gradle.kts), which Pusher Extreme cannot edit yet")
	}

	if _, err := os.Stat(GradleFile(root)); err != nil {
		return fmt.Errorf("no %s to add the exclusion to", filepath.Join(Module, "build.gradle"))
	}

	kotlin := 0
	filepath.Walk(filepath.Join(root, SourceRoot), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".kt") {
			kotlin++
		}
		return nil
	})
	if kotlin > 0 {
		return fmt.Errorf("this project has %d Kotlin source file(s), which reloading "+
			"compiles with javac and would silently drop", kotlin)
	}

	return nil
}

// Excluded reports whether team code is being kept out of the APK.
func Excluded(root string) bool {
	content, err := os.ReadFile(GradleFile(root))
	if err != nil {
		return false
	}
	return blockRe.Match(content)
}

// Kept reports the packages the block leaves in the APK.
//
// Read back from the file rather than from settings, because the file is what
// the build actually obeys.
func Kept(root string) []string {
	content, err := os.ReadFile(GradleFile(root))
	if err != nil {
		return nil
	}

	match := keptRe.FindSubmatch(content)
	if match == nil {
		return nil
	}

	line := strings.TrimSpace(string(match[1]))
	if line == "" || line == "nothing" {
		return nil
	}

	var out []string
	for _, pkg := range strings.Split(line, ",") {
		if pkg = strings.TrimSpace(pkg); pkg != "" {
			out = append(out, pkg)
		}
	}
	return out
}

// Exclude stops team code being packaged into the APK, except for the packages
// named in keep.
//
// Something a library reflects over has to stay in the APK. FtcDashboard scans
// the base APK itself with getPackageCodePath, so a @Config class that is
// reloaded is invisible to it however correctly it loads.
//
// keep is expanded to what those classes need to compile. Excluding a kept
// class's own dependencies leaves it in the source set with nothing to resolve
// against, and the build fails on the import.
func Exclude(root string, keep ...string) error {
	if err := Supported(root); err != nil {
		return err
	}

	keep = Closure(root, keep)

	path := GradleFile(root)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}

	// Replace rather than skip: the keep list may have changed, and a block
	// that no longer matches the settings is worse than no block.
	stripped := blockRe.ReplaceAllString(string(content), "\n")

	updated := strings.TrimRight(stripped, "\n") + "\n\n" + blockFor(root, keep) + "\n"

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// Include puts team code back in the APK.
func Include(root string) error {
	path := GradleFile(root)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	if !blockRe.Match(content) {
		return nil
	}

	updated := blockRe.ReplaceAllString(string(content), "\n")

	if err := os.WriteFile(path, []byte(strings.TrimRight(updated, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}
