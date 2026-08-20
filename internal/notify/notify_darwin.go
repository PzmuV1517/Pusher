//go:build darwin

package notify

import (
	"os/exec"
	"strings"
)

const supported = true

// osascript is already on every Mac, which is the whole reason to use it: a
// notification is not worth a dependency, a helper binary, or a code-signed
// bundle to own the alert.
func send(title, body string) bool {
	script := "display notification " + quote(body) + " with title " + quote(title)
	return exec.Command("osascript", "-e", script).Run() == nil
}

// quote makes an AppleScript string literal, where the escapes are the same two
// C has and nothing else.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
