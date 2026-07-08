//go:build !windows

package process

import (
	"os/exec"
	"syscall"
	"time"
)

// setPlatformProcAttr startet den Prozess in einer eigenen Prozessgruppe,
// damit stopProcess auch eventuelle Kindprozesse von telegraf mit
// beenden kann.
func setPlatformProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopProcess versucht zunächst ein sauberes Beenden per SIGTERM und
// eskaliert nach kurzer Wartezeit auf SIGKILL, falls der Prozess nicht
// reagiert.
func stopProcess(cmd *exec.Cmd) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}

	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
