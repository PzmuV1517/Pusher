package robot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
)

// A Control Hub cannot join a network over its own radio: that radio is the
// access point the Driver Station connects to. A second radio is a different
// question, and the answer is not something a laptop can reason about. It comes
// down to whether the hub's kernel has a driver for the adapter somebody
// plugged in, and the only way to find out is to look at the hub.
//
// So this looks. Read only: it asks what interfaces exist, what is on the USB
// bus, and whether the tools to bring an interface up are there. Nothing here
// changes anything, because the thing that would change something could take
// down the access point the robot is being driven over.

// Iface is one network interface on the hub.
type Iface struct {
	Name     string
	Wireless bool
	Up       bool
	Address  string

	// Phy is the radio this interface belongs to. Two interfaces on one phy are
	// one piece of hardware: a hub has wlan0 and p2p0 on the same radio, and
	// counting interfaces rather than radios calls that an adapter.
	Phy string
}

// USBDevice is something on the hub's USB bus.
type USBDevice struct {
	Vendor  string
	Product string
	Name    string

	// Class is what the device says its interface is, as class/subclass/
	// protocol. It decides whether a driver matches at all: Realtek's tables
	// ask for ff/ff/ff, and a dongle sitting in its flip-flop storage mode
	// presents itself as a CD drive instead and matches nothing.
	Class string
}

// Adapter is what the hub has to work with.
type Adapter struct {
	Ifaces     []Iface
	USB        []USBDevice
	Supplicant bool

	// Extra is a wireless interface that is not the one running the access
	// point, which is the whole question.
	Extra string

	// Drivers is the USB Wi-Fi chipsets this hub's kernel can drive, which is
	// the difference between "that adapter does not work" and "buy this one".
	Drivers []string

	// Ethernet is the USB Ethernet chipsets it can drive. The other way onto a
	// network, cabled rather than wireless, and the one most kernels are better
	// at: a hub with no Wi-Fi driver at all usually still has these.
	Ethernet []string

	// Modules says whether this kernel can load a driver that was not built
	// into it, which decides whether adding one is a project or an
	// impossibility. A kernel built without module support has no /proc/modules
	// at all, and nothing can be added to it short of reflashing.
	Modules bool
	Insmod  bool
}

// Ready reports whether the hub has a second wireless interface to join a
// network with.
func (a Adapter) Ready() bool { return a.Extra != "" }

// ProbeAdapter asks the hub what it has.
func ProbeAdapter(serial string) (Adapter, error) {
	var a Adapter

	names, err := adb.Shell(serial, "ls", "/sys/class/net", "2>/dev/null", "||", "true")
	if err != nil {
		return a, fmt.Errorf("cannot ask the robot what interfaces it has: %w", err)
	}

	for _, name := range strings.Fields(names) {
		if name == "lo" {
			continue
		}

		iface := Iface{Name: name}

		iface.Wireless = isWireless(serial, name)
		if iface.Wireless {
			iface.Phy = phyOf(serial, name)
		}
		if out, err := adb.Shell(serial, "cat", "/sys/class/net/"+name+"/operstate", "2>/dev/null", "||", "true"); err == nil {
			iface.Up = strings.Contains(strings.TrimSpace(out), "up")
		}
		iface.Address = addressOf(serial, name)

		a.Ifaces = append(a.Ifaces, iface)
	}

	sort.Slice(a.Ifaces, func(i, j int) bool { return a.Ifaces[i].Name < a.Ifaces[j].Name })

	a.USB = usbDevices(serial)

	if out, err := adb.Shell(serial, "ls", "/system/bin/wpa_supplicant", "2>/dev/null", "||", "true"); err == nil {
		a.Supplicant = strings.Contains(out, "wpa_supplicant")
	}

	if out, err := adb.Shell(serial, "ls", "/proc/modules", "2>/dev/null", "||", "true"); err == nil {
		a.Modules = strings.Contains(out, "/proc/modules")
	}
	if out, err := adb.Shell(serial, "ls", "/system/bin/insmod", "/sbin/insmod", "2>/dev/null", "||", "true"); err == nil {
		a.Insmod = strings.Contains(out, "insmod")
	}

	a.Drivers = match(serial, usbWifiDrivers)
	a.Ethernet = match(serial, usbEthernetDrivers)

	a.Extra = extraRadio(a.Ifaces)

	return a, nil
}

