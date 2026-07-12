//go:build !windows

package process

import (
	"testing"
	"time"
)

// TestRunner_StopEscalatesToSIGKILL prüft die in process_unix.go
// dokumentierte Eskalation: stopProcess() sendet zuerst SIGTERM, wartet
// bis zu 5s und schickt danach SIGKILL, falls der Prozess nicht reagiert.
// fakeproc im Modus "ignore-term" ignoriert SIGTERM absichtlich, damit
// dieser Test die Eskalation tatsächlich durchläuft.
func TestRunner_StopEscalatesToSIGKILL(t *testing.T) {
	if testing.Short() {
		t.Skip("wartet die volle 5s-SIGTERM-Timeout-Zeit ab (process_unix.go) - übersprungen im -short-Modus")
	}

	r := NewRunner()
	done := make(chan error, 1)
	startWithEnv(t, r, "ignore-term", func(string) {}, func(err error) { done <- err })

	if !r.IsRunning() {
		t.Fatal("IsRunning() sollte true sein, nachdem der Prozess gestartet wurde")
	}

	stopStart := time.Now()
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	elapsed := time.Since(stopStart)

	if elapsed < 5*time.Second {
		t.Errorf("Stop() kehrte bereits nach %v zurück - erwarte mindestens die 5s-SIGTERM-Wartezeit vor der SIGKILL-Eskalation", elapsed)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: Prozess wurde nach der SIGKILL-Eskalation nicht innerhalb von 5s beendet")
	}

	if r.IsRunning() {
		t.Error("IsRunning() sollte nach Stop() false sein")
	}
}
