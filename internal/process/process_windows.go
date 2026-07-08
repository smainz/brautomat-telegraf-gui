//go:build windows

package process

import (
	"os/exec"
)

// setPlatformProcAttr: unter Windows ist kein spezielles ProcAttr nötig.
func setPlatformProcAttr(cmd *exec.Cmd) {}

// stopProcess beendet den Prozess direkt, da Windows kein SIGTERM kennt.
// Für ein saubereres Stoppen inkl. Kindprozessen könnte hier optional
// "taskkill /T /PID <pid>" verwendet werden.
func stopProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