// extraRadio is a wireless interface on a radio other than the one running the
// access point, which is what a plugged in adapter would be.
//
// By radio rather than by interface. A Control Hub has wlan0 and p2p0, and they
// are one piece of hardware: p2p0 is Wi-Fi Direct on the same chip. Counting
// interfaces found a second one on every hub and reported an adapter that was
// not there, which is worse than missing one that is.
func extraRadio(ifaces []Iface) string {
	ap := ""
	for _, iface := range ifaces {
		// The interface holding an address is the one serving the access point.
		if iface.Wireless && iface.Address != "" {
			ap = iface.Phy
			break
		}
	}
	if ap == "" {
		for _, iface := range ifaces {
			if iface.Wireless && strings.HasPrefix(iface.Name, "wlan") {
				ap = iface.Phy
				break
			}
		}
	}

	for _, iface := range ifaces {
		if !iface.Wireless || strings.HasPrefix(iface.Name, "p2p") {
			continue
		}

		// A kernel too old to report a phy leaves this to the name, where
		// anything past wlan0 is something that was plugged in.
		if iface.Phy == "" {
			if iface.Name != "wlan0" && strings.HasPrefix(iface.Name, "wlan") {
				return iface.Name
			}
			continue
		}

		if iface.Phy != ap {
			return iface.Name
		}
	}

	return ""
}

