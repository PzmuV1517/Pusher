package extreme

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Gradle's own chatter surrounds the answer, so the block markers are what make
// the output readable rather than the line shapes.
func TestClasspathIsReadFromTheMarkedBlock(t *testing.T) {
	output := `Downloading gradle...
> Task :TeamCode:pusherClasspath
PUSHER_CP_BEGIN
CP /a/one.jar
CP /a/two.jar
BOOT /sdk/android.jar
PUSHER_CP_END

BUILD SUCCESSFUL in 763ms
CP /this/is/not/in/the/block.jar`

	cp := parseClasspath(output)

	if len(cp.Compile) != 2 {
		t.Fatalf("got %v", cp.Compile)
	}
	if len(cp.Boot) != 1 {
		t.Fatalf("got %v", cp.Boot)
	}
	for _, entry := range cp.Compile {
		if strings.Contains(entry, "not/in/the/block") {
			t.Error("a line outside the block was read")
		}
	}
}

// javac needs the platform as a boot classpath, not as an ordinary dependency,
// or Android classes and the JDK's own resolve against each other.
func TestArgsSeparateThePlatformFromTheDependencies(t *testing.T) {
	cp := Classpath{
		Compile: []string{"/a/one.jar", "/a/two.jar"},
		Boot:    []string{"/sdk/android.jar"},
	}

	args := cp.Args()

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-bootclasspath /sdk/android.jar") {
		t.Errorf("got %v", args)
	}
	if !strings.Contains(joined, "-classpath /a/one.jar") {
		t.Errorf("got %v", args)
	}

	// Nothing to say when there is nothing to say.
	if got := (Classpath{}).Args(); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestAnEmptyClasspathIsAnError(t *testing.T) {
	if cp := parseClasspath("BUILD SUCCESSFUL"); len(cp.Compile) != 0 {
		t.Errorf("got %v", cp.Compile)
	}
}

// The exclusion is marked rather than backed up: slim already keeps a
// .pusher-bak of the same file, and two features sharing one backup means
// undoing either undoes both.
func TestExclusionIsAddedAndRemovedExactly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}

	original := "android {\n    namespace = 'org.firstinspires.ftc.teamcode'\n}\n"
	if err := os.WriteFile(GradleFile(root), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if Excluded(root) {
		t.Fatal("a fresh project reports as excluded")
	}

	if err := Exclude(root); err != nil {
		t.Fatal(err)
	}
	if !Excluded(root) {
		t.Fatal("the exclusion was not detected after adding it")
	}

	after, _ := os.ReadFile(GradleFile(root))
	if !strings.Contains(string(after), TeamPackage) {
		t.Error("the excluded package is not named in the block")
	}
	// The instruction for getting back has to be in the file, not only in a menu.
	if !strings.Contains(string(after), "Remove this block") {
		t.Error("the block does not say how to undo it")
	}

	// Adding twice must not stack.
	if err := Exclude(root); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(mustRead(t, GradleFile(root))), beginMarker); got != 1 {
		t.Errorf("the block appears %d times", got)
	}

	if err := Include(root); err != nil {
		t.Fatal(err)
	}
	if Excluded(root) {
		t.Error("still excluded after going back")
	}

	// And the file has to come back exactly as it was.
	if got := string(mustRead(t, GradleFile(root))); got != original {
		t.Errorf("the file did not come back unchanged:\n%q", got)
	}
}

