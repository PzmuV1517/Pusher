package extreme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opModeProject writes a project with one declared OpMode in it.
func opModeProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	src := filepath.Join(root, SourceRoot)

	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	body := `package org.firstinspires.ftc.teamcode;

@TeleOp(name = "TestTeleop")
public class TestTeleop extends OpMode {
}
`
	if err := os.WriteFile(filepath.Join(src, "TestTeleop.java"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A reload can deliver every class and register none of them, and this is the
// check that catches it. When it cannot run, saying nothing tells a team the
// deploy was fine. It was silence exactly like this that let a robot with an
// empty OpMode list report a clean reload for a day.
func TestAReloadNobodyCouldCheckSaysSo(t *testing.T) {
	root := opModeProject(t)

	if len(DeclaredOpModes(root)) == 0 {
		t.Fatal("the fixture declares no OpModes, so this proves nothing")
	}

	// No robot, so no dashboard can be asked, which is the case that used to
	// pass in silence.
	step, warning := verified(root, "")

	if step != "" {
		t.Errorf("claimed something was verified: %q", step)
	}
	if warning == "" {
		t.Fatal("a reload nobody could check reported nothing at all")
	}
	for _, want := range []string{"could not check", "Driver Station", "Panels"} {
		if !strings.Contains(warning, want) {
			t.Errorf("the warning does not mention %q:\n%s", want, warning)
		}
	}
}

// A project with no OpModes in it has nothing to verify, and a warning there
// would be pusher worrying at somebody about an absence they chose.
func TestAProjectWithNoOpModesIsNotWarnedAt(t *testing.T) {
	root := t.TempDir()

	step, warning := verified(root, "")
	if step != "" || warning != "" {
		t.Errorf("said something about a project with no OpModes: %q / %q", step, warning)
	}
}
