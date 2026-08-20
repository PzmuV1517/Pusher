//go:build !darwin && !linux && !windows

package notify

const supported = false

func send(title, body string) bool { return false }
