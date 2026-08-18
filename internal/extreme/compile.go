package extreme

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/hotreload"
)

// Module is the gradle module holding the team's own code.
const Module = "TeamCode"

// SourceRoot is where that module keeps it.
var SourceRoot = filepath.Join("TeamCode", "src", "main", "java")

// Project is a team's FTC project seen as something that can be reloaded.
type Project struct {
	Root    string
	Wrapper string
}

// Build is what a compile produced.
type Build struct {
	Jar     string
	Dex     string
	Sources int
	Classes int
	// Kept is how many classes stayed in the APK.
	Kept int
	// Kotlin is how many of the sources were Kotlin.
	Kotlin int
	// Bridged is how many classes were handed to a library that could not
	// otherwise see them.
	Bridged int
	// Registered is what the bridge puts into the dashboard, for the next
	// reload to take away again if it stops registering them.
	Registered []string
}

// FindProject locates the FTC project around the working directory.
func FindProject() (*Project, error) {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return nil, fmt.Errorf("not inside an FTC project: %w", err)
	}

	root := gradle.ProjectDir(wrapper)
	if _, err := os.Stat(filepath.Join(root, SourceRoot)); err != nil {
		return nil, fmt.Errorf("no %s in %s", SourceRoot, root)
	}

	return &Project{Root: root, Wrapper: wrapper}, nil
}

// Sourced is the team's code, split by what compiles it.
type Sourced struct {
	Java   []string
	Kotlin []string
}

// All is every source file, in the order a Kotlin compile wants them: the
// Kotlin first, with the Java behind it for reference rather than compilation.
func (s Sourced) All() []string {
	return append(append([]string{}, s.Kotlin...), s.Java...)
}

// Count is how many files the team wrote.
func (s Sourced) Count() int { return len(s.Java) + len(s.Kotlin) }

// Sources lists the team's source files.
func (p *Project) Sources() (Sourced, error) {
	root := filepath.Join(p.Root, SourceRoot)

	var out Sourced

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		switch {
		case strings.HasSuffix(path, ".java"):
			out.Java = append(out.Java, path)
		case strings.HasSuffix(path, ".kt"):
			out.Kotlin = append(out.Kotlin, path)
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	if out.Count() == 0 {
		return out, fmt.Errorf("no Java or Kotlin sources under %s", root)
	}
	return out, nil
}

// Compile turns the team's sources into a jar and a dex.
//
// Everything is compiled, not only what changed. A reload replaces the whole
// classloader, so a partial dex would leave the classes it did not contain
// unresolvable, and the SDK's re-registration abandons everything on the first
// failure rather than skipping one class.
func Compile(p *Project, cp Classpath, work string, keep, previous []string) (Build, error) {
	var out Build

	tc, err := hotreload.FindToolchain()
	if err != nil {
		return out, err
	}
	defer tc.Cleanup()

	sources, err := p.Sources()
	if err != nil {
		return out, err
	}
	out.Sources = sources.Count()
	out.Kotlin = len(sources.Kotlin)

	// The bridge is compiled alongside, so it holds real class references
	// rather than names looked up at runtime, which is the whole reason it can
	// hand reloaded classes to something that cannot see them.
	configs := ConfigClasses(p.Root, keep)
	bridge, err := GenerateBridge(work, configs, previous, cp)
	if err != nil {
		return out, err
	}
	if bridge != "" {
		sources.Java = append(sources.Java, bridge)
		out.Bridged = len(configs)
		out.Registered = RegisteredNames(configs)
	}

	classes := filepath.Join(work, "classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		return out, err
	}

	// Kotlin first, because the Kotlin compiler reads the Java sources for
	// their signatures without compiling them, while javac cannot read Kotlin
	// at all and needs the classes it produced. Either half may reference the
	// other, and this is the order that allows it.
	if len(sources.Kotlin) > 0 {
		if err := compileKotlin(tc, cp, sources, classes, work); err != nil {
			return out, err
		}
	}

	if len(sources.Java) > 0 {
		if err := compileJava(tc, cp, sources.Java, classes, work); err != nil {
			return out, err
		}
	}

	compiled, err := classFiles(classes)
	if err != nil {
		return out, err
	}

	// Everything is compiled, because the reloaded classes reference the kept
	// ones, but only the reloaded ones are packaged. A class in both would
	// resolve from the APK anyway, so shipping it twice is dead weight that
	// reads like a bug when the APK copy is the one that runs.
	compiled, out.Kept = split(compiled, keep)
	out.Classes = len(compiled)

	if len(compiled) == 0 {
		return out, fmt.Errorf("everything is kept in the APK, so there is nothing to reload")
	}

	out.Jar = filepath.Join(work, "teamcode.jar")
	if err := writeJar(out.Jar, classes, compiled); err != nil {
		return out, err
	}

	dexDir := filepath.Join(work, "dex")
	if err := os.MkdirAll(dexDir, 0o755); err != nil {
		return out, err
	}

	// d8 takes the jar rather than the loose classes: one argument instead of
	// hundreds, and it is the same input the robot will be given.
	d8 := exec.Command(tc.D8, "--min-api", "24", "--output", dexDir, out.Jar)
	if result, err := d8.CombinedOutput(); err != nil {
		return out, fmt.Errorf("dexing %s failed:\n%s", Module, lastLines(string(result), 25))
	}

	out.Dex = filepath.Join(dexDir, "classes.dex")
	if _, err := os.Stat(out.Dex); err != nil {
		return out, fmt.Errorf("d8 produced no classes.dex")
	}

	return out, nil
}

// compileJava builds the team's Java, and the bridge with it.
//
// The output directory goes on the classpath ahead of everything else so the
// Java half can see whatever Kotlin just produced. On a project with no Kotlin
// it is simply empty, and nothing about this changes.
func compileJava(tc hotreload.Toolchain, cp Classpath, sources []string, classes, work string) error {
	// The file list goes in an argument file. A hundred and twenty paths
	// exceeds what a command line takes on some platforms, and javac has
	// supported @files for exactly this since forever.
	list := filepath.Join(work, "sources.txt")
	if err := os.WriteFile(list, []byte(strings.Join(sources, "\n")), 0o644); err != nil {
		return err
	}

	// Java 8, matching what the FTC project targets and what the hub runs. A
	// modern javac also refuses -bootclasspath unless the target is old enough
	// to need one, which is exactly the case here.
	args := append([]string{
		"-source", "8", "-target", "8",
		"-nowarn",
		"-encoding", "UTF-8",
		"-d", classes,
	}, cp.Args(classes)...)
	args = append(args, "@"+list)

	javac := exec.Command(tc.Javac, args...)
	if result, err := javac.CombinedOutput(); err != nil {
		return fmt.Errorf("compiling %s failed:\n%s", Module, lastLines(string(result), 25))
	}

	return nil
}

// compileKotlin builds the team's Kotlin into the same output directory.
//
// The compiler ships as a jar rather than a binary, so it is launched the way
// Gradle launches it, and it comes out of the Gradle cache at the version the
// project itself builds with.
func compileKotlin(tc hotreload.Toolchain, cp Classpath, sources Sourced, classes, work string) error {
	kotlin, err := FindKotlin(KotlinVersion(cp))
	if err != nil {
		return err
	}

	// Same argument-file reasoning as javac, and the same syntax.
	list := filepath.Join(work, "kotlin-sources.txt")
	if err := os.WriteFile(list, []byte(strings.Join(sources.All(), "\n")), 0o644); err != nil {
		return err
	}

	// -no-stdlib because the project's own copy is already on the classpath,
	// and it is the copy the APK ships. Letting the compiler add its own would
	// put a second, possibly different standard library in front of it.
	args := []string{
		"-cp", strings.Join(kotlin.Jars, string(os.PathListSeparator)),
		"org.jetbrains.kotlin.cli.jvm.K2JVMCompiler",
		"-no-stdlib",
		"-nowarn",
		"-jvm-target", "1.8",
		"-classpath", cp.Flat(),
		"-d", classes,
		"@" + list,
	}

	kotlinc := exec.Command(tc.Java(), args...)
	if result, err := kotlinc.CombinedOutput(); err != nil {
		return fmt.Errorf("compiling the Kotlin in %s failed:\n%s",
			Module, lastLines(string(result), 25))
	}

	return nil
}

// classFiles lists what javac produced, relative to the output directory.
func classFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".class") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("nothing compiled to a class")
	}
	return files, nil
}

