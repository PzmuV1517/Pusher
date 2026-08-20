// Package notify puts a message in front of somebody who is not looking at the
// terminal.
//
// Everything here is best effort. A notification that cannot be delivered is
// not an error worth reporting: the thing it was announcing has not failed, and
// the message is a courtesy rather than a result.
package notify

import "os"

// Title is what every notification pusher sends is called.
const Title = "Pusher"

// Enabled reports whether notifications will be attempted at all.
func Enabled() bool {
	if os.Getenv("PUSHER_NO_NOTIFY") != "" {
		return false
	}
	return supported
}

// Send shows a desktop notification, and reports whether it went anywhere.
//
// The bool is for tests and for deciding whether something still needs saying
// in the terminal. Callers that simply want to try can ignore it.
func Send(title, body string) bool {
	if !Enabled() || body == "" {
		return false
	}
	if title == "" {
		title = Title
	}
	return send(title, body)
}