// phyOf is the radio an interface belongs to.
func phyOf(serial, name string) string {
	out, err := adb.Shell(serial, "cat", "/sys/class/net/"+name+"/phy80211/name", "2>/dev/null", "||", "true")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// isWireless reports whether an interface is a radio.
//
// Two directories, not one. A driver on the old wireless extensions has
// `wireless`, and one on cfg80211, which is everything modern, has `phy80211`
// instead. Asking only for the first called a Control Hub's own wlan0 wired and
// then concluded the hub had no radio at all, on a hub that is an access point.
func isWireless(serial, name string) bool {
	for _, marker := range []string{"wireless", "phy80211"} {
		out, err := adb.Shell(serial, "ls", "-d", "/sys/class/net/"+name+"/"+marker, "2>/dev/null", "||", "true")
		if err == nil && strings.Contains(out, marker) {
			return true
		}
	}

	// Last resort, for a driver that exposes neither. p2p0 is Wi-Fi Direct and
	// belongs to the same radio as wlan0 rather than being one of its own.
	return strings.HasPrefix(name, "wlan")
}

func addressOf(serial, iface string) string {
	out, err := adb.Shell(serial, "ip", "-4", "-o", "addr", "show", iface, "2>/dev/null", "||", "true")
	if err != nil {
		return ""
	}

	if match := ipLine.FindStringSubmatch(out); match != nil {
		return match[1]
	}
	return ""
}

// usbDevices is what is plugged into the hub, read out of sysfs because Android
// has no lsusb worth relying on.
func usbDevices(serial string) []USBDevice {
	out, err := adb.Shell(serial, "ls", "/sys/bus/usb/devices", "2>/dev/null", "||", "true")
	if err != nil {
		return nil
	}

	var devices []USBDevice
	for _, entry := range strings.Fields(out) {
		// Interfaces rather than devices, which are the same hardware listed
		// again by function.
		if strings.Contains(entry, ":") || strings.HasPrefix(entry, "usb") {
			continue
		}

		base := "/sys/bus/usb/devices/" + entry
		device := USBDevice{
			Vendor:  read(serial, base+"/idVendor"),
			Product: read(serial, base+"/idProduct"),
			Name:    read(serial, base+"/product"),
		}

		device.Class = interfaceClass(serial, base, entry)

		if device.Vendor == "" && device.Name == "" {
			continue
		}
		devices = append(devices, device)
	}

	return devices
}

// interfaceClass is what the device's first interface claims to be.
func interfaceClass(serial, base, entry string) string {
	iface := base + "/" + entry + ":1.0"

	class := read(serial, iface+"/bInterfaceClass")
	if class == "" {
		return ""
	}

	return class + "/" + read(serial, iface+"/bInterfaceSubClass") +
		"/" + read(serial, iface+"/bInterfaceProtocol")
}

// Kernel is what this hub is running, which is what a driver would have to be
// built against.
func Kernel(serial string) string {
	if out, err := adb.Shell(serial, "cat", "/proc/version", "2>/dev/null", "||", "true"); err == nil {
		return strings.TrimSpace(out)
	}
	return ""
}

// OSVersion is the Control Hub OS release, which names the tag in REV's kernel
// repository that this hub was built from.
func OSVersion(serial string) string {
	for _, prop := range []string{"ro.controlhub.os.version", "ro.build.display.id", "ro.build.version.incremental"} {
		if out, err := adb.Shell(serial, "getprop", prop); err == nil {
			if text := strings.TrimSpace(out); text != "" {
				return text
			}
		}
	}
	return ""
}

// Dmesg is what the kernel said, filtered to the lines about USB and radios.
//
// The last word on why an adapter did not come up. Enumerated and unclaimed,
// claimed and then failed, or never seen at all are three different problems
// with three different answers, and only the kernel's own log tells them apart.
func Dmesg(serial string) []string {
	out, err := adb.Shell(serial, "dmesg", "2>/dev/null", "||", "true")
	if err != nil {
		return nil
	}

	var kept []string
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)

		switch {
		case strings.Contains(lower, "usb"), strings.Contains(lower, "rtl"),
			strings.Contains(lower, "8821"), strings.Contains(lower, "8812"),
			strings.Contains(lower, "8822"), strings.Contains(lower, "wlan"),
			strings.Contains(lower, "rtw"):
		default:
			continue
		}

		if text := strings.TrimSpace(line); text != "" {
			kept = append(kept, text)
		}
	}

	// The end of it, which is where a freshly plugged in adapter shows up.
	const most = 40
	if len(kept) > most {
		kept = kept[len(kept)-most:]
	}

	return kept
}