// Removing what was never added must not damage the file.
func TestIncludeOnAnUntouchedProjectDoesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}

	original := "dependencies {\n}\n"
	if err := os.WriteFile(GradleFile(root), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Include(root); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, GradleFile(root))); got != original {
		t.Errorf("got %q", got)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// @Config turned out to be on 45 of 120 files in a real project, including the
// OpModes themselves, so keeping everything it touches in the APK would leave
// most of the project unreloadable. They are bridged instead, and what is found
// here is what gets bridged.
func TestReflectedClassesAreFoundForBridging(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, SourceRoot, "org/firstinspires/ftc/teamcode/OpMode")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	tuned := "package x;\n@Config\n@TeleOp(name = \"a\")\npublic class Tuned {}\n"
	plain := "package x;\n@TeleOp(name = \"b\")\npublic class Plain {}\n"

	if err := os.WriteFile(filepath.Join(src, "Tuned.java"), []byte(tuned), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Plain.java"), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}

	found := FindReflected(root)

	if len(found.Classes) != 1 {
		t.Fatalf("got %d, want only the @Config one", len(found.Classes))
	}
	if found.Classes[0].File != "Tuned.java" {
		t.Errorf("got %q", found.Classes[0].File)
	}

	// The summary says what happens rather than what is lost, because what
	// happens is that it keeps working.
	if !strings.Contains(found.Summary(), "FtcDashboard") {
		t.Errorf("got %q", found.Summary())
	}

	// And what is found is exactly what the bridge is given.
	bridged := ConfigClasses(root, nil)
	if len(bridged) != len(found.Classes) {
		t.Errorf("found %d classes but bridged %d", len(found.Classes), len(bridged))
	}
}

// Setting up must not quietly keep anything back.
func TestSetupKeepsNothingByDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GradleFile(root), []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Exclude(root); err != nil {
		t.Fatal(err)
	}

	if kept := Kept(root); len(kept) != 0 {
		t.Errorf("got %v, want nothing kept", kept)
	}
}

// A keep list still has to work for anyone who wants the tuning back.
func TestAKeptPackageIsNamedInTheBlockAndReadBack(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GradleFile(root), []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := "org/firstinspires/ftc/teamcode/tuning"
	if err := Exclude(root, pkg); err != nil {
		t.Fatal(err)
	}

	kept := Kept(root)
	if len(kept) != 1 || kept[0] != pkg {
		t.Fatalf("got %v", kept)
	}

	content := string(mustRead(t, GradleFile(root)))
	if !strings.Contains(content, "!path.startsWith('"+pkg+"/')") {
		t.Errorf("the kept package is not excluded from the exclusion:\n%s", content)
	}

	// Setting up again with a different list must replace, not stack.
	if err := Exclude(root); err != nil {
		t.Fatal(err)
	}
	if kept := Kept(root); len(kept) != 0 {
		t.Errorf("the old keep list survived: %v", kept)
	}
	if got := strings.Count(string(mustRead(t, GradleFile(root))), beginMarker); got != 1 {
		t.Errorf("the block appears %d times", got)
	}
}

// The split decides what is packaged, so it has to match the gradle exclusion.
func TestKeptClassesAreNotPackagedForReload(t *testing.T) {
	entries := []string{
		"org/firstinspires/ftc/teamcode/OpMode/Auto.class",
		"org/firstinspires/ftc/teamcode/tuning/Constants.class",
		"org/firstinspires/ftc/teamcode/tuning/Constants$Inner.class",
	}

	reload, kept := split(entries, []string{"org/firstinspires/ftc/teamcode/tuning"})

	if kept != 2 {
		t.Errorf("got %d kept, want the constants class and its inner one", kept)
	}
	if len(reload) != 1 || !strings.HasSuffix(reload[0], "Auto.class") {
		t.Errorf("got %v", reload)
	}
}

// A generated file must never mention a library the project does not have, or
// the whole compile fails on a project that simply does not use it.
func TestTheBridgeOnlyNamesLibrariesThatArePresent(t *testing.T) {
	work := t.TempDir()

	path, err := GenerateBridge(work, []string{"a.b.Tuned"}, nil, Classpath{})
	if err != nil {
		t.Fatal(err)
	}

	body := string(mustRead(t, path))
	if strings.Contains(body, "acmerobotics") {
		t.Errorf("dashboard is referenced without being on the classpath:\n%s", body)
	}
	// The hook still has to exist, since it is what runs inside the reload.
	if !strings.Contains(body, "@OpModeRegistrar") {
		t.Error("the registrar hook is missing")
	}
}

// With the library present the classes have to be named directly, because a
// reference compiled in is the thing the APK cannot produce for itself.
func TestTheBridgeNamesEachConfigClass(t *testing.T) {
	work := t.TempDir()
	cp := Classpath{Compile: []string{fakeJar(t, work, dashboardMarker)}}

	path, err := GenerateBridge(work, []string{"a.b.One", "a.b.Two"}, nil, cp)
	if err != nil {
		t.Fatal(err)
	}

	body := string(mustRead(t, path))
	for _, want := range []string{"a.b.One.class", "a.b.Two.class", "createVariableFromClass", "withConfigRoot"} {
		if !strings.Contains(body, want) {
			t.Errorf("%q is missing from the bridge", want)
		}
	}

	// One class that cannot be reflected over must not take the rest with it.
	if strings.Count(body, "catch (Throwable ignored)") < 2 {
		t.Error("a failure in one class is not isolated from the others")
	}
}

