package extreme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Kotlin is a compiler pusher can run, and the jars it needs to run at all.
type Kotlin struct {
	// Version is the Kotlin the project itself compiles with.
	Version string

	// Jars is the compiler's own classpath, not the project's.
	Jars []string
}

// stdlibRe reads the Kotlin version off the standard library on the classpath.
//
// The version is not a preference. Classes compiled by one Kotlin and run
// against another's standard library are a support burden nobody asked for, and
// the project has already told us which one it uses by shipping it in the APK.
var stdlibRe = regexp.MustCompile(`kotlin-stdlib-(\d+\.\d+[\w.\-]*)\.jar$`)

// KotlinVersion is the Kotlin the project compiles with, or empty for a project
// that does not use Kotlin at all.
func KotlinVersion(cp Classpath) string {
	for _, jar := range cp.Compile {
		if m := stdlibRe.FindStringSubmatch(filepath.Base(jar)); m != nil {
			return m[1]
		}
	}
	return ""
}

// The compiler is a Kotlin program, so it needs a Kotlin runtime of its own
// before it can compile anything. Each of these was added because the compiler
// died without it, in this order, and none of them can be dropped.
//
// The pinned ones move with the compiler. The loose ones are its own plumbing,
// where the newest cached copy does, because nothing about the code being
// compiled depends on which one it is.
var (
	pinnedToKotlin = []string{
		"org.jetbrains.kotlin/kotlin-compiler-embeddable",
		"org.jetbrains.kotlin/kotlin-stdlib",
		"org.jetbrains.kotlin/kotlin-reflect",
		"org.jetbrains.kotlin/kotlin-script-runtime",
		"org.jetbrains.kotlin/kotlin-daemon-embeddable",
	}

	anyVersion = []string{
		"org.jetbrains.intellij.deps/trove4j",
		"org.jetbrains.kotlinx/kotlinx-coroutines-core-jvm",
		"org.jetbrains/annotations",
	}
)

// FindKotlin locates a compiler matching what the project builds with.
//
// Everything comes out of the Gradle cache, because the project put it there:
// building the APK downloads the Kotlin plugin, and the plugin brings the
// compiler with it. Nothing is downloaded here, and a project that has never
// been built is told to build.
func FindKotlin(version string) (Kotlin, error) {
	out := Kotlin{Version: version}

	if version == "" {
		return out, fmt.Errorf("no kotlin-stdlib on the classpath, so there is no Kotlin version to match")
	}

	caches := gradleCache()
	if caches == "" {
		return out, fmt.Errorf("no Gradle cache to take the Kotlin compiler from")
	}

	for _, module := range pinnedToKotlin {
		jar := findJar(caches, module, version)
		if jar == "" {
			return out, fmt.Errorf("the Gradle cache has no %s %s.\n"+
				"    Build the project once with `./gradlew assembleDebug`, which downloads it,\n"+
				"    then try again",
				filepath.Base(module), version)
		}
		out.Jars = append(out.Jars, jar)
	}

	for _, module := range anyVersion {
		jar := findJar(caches, module, "")
		if jar == "" {
			return out, fmt.Errorf("the Gradle cache has no %s, which the Kotlin compiler needs to run",
				filepath.Base(module))
		}
		out.Jars = append(out.Jars, jar)
	}

	return out, nil
}

func gradleCache() string {
	root := os.Getenv("GRADLE_USER_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(home, ".gradle")
	}

	modules := filepath.Join(root, "caches", "modules-2", "files-2.1")
	if _, err := os.Stat(modules); err != nil {
		return ""
	}
	return modules
}

// findJar digs a jar out of the Gradle cache, whose layout is
// <group>/<artifact>/<version>/<hash>/<artifact>-<version>.jar. An empty
// version takes the newest one cached.
func findJar(caches, module, version string) string {
	group, artifact, ok := strings.Cut(module, "/")
	if !ok {
		return ""
	}

	base := filepath.Join(caches, group, artifact)

	versions := []string{version}
	if version == "" {
		versions = newestFirst(base)
	}

	for _, v := range versions {
		hashes, err := os.ReadDir(filepath.Join(base, v))
		if err != nil {
			continue
		}

		for _, hash := range hashes {
			jar := filepath.Join(base, v, hash.Name(), artifact+"-"+v+".jar")
			if _, err := os.Stat(jar); err == nil {
				return jar
			}
		}
	}

	return ""
}

func newestFirst(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions
}

// compareVersions orders version strings by their numeric parts, so 1.10 comes
// after 1.9 rather than before it the way a string sort would have it.
func compareVersions(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")

	for i := 0; i < len(left) || i < len(right); i++ {
		var x, y int
		if i < len(left) {
			x = leadingNumber(left[i])
		}
		if i < len(right) {
			y = leadingNumber(right[i])
		}

		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}

	return strings.Compare(a, b)
}

func leadingNumber(part string) int {
	end := 0
	for end < len(part) && part[end] >= '0' && part[end] <= '9' {
		end++
	}

	n, _ := strconv.Atoi(part[:end])
	return n
}
