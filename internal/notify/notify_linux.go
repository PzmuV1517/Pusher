//go:build linux

package notify

import "os/exec"

const supported = true

// notify-send is part of libnotify and present on any desktop that has a
// notification daemon to receive this. Its absence means nowhere to deliver to,
// which is not a failure.
func send(title, body string) bool {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return false
	}
	return exec.Command("notify-send", "--app-name=Pusher", title, body).Run() == nil
}