// Nothing to bridge means nothing generated beyond the hook.
func TestTheBridgeIsEmptyWithNoConfigClasses(t *testing.T) {
	work := t.TempDir()
	cp := Classpath{Compile: []string{fakeJar(t, work, dashboardMarker)}}

	path, err := GenerateBridge(work, nil, nil, cp)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(mustRead(t, path)), "acmerobotics") {
		t.Error("dashboard is referenced with nothing to register")
	}
}

// Classes kept in the APK are found the ordinary way, so bridging them too
// would put them in the dashboard twice.
func TestKeptClassesAreNotBridged(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, SourceRoot, "org/firstinspires/ftc/teamcode/tuning")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Consts.java"),
		[]byte("@Config\npublic class Consts {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ConfigClasses(root, nil); len(got) != 1 ||
		got[0] != "org.firstinspires.ftc.teamcode.tuning.Consts" {
		t.Fatalf("got %v", got)
	}

	if got := ConfigClasses(root, []string{"org/firstinspires/ftc/teamcode/tuning"}); len(got) != 0 {
		t.Errorf("got %v, want nothing bridged", got)
	}
}

// fakeJar writes a jar containing one entry, for classpath detection.
func fakeJar(t *testing.T, dir, entry string) string {
	t.Helper()

	path := filepath.Join(dir, "fake.jar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	if _, err := archive.Create(entry); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Comparing the built APK against the robot's cannot work: two builds of an
// unchanged project differ, because D8 does not pack classes into dex files
// deterministically. Measured on a real project, classes3.dex and classes5.dex
// changed every build while every other entry stayed byte for byte the same, so
// a reload was never once judged equivalent and every deploy installed.
func TestTheSignatureIgnoresTeamCodeAndNoticesEverythingElse(t *testing.T) {
	root := t.TempDir()

	mkdir := func(parts ...string) string {
		dir := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mkdir(Module)
	write(GradleFile(root), "dependencies {\n}\n")

	team := mkdir(Module, "src", "main", "java", "org", "firstinspires", "ftc", "teamcode")
	write(filepath.Join(team, "Auto.java"), "public class Auto {}\n")

	res := mkdir(Module, "src", "main", "res")
	write(filepath.Join(res, "thing.xml"), "<x/>\n")

	before, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	// Team code is the thing that should reload, so it must not count.
	write(filepath.Join(team, "Auto.java"), "public class Auto { int x = 1; }\n")
	write(filepath.Join(team, "New.java"), "public class New {}\n")

	if after, _ := Signature(root); after != before {
		t.Error("a team code change counted as needing an install")
	}

	// Anything that ends up in the APK must count.
	for _, change := range []struct{ name, path, content string }{
		{"a gradle file", GradleFile(root), "dependencies {\n  implementation 'x'\n}\n"},
		{"a resource", filepath.Join(res, "thing.xml"), "<y/>\n"},
	} {
		original, _ := os.ReadFile(change.path)
		write(change.path, change.content)

		if after, _ := Signature(root); after == before {
			t.Errorf("%s did not count as a change", change.name)
		}

		write(change.path, string(original))
	}

	// And restoring everything has to give the original answer back, or the
	// comparison drifts and every deploy installs anyway.
	if after, _ := Signature(root); after != before {
		t.Error("the signature did not come back after undoing the changes")
	}
}

// Build outputs are derived, and hashing them would make the signature change
// on every build for the same reason the APK does.
// The Kotlin DSL names build files .gradle.kts, which does not end in .gradle.
// A project with one of those could change how the APK is built and still sign
// as unchanged, so the next deploy reloaded team code onto a robot whose APK no
// longer matched and reported success.
func TestTheSignatureNoticesKotlinDSLBuildFiles(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}

	// A Groovy project with one converted module, which is enough.
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("build.gradle", "// root\n")
	write(filepath.Join(Module, "build.gradle.kts"), `android { }`)

	before, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	write(filepath.Join(Module, "build.gradle.kts"), `android { buildTypes { } }`)

	after, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	if before == after {
		t.Error("changing a .gradle.kts did not change the signature, so a deploy " +
			"would reload onto an APK that no longer matches")
	}
}

// A version catalog names the version of every library that goes into the APK.
// Changing one changes the build, and a project keeping its SDK version there
// was signing that file as if it were documentation.
func TestTheSignatureNoticesAVersionCatalog(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build.gradle.kts"), []byte("// root"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog := filepath.Join(root, "gradle", "libs.versions.toml")
	if err := os.WriteFile(catalog, []byte("[versions]\nftc = \"11.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(catalog, []byte("[versions]\nftc = \"11.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	if before == after {
		t.Error("changing the SDK version in a catalog did not change the signature")
	}
}

func TestBuildInputsCoverBothGradleDialects(t *testing.T) {
	for _, name := range []string{
		"build.gradle", "build.gradle.kts", "settings.gradle.kts",
		"gradle.properties", "libs.versions.toml", "blob-dev.aar", "something.jar",
	} {
		if !isBuildInput(name) {
			t.Errorf("%s should be part of the signature", name)
		}
	}

	for _, name := range []string{
		"Robot.java", "README.md", "gradlew", "config.xml", "notes.kts",
	} {
		if isBuildInput(name) {
			t.Errorf("%s should not be part of the signature", name)
		}
	}
}

func TestTheSignatureSkipsBuildOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module, "build", "intermediates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Module, "build.gradle"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := Signature(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, Module, "build", "intermediates", "out.jar"),
		[]byte("anything"), 0o644); err != nil {
		t.Fatal(err)
	}

	if after, _ := Signature(root); after != before {
		t.Error("build output counted as a change")
	}
}

// A hardware device driver cannot be reloaded, and that is not a preference.
// Every reload builds a new classloader, so a reloaded driver is a different
// class each time while the device in the hardware map was built under an
// earlier one. hardwareMap.get then finds nothing assignable and the robot
// cannot find its own hardware.
func TestDeviceDriversAreFoundAndKept(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, SourceRoot, "org/firstinspires/ftc/teamcode/hw")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}

	driver := "@I2cDeviceType\n@DeviceProperties(xmlTag = \"MyThing\")\npublic class MyDriver {}\n"
	plain := "@TeleOp\npublic class Ordinary {}\n"

	if err := os.WriteFile(filepath.Join(pkg, "MyDriver.java"), []byte(driver), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "Ordinary.java"), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}

	found := FindDrivers(root)
	if len(found) != 1 || found[0] != "org/firstinspires/ftc/teamcode/hw/MyDriver" {
		t.Fatalf("got %v", found)
	}
}

