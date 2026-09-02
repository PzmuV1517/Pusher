package hubwifi

import "testing"

// Real wpa_cli output, tabs and all.
const wpaScan = "bssid / frequency / signal level / flags / ssid\n" +
	"e8:9c:25:4f:be:b4\t5500\t-62\t[WPA2-PSK-CCMP][WPS][ESS]\tASUS_5G\n" +
	"e8:9c:25:4f:be:b0\t2457\t-63\t[WPA2-PSK-CCMP][WPS][ESS]\tASUS\n" +
	"50:64:2b:8b:01:9b\t2457\t-64\t[WPA-PSK-CCMP+TKIP][ESS]\tASUS\n" +
	"2c:c8:1b:86:74:39\t2447\t-30\t[WPA2-PSK-CCMP][ESS]\tMikroTik-CRS109\n" +
	"aa:bb:cc:dd:ee:ff\t2412\t-70\t[ESS]\tGuest\n" +
	"6a:5a:b0:78:d0:79\t2437\t-86\t[WPA2-PSK-CCMP][ESS]\t\\x00\\x00\\x00\n"

func TestAScanIsSomethingYouCanChooseFrom(t *testing.T) {
	found := parseWpaScan(wpaScan)

	if len(found) == 0 {
		t.Fatal("read nothing out of a real scan")
	}

	// Strongest first, or the one you want is somewhere down a list.
	if found[0].SSID != "MikroTik-CRS109" {
		t.Errorf("strongest is %q, want MikroTik-CRS109", found[0].SSID)
	}

	names := map[string]int{}
	for _, ap := range found {
		names[ap.SSID]++
	}

	// One network is several access points, and the same name six times is a
	// list nobody can choose from.
	if names["ASUS"] != 1 {
		t.Errorf("ASUS appears %d times, want once", names["ASUS"])
	}

	// A hidden network has no name to choose by.
	for _, ap := range found {
		if ap.SSID == "" || ap.SSID[0] == '\\' {
			t.Errorf("kept a nameless network: %q", ap.SSID)
		}
	}
}

// Which band a robot can hear is often the whole question, and an open network
// is worth knowing before the passphrase is refused.
func TestABandAndALockAreReported(t *testing.T) {
	byName := map[string]AP{}
	for _, ap := range parseWpaScan(wpaScan) {
		byName[ap.SSID] = ap
	}

	if got := byName["ASUS_5G"].Band; got != "5GHz" {
		t.Errorf("ASUS_5G is on %q", got)
	}
	if got := byName["ASUS"].Band; got != "2.4GHz" {
		t.Errorf("ASUS is on %q", got)
	}
	if !byName["ASUS"].Secured {
		t.Error("a WPA2 network is not marked as secured")
	}
	if byName["Guest"].Secured {
		t.Error("an open network is marked as secured")
	}
}

// iw is the fallback when no supplicant is running to ask.
func TestTheFallbackScannerIsReadToo(t *testing.T) {
	const iwScan = `BSS e4:c3:2a:00:c5:e2(on wlan1)
	TSF: 922762897 usec
	freq: 2417
	signal: -53.00 dBm
	SSID: TP-Link_C5E2
	RSN:	 * Version: 1
BSS aa:bb:cc:dd:ee:00(on wlan1)
	freq: 5200
	signal: -31.00 dBm
	SSID: FTC-9RbP
`
	found := parseIwScan(iwScan)
	if len(found) != 2 {
		t.Fatalf("read %d networks, want 2", len(found))
	}

	if found[0].SSID != "FTC-9RbP" || found[0].Band != "5GHz" {
		t.Errorf("strongest is %+v", found[0])
	}
	if !found[1].Secured {
		t.Error("an RSN network is not marked as secured")
	}
}
