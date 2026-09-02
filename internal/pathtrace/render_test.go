package pathtrace

import (
	"os"
	"path/filepath"
	"testing"
)

// render writes the page and hands back its text.
func render(t *testing.T, trace *Trace) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "page.html")
	if err := trace.Render(path, DefaultLimits()); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
