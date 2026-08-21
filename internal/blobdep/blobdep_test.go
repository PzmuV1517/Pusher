package blobdep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func project(t *testing.T, gradle string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "TeamCode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GradleFile(root), []byte(gradle), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(GradleFile(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func activeLines(gradle string) int {
	n := 0
	for _, line := range strings.Split(gradle, "\n") {
		if m := depRe.FindStringSubmatch(line); m != nil && m[2] == "" {
			n++
		}
	}
	return n
}

const withComp = `dependencies {
    implementation 'org.firstinspires.ftc:Vision:11.1.0'
    implementation files('libs/blob-competition-v1.4.0.aar')
}
`

func TestDetectFindsTheActiveDependency(t *testing.T) {
	dep, err := Detect(project(t, withComp))
	if err != nil {
		t.Fatal(err)
	}
	if dep == nil {
		t.Fatal("expected to find blob")
	}
	if dep.Artifact != ArtifactComp || dep.Version != "v1.4.0" {
		t.Errorf("got %s %s", dep.Artifact, dep.Version)
	}
	if dep.Commented {
		t.Error("dependency is not commented out")
	}
	if dep.Present {
		t.Error("no AAR was placed, so Present must be false")
	}
}

func TestDetectReportsThePresenceOfTheAAR(t *testing.T) {
	root := project(t, withComp)
	if err := Place(root, ArtifactComp, "v1.4.0", []byte("not really an aar")); err != nil {
		t.Fatal(err)
	}

	dep, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if !dep.Present {
		t.Error("the AAR is on disk, so Present must be true")
	}
}

