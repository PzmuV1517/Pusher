package hubwifi

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeHub puts an adb on the path that answers the way a Control Hub does.
//
// The behaviour that matters: /sys/class/net/wlan1 is a symlink to a directory,
// so `ls` on it prints what is inside and `ls -d` prints the path. Getting that
// backwards made pusher decide the interface was missing on a hub where it was
// plainly there.
func fakeHub(t *testing.T, ifacePresent bool) {
	t.Helper()

	dir := t.TempDir()
	present := "0"
	if ifacePresent {
		present = "1"
	}

	script := `#!/bin/sh
PATH=/bin:/usr/bin
# Drop everything up to the shell command itself.
while [ $# -gt 0 ]; do
  case "$1" in
    shell) shift; break ;;
    *) shift ;;
  esac
done
# And the su prefix the caller adds.
[ "$1" = "su" ] && shift 2

if [ "$1" = "ls" ] && [ "$2" = "-d" ] && [ "$3" = "/sys/class/net/wlan1" ]; then
  if [ ` + present + ` -eq 1 ]; then echo "/sys/class/net/wlan1"; exit 0; fi
  echo "ls: /sys/class/net/wlan1: No such file or directory" >&2; exit 1
fi

if [ "$1" = "ls" ] && [ "$2" = "/sys/class/net/wlan1" ]; then
  # What a bare ls on the symlink actually prints: the contents, with the
  # interface's own name nowhere in it.
  if [ ` + present + ` -eq 1 ]; then printf 'address\nmtu\noperstate\nphy80211\n'; exit 0; fi
  echo "ls: /sys/class/net/wlan1: No such file or directory" >&2; exit 1
fi

if [ "$1" = "id" ]; then echo 0; exit 0; fi
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// The interface is there and pusher has to see it, or it tries to load a driver
// that is already loaded and then waits for something it cannot detect.
func TestAnInterfaceThatIsThereIsSeen(t *testing.T) {
	fakeHub(t, true)

	r := runner{serial: "robot", root: "su 0"}
	if !r.hasIface() {
		t.Error("did not see wlan1 on a hub that has it")
	}
}

func TestAnInterfaceThatIsNotThereIsNotImagined(t *testing.T) {
	fakeHub(t, false)

	r := runner{serial: "robot", root: "su 0"}
	if r.hasIface() {
		t.Error("saw wlan1 on a hub without one")
	}
}