// A driver usually sits among ordinary code, so keeping it must not drag its
// neighbours out of the reload with it.
func TestKeepingOneClassLeavesItsNeighboursReloadable(t *testing.T) {
	keep := []string{"org/firstinspires/ftc/teamcode/hw/MyDriver"}

	entries := []string{
		"org/firstinspires/ftc/teamcode/hw/MyDriver.class",
		"org/firstinspires/ftc/teamcode/hw/MyDriver$Params.class",
		"org/firstinspires/ftc/teamcode/hw/MyDriverHelper.class",
		"org/firstinspires/ftc/teamcode/hw/Ordinary.class",
	}

	reload, kept := split(entries, keep)

	// The class and its inner classes go, because they come from one file and
	// share its identity.
	if kept != 2 {
		t.Errorf("got %d kept, want the driver and its inner class", kept)
	}
	// A class whose name merely starts the same must not be caught.
	if len(reload) != 2 {
		t.Fatalf("got %v", reload)
	}
	for _, entry := range reload {
		if strings.Contains(entry, "MyDriver.class") || strings.Contains(entry, "MyDriver$") {
			t.Errorf("%s should have been kept", entry)
		}
	}
}

// The gradle exclusion has to agree with the split, or a class is in both the
// APK and the reload, or in neither.
func TestTheGradleBlockPinsAClassToItsDot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GradleFile(root), []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Exclude(root, "org/firstinspires/ftc/teamcode/hw/MyDriver"); err != nil {
		t.Fatal(err)
	}

	content := string(mustRead(t, GradleFile(root)))

	// The dot is what stops MyDriverHelper.java being kept too.
	if !strings.Contains(content, "!path.startsWith('org/firstinspires/ftc/teamcode/hw/MyDriver.')") {
		t.Errorf("a class keep is not pinned to its extension:\n%s", content)
	}

	// A package keep still uses a slash.
	if err := Exclude(root, "org/firstinspires/ftc/teamcode/tuning"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mustRead(t, GradleFile(root))),
		"!path.startsWith('org/firstinspires/ftc/teamcode/tuning/')") {
		t.Error("a package keep is not matched as a directory")
	}
}