func read(serial, path string) string {
	out, err := adb.Shell(serial, "cat", path, "2>/dev/null", "||", "true")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Explain says what the hub's answer means, which is the part somebody actually
// wants: whether plugging an adapter in got them anywhere.
func (a Adapter) Explain() []string {
	var out []string

	if a.Ready() {
		out = append(out,
			fmt.Sprintf("%s is a second wireless interface, so the kernel has a driver for it.", a.Extra),
			"That is the hard part, and it is done. Joining a network with it is next.")
		if !a.Supplicant {
			out = append(out, "There is no wpa_supplicant on this hub, though, so nothing here can join a protected network.")
		}
		return out
	}

	radios := map[string]bool{}
	wireless := 0
	for _, iface := range a.Ifaces {
		if !iface.Wireless || strings.HasPrefix(iface.Name, "p2p") {
			continue
		}
		wireless++
		radios[iface.Phy] = true
	}

	switch {
	case wireless == 0:
		out = append(out, "The hub reports no wireless interface at all, which is not what a Control Hub looks like.",
			"Worth checking pusher is talking to the robot and not something else.")

	case len(a.USB) == 0:
		out = append(out, "One wireless interface, which is the access point, and nothing on the USB bus.",
			"If an adapter is plugged in, the hub has not enumerated it: try the other port, or a powered hub.")

	default:
		out = append(out, "One wireless interface, which is the access point the Driver Station uses.",
			"Something is on the USB bus but no driver claimed it as a network interface, so this",
			"hub's kernel has no driver for that chipset. Nothing on this laptop can add one:",
			"the driver would have to be in the kernel REV shipped.")

		if len(a.Drivers) > 0 {
			out = append(out, "",
				"It does have Wi-Fi drivers for these, so an adapter with one of these chipsets",
				"would come up as a second interface. Yours did not, which is how you know",
				"neither of them is one:")
			for _, driver := range a.Drivers {
				out = append(out, "  "+driver)
			}
		}

		if len(a.Ethernet) > 0 {
			out = append(out, "",
				"And it has these for USB Ethernet, which is the cabled way onto a network and",
				"the better bet: the driver list for it is far longer, so almost any adapter works.")
			for _, driver := range a.Ethernet {
				out = append(out, "  "+driver)
			}
		}

		out = append(out, "", loadable(a))

		if len(a.Drivers) == 0 && len(a.Ethernet) == 0 {
			out = append(out, "", "It would not say which drivers it has either, so the only way to find one",
				"that works is to try it and run this again.")
		}
	}

	return out
}

// usbWifiDrivers maps what a kernel calls a driver to what somebody buying an
// adapter would recognise.
//
// Only the USB ones. A hub's own radio is on SDIO and its driver says nothing
// about what can be plugged into a port.
var usbWifiDrivers = []struct {
	// Match is every name one driver goes by: the kernel config symbol, and
	// the directory it registers itself under, which are rarely the same word.
	Match []string
	Name  string
}{
	{[]string{"RTL8188EU", "R8188EU"}, "Realtek RTL8188EU (2.4GHz n, the common cheap nano adapters)"},
	{[]string{"RTL8192CU", "RTL8192CU"}, "Realtek RTL8192CU (2.4GHz n)"},
	{[]string{"RTL8XXXU"}, "Realtek RTL8188/8192/8723 family (2.4GHz n)"},
	{[]string{"RTL8723AU", "R8723AU"}, "Realtek RTL8723AU (2.4GHz n)"},
	{[]string{"RTL8723BU", "R8723BU"}, "Realtek RTL8723BU (2.4GHz n)"},
	{[]string{"RTL8812AU", "88XXAU"}, "Realtek RTL8812AU (802.11ac)"},
	{[]string{"RTL8821AU"}, "Realtek RTL8821AU (802.11ac)"},
	{[]string{"RTL8821CU", "RTL8811CU"}, "Realtek RTL8811CU / RTL8821CU (802.11ac, the AC600 dongles)"},
	{[]string{"RTL8812BU", "RTL8822BU"}, "Realtek RTL8812BU / RTL8822BU (802.11ac)"},
	{[]string{"RTL8822BU"}, "Realtek RTL8822BU (802.11ac)"},
	{[]string{"RTL8187"}, "Realtek RTL8187 (2.4GHz g)"},
	{[]string{"MT7601U"}, "MediaTek MT7601U (2.4GHz n)"},
	{[]string{"MT76"}, "MediaTek MT76 family"},
	{[]string{"RT2800USB"}, "Ralink RT2800 family (2.4GHz n, RT5370 and friends)"},
	{[]string{"RT73USB"}, "Ralink RT73 (2.4GHz g)"},
	{[]string{"ATH9K_HTC"}, "Atheros AR9271 / AR7010 (2.4GHz n)"},
	{[]string{"CARL9170"}, "Atheros AR9170 (2.4GHz n)"},
	{[]string{"ZD1211RW"}, "ZyDAS ZD1211 (2.4GHz g)"},
	{[]string{"AR5523"}, "Atheros AR5523"},
}

// usbEthernetDrivers is the cabled half.
var usbEthernetDrivers = []struct {
	Match []string
	Name  string
}{
	{[]string{"AX88179_178A"}, "ASIX AX88179 / AX88178A (gigabit, most USB 3 adapters)"},
	{[]string{"ASIX"}, "ASIX AX88772 and friends (100Mbit, most cheap adapters)"},
	{[]string{"SMSC95XX"}, "SMSC LAN95xx (100Mbit)"},
	{[]string{"SMSC75XX"}, "SMSC LAN75xx (gigabit)"},
	{[]string{"RTL8150"}, "Realtek RTL8150 (100Mbit)"},
	{[]string{"R8152", "RTL8152"}, "Realtek RTL8152 / RTL8153 (gigabit)"},
	{[]string{"DM9601", "DM9620"}, "Davicom DM96xx (100Mbit)"},
	{[]string{"PEGASUS"}, "ADMtek Pegasus (100Mbit)"},
	{[]string{"CDC_ETHER"}, "CDC Ethernet, the standard class most adapters also speak"},
	{[]string{"CDC_NCM"}, "CDC NCM"},
	{[]string{"RNDIS_HOST"}, "RNDIS, which is how a phone tethers over USB"},
}

// match is which chipsets from a table this kernel can drive.
//
// Asked of the kernel three ways because no one of them is always there: the
// build config when it was kept, loadable modules when the kernel has any, and
// what is loaded right now. A hub that answers none of them is not one this can
// say anything about, which is worth saying rather than guessing from.
func match(serial string, table []struct {
	Match []string
	Name  string
}) []string {
	var haystack strings.Builder

	// The build config, when the kernel kept a copy of it. This is the whole
	// answer when it is there: everything compiled in, not only what is loaded.
	if out, err := adb.Shell(serial, "zcat", "/proc/config.gz", "2>/dev/null",
		"||", "gunzip", "-c", "/proc/config.gz", "2>/dev/null", "||", "true"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "CONFIG_") && (strings.HasSuffix(line, "=y") || strings.HasSuffix(line, "=m")) {
				haystack.WriteString(line + "\n")
			}
		}
	}

	// Every USB driver the kernel has registered, built in or not. This is the
	// one source that is always there: a driver compiled into the kernel never
	// appears as a module and leaves no config behind, but it still registers
	// itself on the bus at boot, and that registration is a directory.
	if out, err := adb.Shell(serial, "ls", "/sys/bus/usb/drivers", "2>/dev/null", "||", "true"); err == nil {
		haystack.WriteString(out + "\n")
	}

	for _, dir := range []string{"/system/lib/modules", "/vendor/lib/modules", "/lib/modules"} {
		if out, err := adb.Shell(serial, "ls", dir, "2>/dev/null", "||", "true"); err == nil {
			haystack.WriteString(out + "\n")
		}
	}

	if out, err := adb.Shell(serial, "cat", "/proc/modules", "2>/dev/null", "||", "true"); err == nil {
		haystack.WriteString(out + "\n")
	}

	text := strings.ToUpper(haystack.String())

	var found []string
	for _, driver := range table {
		for _, name := range driver.Match {
			if strings.Contains(text, name) {
				found = append(found, driver.Name)
				break
			}
		}
	}

	return found
}

// Registered is every USB driver the kernel has, so a chipset this does not
// have a name for is still visible rather than silently absent.
func Registered(serial string) []string {
	out, err := adb.Shell(serial, "ls", "/sys/bus/usb/drivers", "2>/dev/null", "||", "true")
	if err != nil {
		return nil
	}

	names := strings.Fields(out)
	sort.Strings(names)
	return names
}

// loadable says whether a driver could be added to this kernel at all.
//
// The question everybody asks next, and it has a real answer. A kernel built
// without module support cannot be given one: there is nothing to load it with
// and no interface to load it through. A kernel that can load modules could in
// principle, but the module has to be compiled against that exact kernel, from
// its source and its config, with a matching toolchain, and it has to carry the
// same vermagic or the kernel refuses it.
func loadable(a Adapter) string {
	if !a.Modules {
		return "This kernel has no module support at all, so a driver cannot be added to it. " +
			"Everything it can drive was compiled in when it was built."
	}
	if !a.Insmod {
		return "This kernel can hold modules but has no insmod to load them with."
	}
	return "This kernel can load modules, so a driver could in principle be built for it. " +
		"It would have to be compiled against this exact kernel's source and config."
}
