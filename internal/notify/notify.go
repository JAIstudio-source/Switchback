package notify

import (
	"fmt"
	"os/exec"
	"syscall"
)

// SendToast sends a native Windows 10/11 toast notification without blocking.
func SendToast(title, message string) {
	go func() {
		psScript := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$nodes = $template.GetElementsByTagName("text")
$nodes.Item(0).AppendChild($template.CreateTextNode("%s")) | Out-Null
$nodes.Item(1).AppendChild($template.CreateTextNode("%s")) | Out-Null
$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("focusmgr").Show($toast)
`, escapePS(title), escapePS(message))

		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psScript)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
	}()
}

func escapePS(s string) string {
	res := ""
	for _, c := range s {
		if c == '"' {
			res += "`\""
		} else if c == '`' {
			res += "``"
		} else if c == '$' {
			res += "`$"
		} else {
			res += string(c)
		}
	}
	return res
}
