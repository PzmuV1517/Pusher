package extreme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// slothProject writes a project whose module gradle file has the given
// dependency lines in it.
func slothProject(t *testing.T, deps string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Module), 0o755); err != nil {
		t.Fatal(err)
	}

	gradle := filepath.Join(root, Module, "build.gradle")
	body := "android { }\n\ndependencies {\n" + deps + "\n}\n"

	if err := os.WriteFile(gradle, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// The failure this prevents empties the OpMode list and survives uninstalling
// pusher, because the copy that breaks it is on the robot rather than the
// laptop. So it has to be refused before anything is written, not warned about
// afterwards.
func TestAProjectWithSlothIsRefused(t *testing.T) {
	root := slothProject(t, `    implementation "dev.frozenmilk.sinister:Sloth:0.2.4"`)

	if !UsesSloth(root) {
		t.Fatal("did not notice Sloth in the dependencies")
	}

	err := Supported(root)
	if err == nil {
		t.Fatal("set up Pusher Extreme on a project that already reloads team code")
	}
	if !strings.Contains(err.Error(), "Sloth") {
		t.Errorf("the refusal does not say what it is about: %v", err)
	}

	// And the refusal has to hold at the point that writes, since that is what
	// takes team code out of the APK.
	if err := Exclude(root); err == nil {
		t.Error("Exclude went ahead anyway, which is the APK gutted for a reload that will not work")
	}
	if Excluded(root) {
		t.Error("the exclusion block was written to a project that was refused")
	}
}

// A version catalog splits the group and the name across fields, so a check
// that only looks for the joined coordinate misses a project that has it.
func TestSlothIsFoundInAVersionCatalog(t *testing.T) {
	root := slothProject(t, `    implementation libs.sloth`)

	catalog := filepath.Join(root, "gradle")
	if err := os.MkdirAll(catalog, 0o755); err != nil {
		t.Fatal(err)
	}

	body := "[libraries]\nsloth = { group = \"dev.frozenmilk.sinister\", name = \"Sloth\", version = \"0.2.4\" }\n"
	if err := os.WriteFile(filepath.Join(catalog, "libs.versions.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	if !UsesSloth(root) {
		t.Error("Sloth declared in a version catalog went unnoticed")
	}
}

func TestSlothIsFoundAsALocalArchive(t *testing.T) {
	root := slothProject(t, `    implementation files('libs/Sloth-0.2.4.aar')`)

	libs := filepath.Join(root, Module, "libs")
	if err := os.MkdirAll(libs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libs, "Sloth-0.2.4.aar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !UsesSloth(root) {
		t.Error("an AAR dropped in libs went unnoticed")
	}
}

// Refusing the wrong projects is its own bug: it takes a working feature away
// from teams who never had this problem. Sinister is a scanner rather than a
// loader and arrives on its own with other libraries, and Panels does not bring
// it at all.
func TestOrdinaryProjectsAreNotRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps string
	}{
		{"nothing", ""},
		{"panels", `    implementation "com.bylazar:panels:1.0.3"`},
		{"nextftc", `    implementation "dev.nextftc:core:1.0.0"`},
		{"dashboard", `    implementation "com.acmerobotics.dashboard:dashboard:0.4.16"`},
		{"sinister alone", `    implementation "dev.frozenmilk:Sinister:1.0.0"`},
		{"the word in a comment", `    // sloth is a hot reload tool we looked at`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := slothProject(t, tc.deps)

			if UsesSloth(root) {
				t.Error("called this a Sloth project")
			}
			if err := Supported(root); err != nil {
				t.Errorf("refused an ordinary project: %v", err)
			}
		})
	}
}