func TestDetectReturnsNilWhenAbsent(t *testing.T) {
	dep, err := Detect(project(t, "dependencies {\n    implementation 'x:y:1'\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if dep != nil {
		t.Errorf("expected no blob, got %+v", dep)
	}
}

func TestDetectPrefersActiveOverCommented(t *testing.T) {
	root := project(t, `dependencies {
    // implementation files('libs/blob-dev-v1.4.0.aar')
    implementation files('libs/blob-competition-v1.4.0.aar')
}
`)

	dep, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Artifact != ArtifactComp || dep.Commented {
		t.Errorf("got %+v, want the active competition line", dep)
	}
}

func TestSetArtifactSwitchesToDev(t *testing.T) {
	root := project(t, withComp)

	if err := SetArtifact(root, ArtifactDev); err != nil {
		t.Fatal(err)
	}

	dep, _ := Detect(root)
	if dep == nil || !dep.IsDev() || dep.Commented {
		t.Fatalf("got %+v, want an active dev line", dep)
	}
	if !strings.Contains(read(t, root), "libs/blob-dev-v1.4.0.aar") {
		t.Error("the files() path should name the dev AAR")
	}
}

func TestSetArtifactIsReversible(t *testing.T) {
	root := project(t, withComp)

	if err := SetArtifact(root, ArtifactDev); err != nil {
		t.Fatal(err)
	}
	if err := SetArtifact(root, ArtifactComp); err != nil {
		t.Fatal(err)
	}

	dep, _ := Detect(root)
	if dep == nil || dep.IsDev() || dep.Commented {
		t.Fatalf("got %+v, want an active competition line", dep)
	}
}

func TestSetArtifactNeverLeavesTwoActiveLines(t *testing.T) {
	starts := []string{
		withComp,
		`dependencies {
    implementation files('libs/blob-competition-v1.4.0.aar')
    implementation files('libs/blob-dev-v1.4.0.aar')
}
`,
		`dependencies {
    // implementation files('libs/blob-competition-v1.4.0.aar')
    // implementation files('libs/blob-dev-v1.4.0.aar')
}
`,
		`dependencies {
    implementation files('libs/blob-dev-v1.4.0.aar')
    // implementation files('libs/blob-dev-v1.4.0.aar')
}
`,
	}

	for _, start := range starts {
		for _, target := range []string{ArtifactComp, ArtifactDev} {
			root := project(t, start)
			if err := SetArtifact(root, target); err != nil {
				t.Fatalf("%s: %v", target, err)
			}

			gradle := read(t, root)
			if n := activeLines(gradle); n != 1 {
				t.Errorf("target %s left %d active lines:\n%s", target, n, gradle)
			}

			dep, _ := Detect(root)
			if dep == nil || dep.Artifact != target || dep.Commented {
				t.Errorf("target %s: active line is %+v", target, dep)
			}
		}
	}
}

func TestSetVersionUpdatesCommentedLinesToo(t *testing.T) {
	root := project(t, `dependencies {
    implementation files('libs/blob-competition-v1.4.0.aar')
    // implementation files('libs/blob-dev-v1.4.0.aar')
}
`)

	if err := SetVersion(root, "v1.5.0"); err != nil {
		t.Fatal(err)
	}

	gradle := read(t, root)
	if strings.Contains(gradle, "v1.4.0") {
		t.Errorf("a line was left on the old version:\n%s", gradle)
	}
	if !strings.Contains(gradle, "// implementation files('libs/blob-dev-v1.5.0.aar')") {
		t.Errorf("the parked line should stay parked and move version:\n%s", gradle)
	}
	if n := activeLines(gradle); n != 1 {
		t.Errorf("SetVersion changed how many lines are active: %d", n)
	}
}

func TestAddInsertsIntoDependenciesBlock(t *testing.T) {
	root := project(t, "dependencies {\n    implementation 'x:y:1'\n}\n")

	if err := Add(root, ArtifactComp, "v1.4.0"); err != nil {
		t.Fatal(err)
	}

	dep, _ := Detect(root)
	if dep == nil || dep.Artifact != ArtifactComp || dep.Version != "v1.4.0" {
		t.Fatalf("got %+v", dep)
	}
	if !strings.Contains(read(t, root), "files('libs/blob-competition-v1.4.0.aar')") {
		t.Error("expected a files() line")
	}
}

func TestAddRefusesWhenBlobIsAlreadyThere(t *testing.T) {
	if err := Add(project(t, withComp), ArtifactDev, "v1.4.0"); err == nil {
		t.Error("adding a second blob dependency should be refused")
	}
}

func TestDetectHandlesDoubleQuotesAndParens(t *testing.T) {
	root := project(t, `dependencies {
    api files("libs/blob-dev-v1.4.0.aar")
}
`)

	dep, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if dep == nil || !dep.IsDev() {
		t.Fatalf("got %+v", dep)
	}
}

func TestEnsureIgnoredIsIdempotent(t *testing.T) {
	root := project(t, withComp)

	added, err := EnsureIgnored(root)
	if err != nil || !added {
		t.Fatalf("first call: added=%v err=%v", added, err)
	}

	added, err = EnsureIgnored(root)
	if err != nil || added {
		t.Fatalf("second call should be a no-op: added=%v err=%v", added, err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), ignoreRule); n != 1 {
		t.Errorf("rule appears %d times, want 1:\n%s", n, data)
	}
}

func TestEnsureIgnoredKeepsExistingContent(t *testing.T) {
	root := project(t, withComp)
	path := filepath.Join(root, ".gitignore")

	if err := os.WriteFile(path, []byte("build/\n.idea/"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureIgnored(root); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	for _, want := range []string{"build/", ".idea/", ignoreRule} {
		if !strings.Contains(string(data), want) {
			t.Errorf("%q missing from:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), ".idea/"+ignoreRule) {
		t.Errorf("the last existing rule got mangled:\n%s", data)
	}
}

func TestPruneRemovesOtherBlobAARsOnly(t *testing.T) {
	root := project(t, withComp)

	for _, name := range []string{
		"blob-competition-v1.4.0.aar",
		"blob-dev-v1.4.0.aar",
		"blob-competition-v1.3.0.aar",
	} {
		if err := os.MkdirAll(LibsDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(LibsDir(root), name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(LibsDir(root), "someones-other.aar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Prune(root, ArtifactComp, "v1.4.0"); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(LibsDir(root))
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name()] = true
	}

	if !got["blob-competition-v1.4.0.aar"] {
		t.Error("the AAR in use was removed")
	}
	if !got["someones-other.aar"] {
		t.Error("Prune must not touch AARs that are not blob's")
	}
	if got["blob-dev-v1.4.0.aar"] || got["blob-competition-v1.3.0.aar"] {
		t.Errorf("stale blob AARs were left behind: %v", got)
	}
}

func TestPruneSurvivesAMissingLibsDir(t *testing.T) {
	if err := Prune(project(t, withComp), ArtifactComp, "v1.4.0"); err != nil {
		t.Errorf("no libs directory is not an error: %v", err)
	}
}

func TestAARNameMatchesTheReleaseAssetContract(t *testing.T) {
	if got := AARName(ArtifactComp, "v1.4.0"); got != "blob-competition-v1.4.0.aar" {
		t.Errorf("got %q", got)
	}
	if got := AARName(ArtifactDev, "v1.4.0"); got != "blob-dev-v1.4.0.aar" {
		t.Errorf("got %q", got)
	}
}

// A branch release carries its label in the version, v1.8.0-RSTController.1,
// which has to survive being written into the gradle file and read back out.
// The AAR beside it is named after the same string, so a version that does not
// round-trip is a file that cannot be found.
func TestALabelledVersionRoundTrips(t *testing.T) {
	const version = "v1.8.0-RSTController.1"

	root := project(t, `dependencies {
    implementation files('libs/blob-competition-v1.7.0.aar')
}`)

	if err := SetVersion(root, version); err != nil {
		t.Fatal(err)
	}

	dep, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if dep == nil {
		t.Fatal("the dependency was not found after being written")
	}
	if dep.Version != version {
		t.Errorf("version came back as %q, want %q", dep.Version, version)
	}

	if got := AARName(dep.Artifact, dep.Version); got != "blob-competition-"+version+".aar" {
		t.Errorf("AAR name = %q, which is not what the release attaches", got)
	}
}
