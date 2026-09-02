package hubwifi

import (
	"os"
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/adb"
)

// What a Control Hub actually reports, so the parsing is tested against the
// real strings rather than tidy ones.
const (
	procVersion116 = "Linux version 3.10.245 (rev@android-dev-vm-1) (gcc version 4.9 20150123 (prerelease) (GCC) ) #18 SMP PREEMPT Thu May 8 13:13:30 CDT 2025"
	procVersion114 = "Linux version 3.10.104 (rev@android-dev-vm-1) (gcc version 4.9 20150123 (prerelease) (GCC) ) #12 SMP PREEMPT Wed Jan 17 09:00:00 CST 2024"
)

func TestTheHubsOwnStringsAreUnderstood(t *testing.T) {
	if got := KernelRelease(procVersion116); got != "3.10.245" {
		t.Errorf("kernel release = %q", got)
	}
	if got := OSRelease("1.1.6"); got != "1.1.6" {
		t.Errorf("os release = %q", got)
	}

	// The property is not always bare.
	if got := OSRelease("Control Hub OS 1.1.6"); got != "1.1.6" {
		t.Errorf("os release from a decorated property = %q", got)
	}

	if KernelRelease("nonsense") != "" || OSRelease("nonsense") != "" {
		t.Error("read a version out of something that has none")
	}
}

// An exact match is the one with nothing to argue about: same source as the
// kernel it loads into.
func TestAnExactReleaseWins(t *testing.T) {
	driver, match := For("1.1.6", procVersion116)

	if match != ByOS {
		t.Fatalf("match = %v, want ByOS", match)
	}
	if driver.OS != "1.1.6" || driver.Kernel != "3.10.245" {
		t.Errorf("picked %s / %s", driver.OS, driver.Kernel)
	}
}

// Seven of REV's eight releases share one kernel, and the kernel checks
// vermagic and nothing else: this one is built without CONFIG_MODVERSIONS, so
// a module built for a different release of the same kernel is accepted. That
// is what makes an unlisted hub work rather than fail.
func TestADifferentReleaseOnTheSameKernelStillLoads(t *testing.T) {
	// 1.1.3 shares a commit with 1.1.2, so pusher does not carry it by name.
	driver, match := For("1.1.3", procVersion114)

	if match != ByKernel {
		t.Fatalf("match = %v, want ByKernel", match)
	}
	if driver.Kernel != "3.10.104" {
		t.Errorf("picked a driver for kernel %s, which this hub would refuse", driver.Kernel)
	}

	// And the newest of the ones that fit, not the oldest.
	if driver.OS != "1.1.4" {
		t.Errorf("picked %s, want the closest release that fits", driver.OS)
	}
}

// A hub on a kernel nobody has built for has to be told so, with the list and
// somewhere to ask. Loading a mismatched module is not a fallback: the kernel
// refuses it, and if it did not, it would be running code built against a
// different kernel's structures.
func TestAnUnknownKernelIsRefusedAndExplained(t *testing.T) {
	_, match := For("2.0.0", "Linux version 4.9.100 (someone@somewhere)")
	if match != NoMatch {
		t.Fatal("offered a driver for a kernel nothing was built against")
	}

	said := Unsupported("2.0.0", "Linux version 4.9.100 (someone@somewhere)")

	for _, want := range []string{"2.0.0", "4.9.100", "1.1.6", "github.com/PzmuV1517/Pusher/issues", "pusher ip --adapters"} {
		if !strings.Contains(said, want) {
			t.Errorf("the explanation does not mention %q:\n%s", want, said)
		}
	}
}

// Nothing at all to go on is still refused rather than guessed at.
func TestNothingToGoOnIsNotAGuess(t *testing.T) {
	if _, match := For("", ""); match != NoMatch {
		t.Error("picked a driver for a hub that reported nothing")
	}
}

// Every driver named in the table has to be carried, or a hub matches an entry
// that unpacks to nothing.
func TestEveryDriverInTheTableIsActuallyCarried(t *testing.T) {
	if len(Drivers) == 0 {
		t.Fatal("no drivers carried at all")
	}

	for _, d := range Drivers {
		path, err := Extract(d)
		if err != nil {
			t.Errorf("%s is in the table but not in the binary: %v", d.OS, err)
			continue
		}
		defer os.RemoveAll(path)

		info, err := os.Stat(path)
		if err != nil || info.Size() < 1_000_000 {
			t.Errorf("%s unpacked to %d bytes, which is not a kernel module", d.OS, info.Size())
		}
	}
}

