package ftcproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every patch slim makes is Groovy. On a Kotlin DSL project the patterns match
// nothing, so the deploy would package everything it always did while reporting
// that it had been slimmed. Reported success is worse than a refusal, because
// the whole point is the size of the transfer and nobody checks it.
func TestSlimRefusesAProjectItCannotPatch(t *testing.T) {
	write := func(t *testing.T, names ...string) string {
		root := t.TempDir()
		for _, name := range names {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("// x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}

	t.Run("a stock project is fine", func(t *testing.T) {
		if err := Supported(write(t, "build.common.gradle")); err != nil {
			t.Errorf("an ordinary project was refused: %v", err)
		}
	})

	t.Run("kotlin dsl at the root", func(t *testing.T) {
		err := Supported(write(t, "build.gradle.kts", "settings.gradle.kts"))
		if err == nil {
			t.Fatal("a Kotlin DSL project was accepted")
		}
		if !strings.Contains(err.Error(), "Kotlin DSL") {
			t.Errorf("the reason does not name the DSL: %v", err)
		}
	})

	t.Run("kotlin dsl in the module only", func(t *testing.T) {
		err := Supported(write(t, filepath.Join("TeamCode", "build.gradle.kts")))
		if err == nil || !strings.Contains(err.Error(), "Kotlin DSL") {
			t.Errorf("a converted module was not recognised: %v", err)
		}
	})

	t.Run("a converted common file still counts", func(t *testing.T) {
		// Both present: Groovy wins, since that is the file slim edits.
		root := write(t, "build.common.gradle", "build.gradle.kts")
		if err := Supported(root); err != nil {
			t.Errorf("a project that still has build.common.gradle was refused: %v", err)
		}
	})

	t.Run("not an FTC project at all", func(t *testing.T) {
		err := Supported(write(t, "README.md"))
		if err == nil {
			t.Fatal("a directory with no build files was accepted")
		}
		if strings.Contains(err.Error(), "Kotlin") {
			t.Errorf("the reason blames Kotlin for a missing project: %v", err)
		}
	})
}
