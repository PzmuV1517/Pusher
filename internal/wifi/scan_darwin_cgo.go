//go:build darwin && cgo

package wifi

/*
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation
#include <stdlib.h>

int pusherScanFor(const char *ifaceName, const char *ssid, int *found, char *err, int errlen);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const scanSupported = true

// ScanNote explains what pusher does about networks it cannot see yet.
const ScanNote = `pusher looks for the robot's network in the background while the project
builds, so the hub is already in range by the time it tries to join. The scan is
filtered by name inside macOS, so it works without Location Services even though
the network names come back hidden.

Turn it off with PUSHER_NO_SCAN=1 if scanning upsets your Wi-Fi.`

func (m *Manager) visible(ssid string) (bool, error) {
	iface := C.CString(m.wifiInterface())
	defer C.free(unsafe.Pointer(iface))

	name := C.CString(ssid)
	defer C.free(unsafe.Pointer(name))

	var found C.int
	errbuf := make([]C.char, 256)

	if rc := C.pusherScanFor(iface, name, &found, &errbuf[0], C.int(len(errbuf))); rc != 0 {
		return false, fmt.Errorf("could not scan for %q: %s", ssid, C.GoString(&errbuf[0]))
	}

	return found > 0, nil
}
