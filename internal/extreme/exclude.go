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
	kept := "nothing"
	if len(keep) > 0 {
		kept = strings.Join(keep, ", ")
	}

	head := beginMarker + `
//
// Remove this block, or run the menu entry that added it, to go back to
// packaging team code in the APK.
//
// Kept in the APK anyway: ` + kept + "\n"

	if KotlinDSL(root) {
		return head + kotlinBlock(root, keep) + "\n" + endMarker
	}
	return head + groovyBlock(root, keep) + "\n" + endMarker
}

// conditions renders the keep list as the tail of the exclusion test.
//
// A keep entry is either a package, which ends in a slash once trimmed, or a
// single class. Both are matched by prefix, with the class form pinned to a dot
// so Foo does not also keep FooBar, and so it covers Foo.java and Foo.kt alike.
func conditions(root string, keep []string, quote, indent string) string {
	var out strings.Builder

	for _, entry := range keep {
		trimmed := strings.TrimSuffix(entry, "/")
		suffix := "/"
		if isClassEntry(root, trimmed) {
			suffix = "."
		}
		fmt.Fprintf(&out, " &&\n%s!path.startsWith(%s%s%s%s)",
			indent, quote, trimmed, suffix, quote)
	}

	return out.String()
}

func groovyBlock(root string, keep []string) string {
	return `android {
    sourceSets {
        main {
            java {
                exclude { details ->
                    def path = details.path
                    !details.directory &&
                        path.startsWith('` + TeamPackage + `/')` +
		conditions(root, keep, "'", "                        ") + `
                }
            }
        }
    }
}

` + kotlinTasks(`tasks.matching { it.name.startsWith('compile') && it.name.endsWith('Kotlin') }.configureEach {
    exclude { details ->
        def path = details.path
        !details.directory &&
            path.startsWith('`+TeamPackage+`/')`+
		conditions(root, keep, "'", "            ")+`
    }
}`)
}

// kotlinBlock is the same exclusion for a build.gradle.kts.
//
// The cast is not decoration. The Kotlin DSL hands out the newer
// com.android.build.api.dsl.AndroidSourceDirectorySet, which has no exclude at
// all; the object behind it implements the older interface, which does. Groovy
// never showed this because it dispatches dynamically.
func kotlinBlock(root string, keep []string) string {
	return `android {
    sourceSets {
        getByName("main") {
            (java as com.android.build.gradle.api.AndroidSourceDirectorySet).exclude { details ->
                val path = details.path
                !details.isDirectory &&
                    path.startsWith("` + TeamPackage + `/")` +
		conditions(root, keep, `"`, "                    ") + `
            }
        }
    }
}

` + kotlinTasks(`tasks.matching { it.name.startsWith("compile") && it.name.endsWith("Kotlin") }.configureEach {
    (this as org.gradle.api.tasks.util.PatternFilterable).exclude { details ->
        val path = details.path
        !details.isDirectory &&
            path.startsWith("`+TeamPackage+`/")`+
		conditions(root, keep, `"`, "            ")+`
    }
}`)
}

// kotlinTasks explains the second half of the exclusion, which is the half that
// is easy to leave out and impossible to notice missing.
//
// Excluding from the android source set does nothing to Kotlin. The Kotlin
// plugin compiles from its own source set, so on a project with .kt files the
// team's classes stayed in the APK while also being reloaded, and parent-first
// delegation means the APK copy is the one that runs. Measured on a real
// project: six of six Kotlin classes survived an exclusion that reported
// success.
//
// Matched by task name rather than by type, so this file never has to mention a
// plugin that a Java-only project does not apply. There, it matches nothing and
// costs nothing.
func kotlinTasks(block string) string {
	return `// The Kotlin plugin compiles from its own source set, which the exclusion
// above does not reach. Without this, .kt files stay in the APK and the reload
// is overruled by them.
` + block
}

// PowerPackage is the generated power monitor, which has to stay in the APK.
//
// Its entry point is a startup hook the robot controller calls when the web
// server comes up. Reloaded classes are scanned long after that has happened,
// so a monitor that is not in the APK is collected and never called: it would
// sit there looking installed and measure nothing.
const PowerPackage = TeamPackage + "/pusherpower"

// alwaysKept is what has to be in the APK whatever the keep list says, when it
// is in the project at all.
func alwaysKept(root string) []string {
	var out []string

	if _, err := os.Stat(filepath.Join(root, SourceRoot, filepath.FromSlash(PowerPackage))); err == nil {
		out = append(out, PowerPackage)
	}

	return out
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
		for _, ext := range []string{".java", ".kt"} {
			if _, err := os.Stat(filepath.Join(root, SourceRoot, filepath.FromSlash(entry)+ext)); err == nil {
				return true
			}
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

// GradleFile is the module file the exclusion lives in, in whichever dialect
// the project uses.
func GradleFile(root string) string {
	groovy := filepath.Join(root, Module, "build.gradle")
	if _, err := os.Stat(groovy); err == nil {
		return groovy
	}
	if _, err := os.Stat(groovy + ".kts"); err == nil {
		return groovy + ".kts"
	}
	return groovy
}

// KotlinDSL reports whether the module is configured with the Kotlin DSL, which
// needs a differently written block.
func KotlinDSL(root string) bool {
	return strings.HasSuffix(GradleFile(root), ".kts")
}

// Supported reports whether this project can be reloaded at all, and why not.
//
// Checked before anything is written. Setting up a project that cannot reload
// leaves team code out of the APK with nothing putting it back, which is a
// robot with no OpModes.
func Supported(root string) error {
	if _, err := os.Stat(GradleFile(root)); err != nil {
		return fmt.Errorf("no %s to add the exclusion to", filepath.Join(Module, "build.gradle"))
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

	keep = Closure(root, append(keep, alwaysKept(root)...))

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
