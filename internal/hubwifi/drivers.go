package hubwifi

import (
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A kernel module is compiled against one kernel and the kernel refuses
// anything else, so a driver cannot be downloaded and cannot be built on the
// laptop that wants it. These are built ahead of time from the kernel source
// REV publishes, one per Control Hub OS release, and carried in pusher so a
// team at an event with no internet still has the one their hub needs.
//
// Built from github.com/REVrobotics/kernel-controlhub-android, which already
// contains the driver at drivers/net/wireless/rockchip_wlan/rtl8822bu. It is
// simply not enabled in the config REV ships, so this is their own source with
// one line turned on, plus the TP-Link device IDs Realtek never listed.

//go:embed drivers/*.ko.gz
var carried embed.FS

// Driver is one module pusher carries.
type Driver struct {
	// OS is the Control Hub OS release it was built from.
	OS string

	// Kernel is the release string, which is what the kernel compares. Modules
	// are checked against vermagic and nothing else here: this kernel is built
	// without CONFIG_MODVERSIONS, so a module whose kernel release matches is
	// accepted whatever it was built from.
	Kernel string

	file string
}

// Vermagic is what the kernel will compare this module against.
func (d Driver) Vermagic() string {
	return d.Kernel + " SMP preempt mod_unload aarch64"
}

// Drivers is everything pusher carries, newest first.
//
// Six modules for eight OS releases: 1.1.1 and 1.1.3 share a commit with the
// release below them, so building them again would produce the same driver
// under another name.
var Drivers = []Driver{
	{OS: "1.1.6", Kernel: "3.10.245", file: "drivers/8822bu-1.1.6.ko.gz"},
	{OS: "1.1.4", Kernel: "3.10.104", file: "drivers/8822bu-1.1.4.ko.gz"},
	{OS: "1.1.2", Kernel: "3.10.104", file: "drivers/8822bu-1.1.2.ko.gz"},
	{OS: "1.1.0", Kernel: "3.10.104", file: "drivers/8822bu-1.1.0.ko.gz"},
	{OS: "1.0.1", Kernel: "3.10.104", file: "drivers/8822bu-1.0.1.ko.gz"},
	{OS: "1.0.0", Kernel: "3.10.104", file: "drivers/8822bu-1.0.0.ko.gz"},
}

// Match is how well a carried driver fits a hub.
type Match int

const (
	// NoMatch means nothing pusher carries can load on this hub.
	NoMatch Match = iota

	// ByKernel means the kernel will accept it, but it was built from a
	// different OS release. That is enough: the kernel compares vermagic and
	// this one has no module versioning to check anything finer.
	ByKernel

	// ByOS means it was built from exactly this OS release.
	ByOS
)

var releaseRe = regexp.MustCompile(`Linux version ([0-9]+\.[0-9]+\.[0-9]+)`)

// KernelRelease pulls the version out of what /proc/version says.
func KernelRelease(procVersion string) string {
	if m := releaseRe.FindStringSubmatch(procVersion); m != nil {
		return m[1]
	}
	return ""
}

// OSRelease pulls a Control Hub OS number out of whatever the property says.
//
// The property is not always bare: it can carry a name around the number, and
// what matters is the number.
var osRe = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+)`)

func OSRelease(property string) string {
	if m := osRe.FindStringSubmatch(property); m != nil {
		return m[1]
	}
	return ""
}

// For picks the driver to load on a hub.
//
// The exact OS release first, because a driver built from the same source as
// the kernel it loads into is the one with nothing to argue about. Failing
// that, any module built for the same kernel release, which is what the kernel
// itself checks.
func For(osVersion, procVersion string) (Driver, Match) {
	os := OSRelease(osVersion)
	kernel := KernelRelease(procVersion)

	if os != "" {
		for _, d := range Drivers {
			if d.OS == os {
				return d, ByOS
			}
		}
	}

	if kernel != "" {
		// Newest first, so a hub matched by kernel alone gets the driver built
		// from the closest release rather than the oldest one that fits.
		candidates := append([]Driver(nil), Drivers...)
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].OS > candidates[j].OS })

		for _, d := range candidates {
			if d.Kernel == kernel {
				return d, ByKernel
			}
		}
	}

	return Driver{}, NoMatch
}

// Extract writes a carried driver out where adb can push it from.
func Extract(d Driver) (string, error) {
	packed, err := carried.Open(d.file)
	if err != nil {
		return "", fmt.Errorf("pusher does not carry %s after all: %w", d.OS, err)
	}
	defer packed.Close()

	unpacked, err := gzip.NewReader(packed)
	if err != nil {
		return "", err
	}
	defer unpacked.Close()

	dir, err := os.MkdirTemp("", "pusher-driver-*")
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "8822bu.ko")

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, unpacked); err != nil {
		return "", err
	}
	return path, nil
}

// Unsupported is what to tell somebody whose hub pusher has no driver for.
//
// Named versions rather than a shrug: the answer is either "update your hub to
// one of these" or "ask for yours", and both need the list.
func Unsupported(osVersion, procVersion string) string {
	var have []string
	for _, d := range Drivers {
		have = append(have, d.OS)
	}

	os := OSRelease(osVersion)
	if os == "" {
		os = "unknown"
	}
	kernel := KernelRelease(procVersion)
	if kernel == "" {
		kernel = "unknown"
	}

	return fmt.Sprintf(
		"pusher has no Wi-Fi driver for Control Hub OS %s (kernel %s).\n"+
			"    It carries: %s\n"+
			"\n"+
			"    A driver has to be built against the exact kernel it loads into, so one\n"+
			"    for your hub has to be compiled rather than downloaded. Either update the\n"+
			"    hub to a release above, or ask for yours to be added:\n"+
			"    https://github.com/PzmuV1517/Pusher/issues/new?title=Wi-Fi+driver+for+Control+Hub+OS+%s\n"+
			"\n"+
			"    Include the output of `pusher ip --adapters`, which has the kernel and\n"+
			"    the adapter's USB id in it. That is everything needed to build one.",
		os, kernel, strings.Join(have, ", "), os)
}