// The vermagic a module carries is the only thing the hub checks, so a table
// claiming a kernel the module was not built for would hand somebody a driver
// their hub silently refuses.
//
// Read out of the module's own bytes rather than with modinfo, which is not on
// every machine pusher is developed on and cannot always read another
// architecture's modules when it is.
func TestTheTableAgreesWithWhatTheModulesSayAboutThemselves(t *testing.T) {
	for _, d := range Drivers {
		path, err := Extract(d)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(path)

		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(body), "vermagic="+d.Vermagic()) {
			t.Errorf("%s: the table says %q, which is not what the module carries",
				d.OS, d.Vermagic())
		}
	}
}

// insmod says why it refused, and it does not always use the word "error".
// Checking only for that word turned every refusal into "the driver loaded but
// claimed nothing", which sends somebody looking at their adapter when the
// module was never loaded at all.
func TestARefusalIsRecognisedHoweverItIsWorded(t *testing.T) {
	for _, out := range []string{
		"insmod: failed to load /data/local/tmp/wifi.ko: Exec format error",
		"insmod: init_module '/data/local/tmp/wifi.ko' failed (File exists)",
		"insmod: failed to load: Operation not permitted",
		"insmod: can't insert module: invalid module format",
		"open failed: No such file or directory",
		"Permission denied",
	} {
		if !refused(out) {
			t.Errorf("did not recognise a refusal: %q", out)
		}
	}

	// And silence is what success looks like.
	for _, out := range []string{"", "   ", "\n"} {
		if refused(out) {
			t.Errorf("called %q a refusal", out)
		}
	}
}

// Each refusal has a different answer, and the message is the only place
// somebody finds out which one they have.
func TestEachRefusalSaysWhatToDoAboutIt(t *testing.T) {
	for _, tc := range []struct{ out, want string }{
		{"insmod: failed to load: Exec format error", "kernel mismatch"},
		{"insmod: init_module failed (File exists)", "already in the kernel"},
		{"insmod: Operation not permitted", "needs root"},
	} {
		if got := insmodHint(tc.out); !strings.Contains(got, tc.want) {
			t.Errorf("%q gave no hint about %q, got: %s", tc.out, tc.want, got)
		}
	}

	// Something unrecognised gets no invented advice.
	if got := insmodHint("something nobody has seen before"); got != "" {
		t.Errorf("invented a hint: %s", got)
	}
}

// A command that refuses says why on the way out, and adb.Shell used to throw
// exactly that away: it returns an empty string whenever the exit code is
// non-zero and buries the message inside the error. Every refusal therefore
// arrived as "no reason given", which is worse than useless because it points
// at the adapter when the module was never loaded.
func TestAFailedCommandKeepsWhatItSaid(t *testing.T) {
	// A command that prints and then fails, which is the shape of every refusal.
	out, err := adb.ShellOutput("", "definitely-not-a-real-adb-subcommand")

	if err == nil {
		t.Skip("adb accepted a command it should not have")
	}
	if strings.TrimSpace(out) == "" {
		t.Error("the output was thrown away, which is how a refusal loses its reason")
	}
}

// A second run finds the driver already in the kernel, and so does every run
// after a manual one. That is a state to carry on from, not a failure: the
// driver is loaded either way, and whether it has claimed the adapter is a
// different question with a different answer.
func TestAlreadyLoadedIsNotAFailure(t *testing.T) {
	for _, out := range []string{
		"insmod: failed to load /data/local/tmp/pusher-wifi/wifi.ko: File exists",
		"insmod: init_module failed (File exists)",
	} {
		if !alreadyLoaded(out) {
			t.Errorf("treated an already loaded module as a failure: %q", out)
		}
	}

	for _, out := range []string{
		"insmod: failed to load: Exec format error",
		"insmod: Operation not permitted",
		"",
	} {
		if alreadyLoaded(out) {
			t.Errorf("called %q already loaded", out)
		}
	}
}

// The boot script is the whole sequence with nobody watching, so every trap the
// laptop side works around has to be worked around there too. A missing line
// here is a robot that comes back from a reboot on nothing.
func TestTheBootScriptCarriesEveryWorkaround(t *testing.T) {
	r := runner{}
	script := r.bootScript(Options{SSID: "ASUS"})

	for what, needle := range map[string]string{
		"loads the driver":            "insmod",
		"kills every supplicant":      "wpa_supplican[t]",
		"clears the stale socket":     "rm -f \"$CTRL/$IFACE\"",
		"cycles the interface":        "ip link set \"$IFACE\" down",
		"selects the network":         "select_network 0",
		"waits for the association":   "COMPLETED",
		"applies the lease itself":    "udhcpc",
		"routes where the rules look": "table $TABLE",
		"keeps watching":              "while true",
	} {
		if !strings.Contains(script, needle) {
			t.Errorf("the boot script no longer %s (looked for %q)", what, needle)
		}
	}

	// And the network it was set up for, or it joins nothing.
	if !strings.Contains(script, "ASUS") {
		t.Error("the boot script does not name the network")
	}
}
