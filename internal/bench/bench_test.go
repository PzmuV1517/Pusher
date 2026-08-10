package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A measurement taken against an APK older than the build files is not a
// measurement of the settings being reported. The Kotlin DSL names its build
// files .gradle.kts, which does not match *.gradle, so a project using it was
// never flagged as stale.
func TestAStaleAPKIsNoticedInEitherGradleDialect(t *testing.T) {
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()

			outputs := filepath.Join(root, "TeamCode", "build", "outputs", "apk", "debug")
			if err := os.MkdirAll(outputs, 0o755); err != nil {
				t.Fatal(err)
			}

			apk := filepath.Join(outputs, "TeamCode-debug.apk")
			if err := os.WriteFile(apk, []byte("apk"), 0o644); err != nil {
				t.Fatal(err)
			}

			gradleFile := filepath.Join(root, "TeamCode", name)
			if err := os.WriteFile(gradleFile, []byte("android { }"), 0o644); err != nil {
				t.Fatal(err)
			}

			built := time.Now().Add(-time.Hour)

			// The build file was written after the APK, so the APK is stale.
			if !olderThanGradle(apk, built) {
				t.Errorf("an APK built an hour before %s was not reported as stale", name)
			}

			// And a build file older than the APK is not.
			old := time.Now().Add(-2 * time.Hour)
			if err := os.Chtimes(gradleFile, old, old); err != nil {
				t.Fatal(err)
			}
			if olderThanGradle(apk, built) {
				t.Errorf("an APK newer than %s was reported as stale", name)
			}
		})
	}
}
