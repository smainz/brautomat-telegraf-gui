// Command fakeproc ist ein winziger Testhelfer für internal/process
// (runner_test.go): steht stellvertretend für die echte telegraf-Binary,
// damit Runner.Start()/Stop()/IsRunning() ohne ein reales telegraf
// getestet werden können. Wird von TestMain per "go build" erzeugt.
//
// Verhalten steuerbar über FAKEPROC_MODE:
//   - ""            : eine stdout- und eine stderr-Zeile ausgeben, sofort beenden (Exitcode 0)
//   - "exit1"       : wie oben, aber Exitcode 1
//   - "sleep"       : wie oben, danach 30s schlafen (zum Testen von Stop())
//   - "ignore-term" : wie "sleep", ignoriert dabei aber SIGTERM (zum Testen
//     der SIGTERM->SIGKILL-Eskalation in process_unix.go; unter Windows
//     verhält es sich wie "sleep", da es dort kein SIGTERM gibt). Gibt unter
//     unix zusätzlich die Zeile "sigterm-handler-ready" aus, sobald der
//     SIGTERM-Handler registriert ist (siehe signal_unix.go) - Aufrufer
//     müssen darauf warten, bevor sie SIGTERM schicken.
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("stdout line 1")
	fmt.Fprintln(os.Stderr, "stderr line 1")

	switch os.Getenv("FAKEPROC_MODE") {
	case "exit1":
		os.Exit(1)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "ignore-term":
		blockIgnoringTerm()
	}
}
