package wifi

import (
	"encoding/xml"
	"sort"
	"strconv"
	"strings"
)

func firstNotIn(networks, exclude []string) string {
	skip := make(map[string]bool, len(exclude))
	for _, ssid := range exclude {
		if ssid != "" {
			skip[ssid] = true
		}
	}

	for _, ssid := range networks {
		if !skip[ssid] {
			return ssid
		}
	}

	return ""
}

func parseNetworksetupSSID(output string) string {
	_, ssid, found := strings.Cut(strings.TrimSpace(output), ":")
	if !found {
		return ""
	}
	return strings.TrimSpace(ssid)
}

func isRedacted(ssid string) bool {
	lower := strings.ToLower(ssid)
	return strings.Contains(lower, "redacted") ||
		strings.Contains(lower, "not associated") ||
		lower == "<unknown>"
}

func parseDarwinPreferred(output string) []string {
	var networks []string
	for _, line := range strings.Split(output, "\n") {

		if !strings.HasPrefix(line, "\t") {
			continue
		}
		if name := strings.TrimSpace(line); name != "" {
			networks = append(networks, name)
		}
	}
	return networks
}

func splitTerse(line string) []string {
	var (
		fields  []string
		current strings.Builder
		escaped bool
	)

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	return append(fields, current.String())
}

func parseNmcliWiFiDevice(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[1] == "wifi" && fields[0] != "" {
			return fields[0]
		}
	}
	return ""
}

func parseNmcliActiveSSID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "yes" {
			return fields[1]
		}
	}
	return ""
}

func parseNmcliSavedNetworks(output string) []string {
	type profile struct {
		name string
		used int64
	}

	var profiles []profile
	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimSpace(line))
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		if !strings.Contains(fields[1], "wireless") {
			continue
		}

		used, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil {

			used = 0
		}
		profiles = append(profiles, profile{name: fields[0], used: used})
	}

	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].used > profiles[j].used
	})

	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.name)
	}
	return names
}

func parseNmcliRadio(output string) bool {
	return strings.EqualFold(strings.TrimSpace(output), "enabled")
}

// nmcliSawSSID reports whether a scan listed a network.
//
// One SSID per line, and an empty line for every hidden network, so the exact
// match matters: a hidden network would otherwise match an empty search.
func nmcliSawSSID(output, ssid string) bool {
	if ssid == "" {
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimRight(line, "\r"))
		if len(fields) > 0 && fields[0] == ssid {
			return true
		}
	}

	return false
}

// netshSawSSID reports whether a scan listed a network.
//
// The lines are numbered, as in "SSID 3 : Robot", so the name is whatever
// follows the first colon.
func netshSawSSID(output, ssid string) bool {
	if ssid == "" {
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.HasPrefix(trimmed, "SSID ") {
			continue
		}

		_, value, found := strings.Cut(trimmed, ":")
		if found && strings.TrimSpace(value) == ssid {
			return true
		}
	}

	return false
}

func netshField(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		if strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseNetshProfiles(output string) []string {
	var networks []string

	for _, line := range strings.Split(output, "\n") {

		if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, " ") {
			continue
		}

		_, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		name := strings.TrimSpace(value)
		if name == "" || name == "<None>" {
			continue
		}
		networks = append(networks, name)
	}

	return networks
}

func wlanProfileXML(ssid, password string) (string, error) {
	name, err := xmlEscape(ssid)
	if err != nil {
		return "", err
	}
	key, err := xmlEscape(password)
	if err != nil {
		return "", err
	}

	return `<?xml version="1.0"?>
<WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1">
  <name>` + name + `</name>
  <SSIDConfig>
    <SSID>
      <name>` + name + `</name>
    </SSID>
  </SSIDConfig>
  <connectionType>ESS</connectionType>
  <connectionMode>manual</connectionMode>
  <MSM>
    <security>
      <authEncryption>
        <authentication>WPA2PSK</authentication>
        <encryption>AES</encryption>
        <useOneX>false</useOneX>
      </authEncryption>
      <sharedKey>
        <keyType>passPhrase</keyType>
        <protected>false</protected>
        <keyMaterial>` + key + `</keyMaterial>
      </sharedKey>
    </security>
  </MSM>
</WLANProfile>`, nil
}

func xmlEscape(value string) (string, error) {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return "", err
	}
	return b.String(), nil
}

func escapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
