//go:build windows

package notify

import (
	"os/exec"
	"strings"
)

const supported = true

// Windows has no command that simply shows a notification, so this drives the
// same WinRT toast API the Settings app uses, through PowerShell, which is
// present on every Windows 10 and 11.
//
// Untested: no Windows machine has run this. It fails silently if anything
// about the API or the execution policy disagrees, which is the same outcome as
// not sending one.
const toastScript = `
$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] > $null

$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(
    [Windows.UI.Notifications.ToastTemplateType]::ToastText02)

$text = $template.GetElementsByTagName('text')
$text[0].AppendChild($template.CreateTextNode('%TITLE%')) > $null
$text[1].AppendChild($template.CreateTextNode('%BODY%')) > $null

$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Pusher').Show($toast)
`

func send(title, body string) bool {
	script := strings.NewReplacer("%TITLE%", quote(title), "%BODY%", quote(body)).Replace(toastScript)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)

	return cmd.Run() == nil
}

// quote escapes for a PowerShell single-quoted string, where the only escape
// there is doubles the quote.
func quote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