// writeJar packs the compiled classes under their package paths, which is where
// the SDK reads class names from.
func writeJar(jarPath, root string, entries []string) error {
	out, err := os.Create(jarPath)
	if err != nil {
		return err
	}
	defer out.Close()

	archive := zip.NewWriter(out)

	for _, entry := range entries {
		source, err := os.Open(filepath.Join(root, filepath.FromSlash(entry)))
		if err != nil {
			return err
		}

		w, err := archive.Create(entry)
		if err != nil {
			source.Close()
			return err
		}
		if _, err := io.Copy(w, source); err != nil {
			source.Close()
			return err
		}
		source.Close()
	}

	return archive.Close()
}

// gradleEnv gives Gradle a JDK when the environment has not.
func gradleEnv() []string {
	env := os.Environ()
	if os.Getenv("JAVA_HOME") != "" {
		return env
	}

	tc, err := hotreload.FindToolchain()
	if err != nil {
		return env
	}
	defer tc.Cleanup()

	// javac lives in <home>/bin, so the home is two levels up.
	home := filepath.Dir(filepath.Dir(tc.Javac))
	return append(env, "JAVA_HOME="+home)
}

// classNames lists the classes in a jar, the same way the SDK does.
func classNames(jarPath string) ([]string, error) {
	archive, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	var names []string
	for _, entry := range archive.File {
		if strings.HasSuffix(entry.Name, ".class") {
			names = append(names, strings.TrimSuffix(entry.Name, ".class"))
		}
	}
	return names, nil
}

// split separates the classes that get reloaded from the ones staying in the
// APK.
func split(entries []string, keep []string) (reload []string, kept int) {
	for _, entry := range entries {
		if inAny(entry, keep) {
			kept++
			continue
		}
		reload = append(reload, entry)
	}
	return reload, kept
}

// inAny reports whether a class entry is covered by a keep entry, which names
// either a package or a single class. A class keeps its inner classes with it,
// since they are compiled from the same file and share its identity.
func inAny(entry string, keep []string) bool {
	entry = strings.TrimSuffix(entry, ".class")

	for _, item := range keep {
		item = strings.TrimSuffix(item, "/")

		// Both forms are checked rather than one of them guessed at: the entry
		// is that class, an inner class of it, or something under it as a
		// package. No naming convention separates the two reliably.
		if entry == item || strings.HasPrefix(entry, item+"$") ||
			strings.HasPrefix(entry, item+"/") {
			return true
		}
	}
	return false
}
