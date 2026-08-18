package extreme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A reloaded class lives below the APK in the classloader chain, so the APK can
// see nothing of it. Team code calling into a library is fine. A library
// reaching back into team code is not, and it fails at runtime rather than at
// compile time.
//
// Most FTC libraries never do it: pedro, Panels, EasyOpenCV and blob all go
// through the SDK, which does see reloaded classes. FtcDashboard is the
// exception. It scans the base APK itself with getPackageCodePath, so a @Config
// class that is reloaded is invisible to it however correctly it loads.
//
// Leaving those classes in the APK would fix it, but that is not a default
// worth having. In a real project @Config turned out to be on 45 of 120 files
// including the OpModes themselves, so keeping them all would make most of the
// project unreloadable, which is worse than what it fixes.
//
// So they are bridged instead: see bridge.go. What is found here is handed to
// the dashboard by generated code that runs inside the reload, which gets the
// tuning back without keeping anything in the APK. Keeping a package in the APK
// stays available for a library nothing here knows how to bridge.

// reflectedBy are annotations a library reads by scanning rather than by being
// handed the class.
var reflectedBy = map[string]string{
	"@Config": "FtcDashboard scans the APK for these, so a reloaded one will not appear",
}

var annotationRe = regexp.MustCompile(`(?m)^\s*@(\w+)`)

// declarationRe names what a Kotlin annotation was attached to.
//
// Kotlin does not require the file to be named after what is in it, so the
// class FtcDashboard has to be handed cannot be read off the filename the way
// it can in Java. Config.kt holding `object Tuning` is `Tuning`, and bridging
// `Config` would register a class that does not exist.
var declarationRe = regexp.MustCompile(`(?m)^\s*(?:[\w@]+\s+)*?(?:object|class|interface)\s+(\w+)`)

// Reflected is a class something in the APK reads by scanning.
type Reflected struct {
	Package string
	File    string
	// Class is the name the JVM knows it by, which in Kotlin need not be the
	// file's.
	Class string
	Why   string
}

// Reflection is what a project would lose by reloading.
type Reflection struct {
	Classes  []Reflected
	Packages []string
	Why      string
}

// Any reports whether there is anything to say.
func (r Reflection) Any() bool { return len(r.Classes) > 0 }

// Summary is the one line for a menu.
func (r Reflection) Summary() string {
	if !r.Any() {
		return ""
	}
	return fmt.Sprintf("%d classes use @Config: they are registered with "+
		"FtcDashboard from inside the reload, since it cannot find them itself", len(r.Classes))
}

// FindReflected looks for team code something in the APK reads by scanning.
//
// Source rather than bytecode: this runs before anything is compiled, and the
// answer decides what gets compiled.
func FindReflected(root string) Reflection {
	base := filepath.Join(root, SourceRoot)

	var out Reflection
	packages := map[string]bool{}

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isSource(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		for _, match := range annotationRe.FindAllStringSubmatchIndex(string(content), -1) {
			why, reflected := reflectedBy["@"+string(content[match[2]:match[3]])]
			if !reflected {
				continue
			}

			rel, err := filepath.Rel(base, path)
			if err != nil {
				continue
			}

			pkg := filepath.ToSlash(filepath.Dir(rel))
			packages[pkg] = true
			out.Classes = append(out.Classes, Reflected{
				Package: pkg,
				File:    filepath.Base(path),
				Class:   className(path, string(content), match[1]),
				Why:     why,
			})
			break
		}
		return nil
	})

	for pkg := range packages {
		out.Packages = append(out.Packages, pkg)
	}
	sort.Strings(out.Packages)
	sort.Slice(out.Classes, func(a, b int) bool { return out.Classes[a].File < out.Classes[b].File })

	if out.Any() {
		out.Why = reflectedBy["@Config"]
	}

	return out
}

// isSource reports whether a file is team code in either language.
func isSource(path string) bool {
	return strings.HasSuffix(path, ".java") || strings.HasSuffix(path, ".kt")
}

// className is what the JVM calls the annotated declaration.
//
// Java takes it from the filename, which the language guarantees for a public
// class. Kotlin guarantees nothing of the sort, so the declaration after the
// annotation is read instead, and the filename is only the fallback for a file
// this cannot make sense of.
func className(path, content string, after int) string {
	base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".java"), ".kt")

	if !strings.HasSuffix(path, ".kt") || after >= len(content) {
		return base
	}

	if m := declarationRe.FindStringSubmatch(content[after:]); m != nil {
		return m[1]
	}
	return base
}

// driverAnnotations mark a class the SDK instantiates as a hardware device.
//
// These cannot be reloaded, and unlike anything else here that is not a
// preference. Every reload builds a new classloader, so a reloaded driver is a
// different class each time while the device instance in the hardware map was
// built under an earlier one. hardwareMap.get then finds nothing assignable to
// what the OpMode asked for, and the robot reports that it cannot find its
// hardware.
var driverAnnotations = []string{
	"@DeviceProperties", "@I2cDeviceType", "@MotorType", "@ServoType",
	"@AnalogSensorType", "@DigitalIoDeviceType", "@I2cSensor",
}

// FindDrivers returns the team files defining hardware device drivers, as paths
// without the .java extension.
//
// File granularity rather than package: a driver usually sits among ordinary
// code that should still reload.
func FindDrivers(root string) []string {
	base := filepath.Join(root, SourceRoot)

	var out []string

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isSource(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		for _, annotation := range driverAnnotations {
			if !strings.Contains(string(content), annotation) {
				continue
			}

			rel, err := filepath.Rel(base, path)
			if err != nil {
				return nil
			}

			entry := filepath.ToSlash(rel)
			out = append(out, strings.TrimSuffix(strings.TrimSuffix(entry, ".java"), ".kt"))
			return nil
		}
		return nil
	})

	sort.Strings(out)
	return out
}
