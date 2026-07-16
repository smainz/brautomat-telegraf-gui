//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// blockIgnoringTerm ignoriert SIGTERM bewusst, damit der Aufrufer (Test)
// die SIGTERM->SIGKILL-Eskalation in process_unix.go beobachten kann.
//
// Nach dem Registrieren des Handlers wird die Zeile "sigterm-handler-ready"
// auf stdout ausgegeben. Der Aufrufer (runner_unix_test.go) MUSS auf diese
// Zeile warten, bevor er SIGTERM schickt - sonst besteht ein Startup-Race:
// bis signal.Notify() tatsächlich ausgeführt wurde, hat der Prozess noch die
// Standard-Disposition für SIGTERM (sofortige Terminierung). Auf langsamen/
// CPU-gedrosselten CI-Runnern (z.B. Docker mit cgroup-Limits) ist die Zeit
// bis dahin unzuverlässig, ein reines time.Sleep() im Test wäre daher flaky.
func blockIgnoringTerm() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		for range sigCh {
			// bewusst kein os.Exit() - SIGTERM soll wirkungslos bleiben.
		}
	}()
	fmt.Println("sigterm-handler-ready")
	time.Sleep(30 * time.Second)
}