// Excluding a directory prunes the subtree before any file under it is seen,
// so without the guard the kept class is in neither the APK nor the reload and
// the robot dies resolving it. On a real project this took the source set from
// 0 files to exactly the one kept file.
func TestTheGradleBlockNeverExcludesADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GradleFile(root), []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Exclude(root, "org/firstinspires/ftc/teamcode/hw/MyDriver"); err != nil {
		t.Fatal(err)
	}

	content := string(mustRead(t, GradleFile(root)))

	if !strings.Contains(content, "!details.directory") {
		t.Fatalf("directories are not spared, so a keep cannot survive:\n%s", content)
	}

	// It has to come first, or it is only reached once the path already matched.
	guard := strings.Index(content, "!details.directory")
	pkg := strings.Index(content, "path.startsWith('"+TeamPackage)
	if guard > pkg {
		t.Errorf("the directory guard is tested after the path, so it does not guard anything:\n%s", content)
	}
}

// Keeping a class keeps it in the source set while excluding everything it
// depends on, and javac then reports "cannot find symbol" on the import. The
// unannotated classes look fine only because they are not compiled at all.
func TestAKeptClassKeepsWhatItNeedsToCompile(t *testing.T) {
	root := t.TempDir()

	write := func(pkg, name, body string) {
		dir := filepath.Join(root, SourceRoot, filepath.FromSlash(TeamPackage), pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".java"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	team := strings.ReplaceAll(TeamPackage, "/", ".")

	write("hw", "MyDriver", `package `+team+`.hw;
import `+team+`.foo.MyTimer;
@DeviceProperties
public class MyDriver {
    MyTimer timer;
    Helper helper;
}`)
	write("hw", "Helper", "package "+team+".hw;\npublic class Helper {}")
	write("foo", "MyTimer", `package `+team+`.foo;
import `+team+`.util.Clock;
public class MyTimer {}`)
	write("util", "Clock", "package "+team+".util;\npublic class Clock {}")
	write("opmode", "Unrelated", "package "+team+".opmode;\npublic class Unrelated {}")

	kept := Closure(root, []string{TeamPackage + "/hw/MyDriver"})

	want := map[string]bool{
		TeamPackage + "/hw/MyDriver": true,
		// Imported directly.
		TeamPackage + "/foo/MyTimer": true,
		// Imported by what it imports.
		TeamPackage + "/util/Clock": true,
		// Same package, so referred to without an import.
		TeamPackage + "/hw/Helper": true,
	}

	for _, entry := range kept {
		if !want[entry] {
			t.Errorf("kept %s, which nothing needs", entry)
		}
		delete(want, entry)
	}
	for entry := range want {
		t.Errorf("did not keep %s, so the build cannot resolve it", entry)
	}
}

// A team class can be named in full where it is used, with no import line to
// follow, and the build fails just the same.
func TestAFullyQualifiedReferenceIsFollowed(t *testing.T) {
	root := t.TempDir()
	team := strings.ReplaceAll(TeamPackage, "/", ".")

	write := func(pkg, name, body string) {
		dir := filepath.Join(root, SourceRoot, filepath.FromSlash(TeamPackage), pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".java"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("hw", "MyDriver", `package `+team+`.hw;
@I2cDeviceType
public class MyDriver {
    `+team+`.foo.MyTimer timer = new `+team+`.foo.MyTimer();
}`)
	write("foo", "MyTimer", "package "+team+".foo;\npublic class MyTimer {}")

	kept := Closure(root, []string{TeamPackage + "/hw/MyDriver"})

	found := false
	for _, entry := range kept {
		if entry == TeamPackage+"/foo/MyTimer" {
			found = true
		}
	}
	if !found {
		t.Errorf("kept %v, missing the class named in full", kept)
	}
}

// Naming conventions do not separate a class from a package. Real projects have
// packages called Hardware and classes called velocityController, and guessing
// wrong emits a condition that matches nothing, so the entry is excluded
// despite being on the keep list.
func TestClassAndPackageAreToldApartByLooking(t *testing.T) {
	root := t.TempDir()
	team := filepath.Join(root, SourceRoot, filepath.FromSlash(TeamPackage))

	if err := os.MkdirAll(filepath.Join(team, "Hardware"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(team, "Hardware", "velocityController.java"),
		[]byte("package x;"), 0o644); err != nil {
		t.Fatal(err)
	}

	if isClassEntry(root, TeamPackage+"/Hardware") {
		t.Error("a capitalised package was read as a class")
	}
	if !isClassEntry(root, TeamPackage+"/Hardware/velocityController") {
		t.Error("a lowercase class was read as a package")
	}

	// Nothing on disk falls back to the convention rather than guessing wrong
	// in a new way.
	if !isClassEntry(root, TeamPackage+"/gone/Missing") {
		t.Error("an unknown entry did not fall back to the convention")
	}
}

// Everything reachable would be most of a project, and a kept class cannot
// reload. What is not needed has to stay out.
func TestTheClosureDoesNotKeepUnrelatedCode(t *testing.T) {
	root := t.TempDir()
	team := strings.ReplaceAll(TeamPackage, "/", ".")

	dir := filepath.Join(root, SourceRoot, filepath.FromSlash(TeamPackage), "hw")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"MyDriver": "package " + team + ".hw;\n@MotorType\npublic class MyDriver {}",
		"Sibling":  "package " + team + ".hw;\npublic class Sibling {}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name+".java"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	kept := Closure(root, []string{TeamPackage + "/hw/MyDriver"})

	if len(kept) != 1 || kept[0] != TeamPackage+"/hw/MyDriver" {
		t.Errorf("kept %v, want only the driver: an unreferenced sibling still reloads", kept)
	}
}

// The bridge must not touch anything shared. An earlier version set the thread
// context classloader, which repointed an SDK-owned thread at a loader that is
// discarded on the next reload and left it resolving through a dead one.
func TestTheBridgeChangesNothingGlobal(t *testing.T) {
	work := t.TempDir()

	path, err := GenerateBridge(work, []string{"a.b.Tuned"},
		nil, Classpath{Compile: []string{fakeJar(t, work, dashboardMarker)}})
	if err != nil {
		t.Fatal(err)
	}

	body := string(mustRead(t, path))
	for _, forbidden := range []string{"setContextClassLoader", "System.setProperty", "Thread.currentThread"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the bridge touches shared state: %s", forbidden)
		}
	}
}

// A class registered by an earlier reload holds a Class from a loader that no
// longer exists. Leaving it there is both a dead dashboard entry and a
// reference into a discarded classloader.
func TestTheBridgeRemovesWhatItNoLongerRegisters(t *testing.T) {
	work := t.TempDir()
	cp := Classpath{Compile: []string{fakeJar(t, work, dashboardMarker)}}

	// Previously registered Gone and Stays; now only Stays exists.
	path, err := GenerateBridge(work, []string{"a.b.Stays"}, []string{"Gone", "Stays"}, cp)
	if err != nil {
		t.Fatal(err)
	}

	body := string(mustRead(t, path))
	if !strings.Contains(body, `drop(root, "Gone")`) {
		t.Errorf("a class that is no longer registered is not removed:\n%s", body)
	}
	if strings.Contains(body, `drop(root, "Stays")`) {
		t.Error("a class that is still registered was removed")
	}
	if !strings.Contains(body, "a.b.Stays.class") {
		t.Error("the surviving class was not registered")
	}
}

func TestStaleNamesAreTheDifference(t *testing.T) {
	got := stale([]string{"a.b.One", "a.b.Two"}, []string{"One", "Three"})

	if len(got) != 1 || got[0] != "Three" {
		t.Errorf("got %v, want only Three", got)
	}

	if got := RegisteredNames([]string{"a.b.One", "Two"}); len(got) != 2 ||
		got[0] != "One" || got[1] != "Two" {
		t.Errorf("got %v", got)
	}
}
