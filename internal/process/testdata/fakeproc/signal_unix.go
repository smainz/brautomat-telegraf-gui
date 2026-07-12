//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// blockIgnoringTerm ignoriert SIGTERM bewusst, damit der Aufrufer (Test)
// die SIGTERM->SIGKILL-Eskalation in process_unix.go beobachten kann.
func blockIgnoringTerm() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		for range sigCh {
			// bewusst kein os.Exit() - SIGTERM soll wirkungslos bleiben.
		}
	}()
	time.Sleep(30 * time.Second)
}
