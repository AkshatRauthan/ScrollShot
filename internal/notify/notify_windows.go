//go:build windows

// Package notify sends best-effort desktop notifications after a successful
// save. Errors are always silently ignored — notifications are never critical
// to the tool's function.
package notify

import (
	"fmt"
	"os/exec"
)

// Send shows a Windows system-tray balloon notification via PowerShell.
//
// Uses System.Windows.Forms.NotifyIcon, which is available on all versions
// of Windows that this binary targets (Windows 10/11). The notification
// appears as a standard balloon tip in the system tray.
func Send(title, body string) {
	defer func() { recover() }()

	// Escape single quotes so the PowerShell inline string isn't broken
	// by paths that contain apostrophes (e.g. C:\Users\O'Brien\...).
	safeTitle := escapePS(title)
	safeBody := escapePS(body)

	script := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`$n = New-Object System.Windows.Forms.NotifyIcon; `+
			`$n.Icon = [System.Drawing.SystemIcons]::Information; `+
			`$n.Visible = $True; `+
			`$n.ShowBalloonTip(5000, '%s', '%s', `+
			`[System.Windows.Forms.ToolTipIcon]::Info); `+
			`Start-Sleep -Milliseconds 5500; `+
			`$n.Dispose()`,
		safeTitle, safeBody,
	)

	exec.Command(
		"powershell",
		"-NoProfile",
		"-WindowStyle", "Hidden",
		"-Command", script,
	).Run()
}

// escapePS replaces single quotes with two single quotes, which is the
// PowerShell escape sequence for a literal single quote inside a
// single-quoted string.
func escapePS(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}
