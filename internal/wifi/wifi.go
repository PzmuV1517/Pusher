//go:build darwin

package wifi

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

// cwCurrentSSID returns the current Wi-Fi SSID as a newly allocated
// C string (or NULL if not associated). Caller must free().
const char* cwCurrentSSID() {
	@autoreleasepool {
		CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
		CWInterface *iface = [client interface];
		if (!iface) {
			return NULL;
		}

		NSString *ssid = iface.ssid;
        if (!ssid) {
            return NULL;
        }

        const char *utf8 = [ssid UTF8String];
        if (!utf8) {
            return NULL;
        }

        return strdup(utf8);
    }
}

// cwConnectNetwork attempts to associate to the given SSID with an
// optional password. On error it returns a non-zero code and sets
// *errOut to a newly allocated error message.
int cwConnectNetwork(const char* cssid, const char* cpassword, char** errOut) {
    @autoreleasepool {
        if (!cssid) {
            *errOut = strdup("SSID is nil");
            return 1;
        }

		CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
		CWInterface *iface = [client interface];
		if (!iface) {
			*errOut = strdup("No Wi-Fi interface found");
			return 2;
		}

		NSString *ssid = [NSString stringWithUTF8String:cssid];
        if (!ssid) {
            *errOut = strdup("Invalid SSID");
            return 3;
        }

        NSError *error = nil;
		NSSet<CWNetwork *> *nets = [iface scanForNetworksWithName:ssid error:&error];
        if (!nets || [nets count] == 0) {
            if (error) {
                const char *msg = [[error localizedDescription] UTF8String];
                *errOut = strdup(msg ? msg : "Network not found");
            } else {
                *errOut = strdup("Network not found");
            }
            return 4;
        }

        CWNetwork *target = [nets anyObject];

        NSString *password = nil;
        if (cpassword && strlen(cpassword) > 0) {
            password = [NSString stringWithUTF8String:cpassword];
        }

        NSError *assocError = nil;
		BOOL ok = [iface associateToNetwork:target password:password error:&assocError];
		if (!ok) {
			if (assocError) {
				NSString *full = [NSString stringWithFormat:@"Association failed (%@/%ld): %@",
								   assocError.domain,
								   (long)assocError.code,
								   assocError.localizedDescription ?: @"<no description>"];
				const char *msg = [full UTF8String];
				*errOut = strdup(msg ? msg : "Association failed");
			} else {
				*errOut = strdup("Association failed");
			}
			return 5;
		}

		return 0;
    }
}
*/
import "C"

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"unsafe"
)

// Manager handles Wi-Fi operations
type Manager struct {
	iface string
}

// NewManager creates a new Wi-Fi manager
func NewManager() *Manager {
	// Default to en0 for macOS; CoreWLAN uses the default interface
	return &Manager{
		iface: "en0",
	}
}

// GetCurrentSSID returns the currently connected Wi-Fi SSID
func (m *Manager) GetCurrentSSID() (string, error) {
	ssidC := C.cwCurrentSSID()
	if ssidC == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(ssidC))

	return C.GoString(ssidC), nil
}

// Connect connects to a Wi-Fi network using CoreWLAN
func (m *Manager) Connect(ssid, password string) error {
	fmt.Printf("[*] Connecting to %s...\n", ssid)

	cssid := C.CString(ssid)
	defer C.free(unsafe.Pointer(cssid))

	cpassword := C.CString(password)
	defer C.free(unsafe.Pointer(cpassword))

	var errOut *C.char
	code := C.cwConnectNetwork(cssid, cpassword, &errOut)
	if code != 0 {
		var msg string
		if errOut != nil {
			msg = C.GoString(errOut)
			C.free(unsafe.Pointer(errOut))
		} else {
			msg = "unknown error"
		}

		// CoreWLAN association failed; fall back silently to networksetup.
		// We keep the CoreWLAN error text in `msg` so that if the
		// networksetup fallback also fails, the combined error still
		// contains useful diagnostics.
		args := []string{"-setairportnetwork", m.iface, ssid}
		if password != "" {
			args = append(args, password)
		}
		cmd := exec.Command("networksetup", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to connect to %s via CoreWLAN (code=%d, %s) and networksetup: %w (output: %s)", ssid, int(code), msg, err, string(output))
		}
	}

	fmt.Println("[OK] Connected to", ssid)
	return nil
}

// ConnectWithRetry is kept for compatibility but just calls Connect once
func (m *Manager) ConnectWithRetry(ssid, password string, maxRetries int) error {
	return m.Connect(ssid, password)
}

// GetIPv4 returns the IPv4 address of the Wi-Fi interface (e.g., en0)
func (m *Manager) GetIPv4() (string, error) {
	iface, err := net.InterfaceByName(m.iface)
	if err != nil {
		return "", fmt.Errorf("failed to get interface %s: %w", m.iface, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to get addresses for %s: %w", m.iface, err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ip4 := ipNet.IP.To4()
			if ip4 != nil {
				return ip4.String(), nil
			}
		}
	}

	return "", nil
}

// IsOnRobotNetwork reports whether the Wi-Fi IPv4 is in the FTC robot subnet
// Current FTC default robot controller hotspot uses 192.168.43.x
func (m *Manager) IsOnRobotNetwork() (bool, error) {
	ip, err := m.GetIPv4()
	if err != nil {
		return false, err
	}
	if ip == "" {
		return false, nil
	}
	return strings.HasPrefix(ip, "192.168.43."), nil
}

// IsConnected checks if connected to a specific SSID
func (m *Manager) IsConnected(ssid string) (bool, error) {
	current, err := m.GetCurrentSSID()
	if err != nil {
		return false, err
	}
	return current == ssid, nil
}
