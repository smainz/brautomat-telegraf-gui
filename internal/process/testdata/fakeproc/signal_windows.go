//go:build windows

package main

import "time"

// blockIgnoringTerm: Windows kennt kein SIGTERM - hier bewusst identisch
// zu "sleep", der SIGKILL-Eskalationstest läuft daher nur unter unix
// (siehe runner_unix_test.go).
func blockIgnoringTerm() {
	time.Sleep(30 * time.Second)
}
