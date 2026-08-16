//go:build darwin && cgo

// A name-filtered Wi-Fi scan.
//
// macOS 15 stopped telling command-line tools the name of any network: CWNetwork
// hands back a nil ssid unless the app that spawned the process holds Location
// Services. The filter passed to scanForNetworksWithName: is applied inside the
// OS, before that redaction, so asking "is this exact network nearby?" still
// gets a true answer. Counting the results is all pusher needs, and it needs no
// permission at all.

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>

#include <string.h>

static void copyError(char *buf, int len, NSString *message) {
    if (buf == NULL || len <= 0) {
        return;
    }
    const char *text = [message UTF8String];
    if (text == NULL) {
        text = "unknown error";
    }
    strncpy(buf, text, (size_t)len - 1);
    buf[len - 1] = '\0';
}

// pusherScanFor counts the networks nearby whose name is ssid.
//
// Returns 0 on success with the count written to found, or non-zero with a
// message in err.
int pusherScanFor(const char *ifaceName, const char *ssid, int *found, char *err, int errlen) {
    @autoreleasepool {
        if (ssid == NULL || ssid[0] == '\0') {
            copyError(err, errlen, @"no network name given");
            return 1;
        }

        CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
        CWInterface *iface = nil;
        if (ifaceName != NULL && ifaceName[0] != '\0') {
            iface = [client interfaceWithName:[NSString stringWithUTF8String:ifaceName]];
        }
        if (iface == nil) {
            iface = [client interface];
        }
        if (iface == nil) {
            copyError(err, errlen, @"no Wi-Fi interface");
            return 1;
        }

        NSError *scanErr = nil;
        NSSet<CWNetwork *> *networks =
            [iface scanForNetworksWithName:[NSString stringWithUTF8String:ssid]
                             includeHidden:YES
                                     error:&scanErr];

        if (scanErr != nil) {
            copyError(err, errlen, [scanErr localizedDescription]);
            return 1;
        }

        if (found != NULL) {
            *found = (int)[networks count];
        }
        return 0;
    }
}
