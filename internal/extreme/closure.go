package extreme

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/andreibanu/pusher/internal/javasrc"
)

// Keeping a class in the APK keeps it in the source set, and excluding the rest
// of team code takes its dependencies out of that same source set. javac then
// cannot resolve them:
//
//	error: cannot find symbol
//	import org.firstinspires.ftc.teamcode.foo.MyTimer;
//
// The classes with no annotation appear to compile only because they are not
// compiled at all.
//
// So a keep list is not a list, it is a starting point. Everything a kept class
// needs has to be kept with it, or the build cannot succeed.

var (
	// No trailing semicolon in the pattern: Kotlin does not write one, and a
	// wildcard never matched with it there, so `import x.y.*;` quietly kept
	// nothing at all on the Java side too.
	importRe = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?((?:\w+\.)*(?:\w+|\*))`)
	wordRe   = regexp.MustCompile(`\b\w+\b`)

	// A team class can also be named in full where it is used, with no import
	// to follow. Rarer than an import, and just as fatal to the build.
	qualifiedRe = regexp.MustCompile(regexp.QuoteMeta(strings.ReplaceAll(TeamPackage, "/", ".")) + `(?:\.\w+)+`)
)

// teamPrefix is the team package in java form.
var teamPrefix = strings.ReplaceAll(TeamPackage, "/", ".") + "."

// Closure expands a keep list to everything the kept classes need to compile.
//
// Entries come back in the form they went in: a path under the team package
// with no extension.
func Closure(root string, keep []string) []string {
	index := indexSources(root)
	if len(index.files) == 0 {
		return keep
	}

	kept := map[string]bool{}
	for _, entry := range keep {
		trimmed := strings.TrimSuffix(entry, "/")
		if isClassEntry(root, trimmed) {
			if index.files[trimmed] {
				kept[trimmed] = true
			}
			continue
		}
		for _, file := range index.packages[trimmed] {
			kept[file] = true
		}
	}

	// Fixpoint: what a kept class needs may itself need something else.
	for changed := true; changed; {
		changed = false
		for file := range kept {
			for _, needed := range index.needs(root, file) {
				if !kept[needed] {
					kept[needed] = true
					changed = true
				}
			}
		}
	}

	out := make([]string, 0, len(kept))
	for file := range kept {
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

// sources is every team source file, addressable the two ways a reference can
// name one.
type sources struct {
	// files holds each path under the team package, without its extension.
	files map[string]bool
	// ext remembers which language each one was written in, because the path
	// alone no longer says.
	ext map[string]string
	// packages maps a package path to the files in it.
	packages map[string][]string
	// byName maps a fully qualified class name to its path.
	byName map[string]string
}

func indexSources(root string) sources {
	out := sources{
		files:    map[string]bool{},
		ext:      map[string]string{},
		packages: map[string][]string{},
		byName:   map[string]string{},
	}

	base := filepath.Join(root, SourceRoot)

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".java" && ext != ".kt" {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}

		entry := strings.TrimSuffix(filepath.ToSlash(rel), ext)
		if !strings.HasPrefix(entry, TeamPackage+"/") {
			return nil
		}

		pkg := filepath.ToSlash(filepath.Dir(entry))

		out.files[entry] = true
		out.ext[entry] = ext
		out.packages[pkg] = append(out.packages[pkg], entry)
		out.byName[strings.ReplaceAll(entry, "/", ".")] = entry

		return nil
	})

	return out
}

// needs is every team file the given one refers to.
func (s sources) needs(root, file string) []string {
	blob, err := os.ReadFile(filepath.Join(root, SourceRoot, filepath.FromSlash(file)+s.ext[file]))
	if err != nil {
		return nil
	}
	// Comments and string literals are blanked first. A driver's javadoc and its
	// @DeviceProperties name mention class names constantly, and matching those
	// keeps half the project in the APK where none of it can reload.
	code := javasrc.Mask(string(blob))

	var out []string

	for _, match := range importRe.FindAllStringSubmatch(code, -1) {
		name := match[1]
		if !strings.HasPrefix(name, teamPrefix) {
			continue
		}

		// A wildcard needs the whole package. A static import names a member,
		// so its owning class is one segment up.
		if strings.HasSuffix(name, ".*") {
			pkg := strings.ReplaceAll(strings.TrimSuffix(name, ".*"), ".", "/")
			out = append(out, s.packages[pkg]...)
			continue
		}

		if path, found := s.byName[name]; found {
			out = append(out, path)
			continue
		}
		if i := strings.LastIndex(name, "."); i > 0 {
			if path, found := s.byName[name[:i]]; found {
				out = append(out, path)
			}
		}
	}

	// A name written out in full at the point of use, with no import.
	for _, name := range qualifiedRe.FindAllString(code, -1) {
		for candidate := name; strings.Contains(candidate, "."); {
			if path, found := s.byName[candidate]; found {
				out = append(out, path)
				break
			}
			candidate = candidate[:strings.LastIndex(candidate, ".")]
		}
	}

	// A class in the same package is referred to without an import, so there is
	// nothing to follow. Its name appearing in the file is the only signal
	// there is, which keeps more than strictly necessary rather than less.
	pkg := filepath.ToSlash(filepath.Dir(file))
	siblings := s.packages[pkg]
	if len(siblings) < 2 {
		return out
	}

	used := map[string]bool{}
	for _, word := range wordRe.FindAllString(code, -1) {
		used[word] = true
	}

	for _, sibling := range siblings {
		if sibling == file {
			continue
		}
		if used[filepath.Base(sibling)] {
			out = append(out, sibling)
		}
	}

	return out
}
