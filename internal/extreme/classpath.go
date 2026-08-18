// Package extreme compiles a team's own OpModes outside the APK, so they can be
// reloaded onto a running robot instead of installed.
//
// The mechanism it feeds is proven: see internal/hotreload. What this adds is
// the team's real code, with its real dependencies, in place of one synthetic
// OpMode.
package extreme

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// initScript asks Gradle what the module compiles against.
//
// An init script rather than a change to the project: this has to work on a
// repository pusher has never touched, and leave nothing behind if it fails.
//
// The android-classes-jar view matters. The raw artifacts include AARs, which
// javac cannot read; asking for that attribute makes AGP hand over the jars it
// has already extracted from them.
const initScript = `import org.gradle.api.attributes.Attribute

allprojects { p ->
    p.tasks.register("pusherClasspath") {
        doLast {
            println "PUSHER_CP_BEGIN"
            def cfg = p.configurations.findByName("debugCompileClasspath")
            if (cfg != null) {
                def view = cfg.incoming.artifactView { av ->
                    av.lenient = true
                    av.attributes { attrs ->
                        attrs.attribute(Attribute.of("artifactType", String), "android-classes-jar")
                    }
                }
                view.files.each { f -> println "CP " + f.absolutePath }
            }
            if (p.extensions.findByName("android") != null) {
                p.extensions.android.bootClasspath.each { f -> println "BOOT " + f.absolutePath }
            }
            println "PUSHER_CP_END"
        }
    }
}
`

// Classpath is what the team's code compiles against.
type Classpath struct {
	// Compile is the dependency jars.
	Compile []string
	// Boot is android.jar and friends, which javac needs as the platform
	// rather than as ordinary dependencies.
	Boot []string
}

// Args renders the classpath for javac. Anything in first goes ahead of the
// project's own jars, which is how the Kotlin output becomes visible to the
// Java half of a mixed compile.
func (c Classpath) Args(first ...string) []string {
	var args []string
	if len(c.Boot) > 0 {
		args = append(args, "-bootclasspath", strings.Join(c.Boot, string(os.PathListSeparator)))
	}
	if entries := append(append([]string{}, first...), c.Compile...); len(entries) > 0 {
		args = append(args, "-classpath", strings.Join(entries, string(os.PathListSeparator)))
	}
	return args
}

// Flat is every jar as one classpath, platform included.
//
// For the Kotlin compiler, which has no notion of a boot classpath: android.jar
// is just another entry to it, and leaving it out means every android.* type in
// the team's code goes unresolved.
func (c Classpath) Flat() string {
	return strings.Join(append(append([]string{}, c.Compile...), c.Boot...),
		string(os.PathListSeparator))
}

// ResolveClasspath asks Gradle what the module compiles against.
//
// Slow the first time and cached by Gradle after that, so it runs per build
// rather than per reload.
func ResolveClasspath(wrapper, module string) (Classpath, error) {
	var out Classpath

	script, err := os.CreateTemp("", "pusher-classpath-*.gradle")
	if err != nil {
		return out, err
	}
	defer os.Remove(script.Name())

	if _, err := script.WriteString(initScript); err != nil {
		script.Close()
		return out, err
	}
	if err := script.Close(); err != nil {
		return out, err
	}

	cmd := exec.Command(wrapper, "-I", script.Name(), ":"+module+":pusherClasspath")
	cmd.Dir = filepath.Dir(wrapper)
	cmd.Env = gradleEnv()

	result, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gradle could not report the classpath: %w\n%s",
			err, lastLines(string(result), 12))
	}

	out = parseClasspath(string(result))
	if len(out.Compile) == 0 {
		return out, fmt.Errorf("gradle reported no classpath for :%s", module)
	}

	return out, nil
}

// parseClasspath reads the marked block, so Gradle's own chatter around it is
// ignored.
func parseClasspath(output string) Classpath {
	var out Classpath

	inside := false
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case line == "PUSHER_CP_BEGIN":
			inside = true
		case line == "PUSHER_CP_END":
			inside = false
		case !inside:
		case strings.HasPrefix(line, "CP "):
			out.Compile = append(out.Compile, strings.TrimPrefix(line, "CP "))
		case strings.HasPrefix(line, "BOOT "):
			out.Boot = append(out.Boot, strings.TrimPrefix(line, "BOOT "))
		}
	}

	return out
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
