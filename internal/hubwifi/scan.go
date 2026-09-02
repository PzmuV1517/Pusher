package hubwifi

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Choosing a network by typing its name is choosing blind: a venue's network is
// something somebody read off a whiteboard, and the robot is the only thing in
// the room that can say what it can actually hear from where it is sitting.
//
// So the robot does the looking. The adapter has to be up for it, which means
// the driver has to be loaded, which is the same work as joining and is done
// the same way.

// AP is a network the robot can see.
type AP struct {
	SSID   string
	BSSID  string
	Signal int

	// Band is 2.4 or 5, worked out from the frequency, because which one a
	// robot can see is often the whole question.
	Band string

	// Secured is false for an open network, which is worth showing rather than
	// discovering when the passphrase is refused.
	Secured bool
}

// Bars is the signal as something readable at a glance.
func (a AP) Bars() string {
	switch {
	case a.Signal >= -55:
		return "••••"
	case a.Signal >= -67:
		return "•••"
	case a.Signal >= -78:
		return "••"
	default:
		return "•"
	}
}

// Scan asks the robot what it can hear.
func Scan(serial string) ([]AP, error) {
	prefix, err := rootPrefix(serial)
	if err != nil {
		return nil, err
	}
	r := runner{serial: serial, root: prefix}

	if !r.hasIface() {
		return nil, errNoAdapter
	}

	r.quiet("ip", "link", "set", Iface, "up")

	// wpa_supplicant owns the radio when it is running, and its scan results
	// are the ones to ask for. Without it, iw can scan directly.
	if r.supplicants() > 0 {
		r.quiet("wpa_cli", "-p", CtrlDir, "-i", Iface, "scan")
		time.Sleep(4 * time.Second)

		out, err := r.run("wpa_cli", "-p", CtrlDir, "-i", Iface, "scan_results")
		if err == nil && strings.Contains(out, "/ ssid") {
			return parseWpaScan(out), nil
		}
	}

	out, err := r.run("iw", "dev", Iface, "scan")
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, errScanFailed
	}
	return parseIwScan(out), nil
}

// parseWpaScan reads wpa_cli's table: bssid, frequency, signal, flags, ssid.
func parseWpaScan(out string) []AP {
	var found []AP

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 5 || !strings.Contains(fields[0], ":") {
			continue
		}

		signal, _ := strconv.Atoi(fields[2])
		freq, _ := strconv.Atoi(fields[1])

		found = append(found, AP{
			SSID:    fields[4],
			BSSID:   fields[0],
			Signal:  signal,
			Band:    bandOf(freq),
			Secured: strings.Contains(fields[3], "WPA") || strings.Contains(fields[3], "WEP"),
		})
	}

	return tidy(found)
}

// parseIwScan reads iw's paragraphs, which are the fallback when no supplicant
// is running to ask.
func parseIwScan(out string) []AP {
	var found []AP
	var current AP

	flush := func() {
		if current.BSSID != "" {
			found = append(found, current)
		}
		current = AP{}
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "BSS "):
			flush()
			current.BSSID = strings.Fields(strings.TrimPrefix(trimmed, "BSS "))[0]
			if i := strings.Index(current.BSSID, "("); i > 0 {
				current.BSSID = current.BSSID[:i]
			}

		case strings.HasPrefix(trimmed, "SSID: "):
			current.SSID = strings.TrimPrefix(trimmed, "SSID: ")

		case strings.HasPrefix(trimmed, "signal: "):
			value := strings.Fields(strings.TrimPrefix(trimmed, "signal: "))
			if len(value) > 0 {
				if f, err := strconv.ParseFloat(value[0], 64); err == nil {
					current.Signal = int(f)
				}
			}

		case strings.HasPrefix(trimmed, "freq: "):
			if freq, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "freq: "))); err == nil {
				current.Band = bandOf(freq)
			}

		case strings.Contains(trimmed, "RSN:"), strings.Contains(trimmed, "WPA:"):
			current.Secured = true
		}
	}
	flush()

	return tidy(found)
}

func bandOf(freq int) string {
	switch {
	case freq >= 4900:
		return "5GHz"
	case freq > 0:
		return "2.4GHz"
	}
	return ""
}

// tidy drops the nameless, keeps the strongest of each name, and sorts by
// signal.
//
// One network is several access points, and a list with the same name six times
// is a list nobody can choose from. A hidden network has no name to choose by
// at all.
func tidy(found []AP) []AP {
	best := map[string]AP{}

	for _, ap := range found {
		name := strings.TrimSpace(ap.SSID)
		if name == "" || strings.HasPrefix(name, `\x00`) {
			continue
		}
		ap.SSID = name

		if seen, ok := best[name]; !ok || ap.Signal > seen.Signal {
			best[name] = ap
		}
	}

	out := make([]AP, 0, len(best))
	for _, ap := range best {
		out = append(out, ap)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Signal > out[j].Signal })
	return out
}
