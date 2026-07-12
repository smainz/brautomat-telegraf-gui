package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeProcPath zeigt auf die von TestMain gebaute Testhelfer-Binary
// (testdata/fakeproc) - steht stellvertretend für die echte
// telegraf-Binary, damit Start()/Stop()/IsRunning() ohne echtes telegraf
// getestet werden können.
var fakeProcPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "fakeproc-build-*")
	if err != nil {
		panic("Temp-Verzeichnis für fakeproc konnte nicht angelegt werden: " + err.Error())
	}

	name := "fakeproc"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	fakeProcPath = filepath.Join(tmpDir, name)

	build := exec.Command("go", "build", "-o", fakeProcPath, "./testdata/fakeproc")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.RemoveAll(tmpDir)
		panic("Testhelfer fakeproc konnte nicht gebaut werden: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// startWithEnv setzt FAKEPROC_MODE für den aktuellen Test (wird von
// exec.Command an den Kindprozess vererbt, da Runner.Start() kein
// eigenes Cmd.Env setzt) und startet fakeProcPath über r.
func startWithEnv(t *testing.T, r *Runner, mode string, onLine func(string), onExit func(error)) {
	t.Helper()
	t.Setenv("FAKEPROC_MODE", mode)

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "telegraf.conf")
	if err := os.WriteFile(configPath, nil, 0o644); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}

	if err := r.Start(fakeProcPath, configPath, configDir, onLine, onExit); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestRunner_StartStreamsOutputAndExitsCleanly(t *testing.T) {
	r := NewRunner()

	var mu sync.Mutex
	var lines []string
	done := make(chan error, 1)

	onLine := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	}
	onExit := func(err error) { done <- err }

	startWithEnv(t, r, "", onLine, onExit)

	if !r.IsRunning() {
		t.Error("IsRunning() sollte direkt nach dem Start true sein")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("onExit erhielt einen Fehler bei sauberem Prozessende: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: Prozess hat sich nicht innerhalb von 5s beendet")
	}

	if r.IsRunning() {
		t.Error("IsRunning() sollte nach Prozessende false sein")
	}

	mu.Lock()
	joined := strings.Join(lines, "\n")
	mu.Unlock()
	if !strings.Contains(joined, "stdout line 1") {
		t.Errorf("erwarte stdout-Zeile des Kindprozesses, erhalten: %v", lines)
	}
	if !strings.Contains(joined, "stderr line 1") {
		t.Errorf("erwarte stderr-Zeile des Kindprozesses, erhalten: %v", lines)
	}
}

func TestRunner_ExitError(t *testing.T) {
	r := NewRunner()
	done := make(chan error, 1)

	startWithEnv(t, r, "exit1", func(string) {}, func(err error) { done <- err })

	select {
	case err := <-done:
		if err == nil {
			t.Error("erwarte einen Fehler, da der Kindprozess mit Exitcode 1 beendet wurde")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout beim Warten auf onExit")
	}
}

func TestRunner_Stop(t *testing.T) {
	r := NewRunner()
	done := make(chan error, 1)

	startWithEnv(t, r, "sleep", func(string) {}, func(err error) { done <- err })

	if !r.IsRunning() {
		t.Fatal("IsRunning() sollte true sein, nachdem der Prozess gestartet wurde")
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: Prozess wurde nach Stop() nicht innerhalb von 5s beendet")
	}

	if r.IsRunning() {
		t.Error("IsRunning() sollte nach Stop() false sein")
	}
}

func TestRunner_StopWithoutRunningProcessIsNoop(t *testing.T) {
	r := NewRunner()
	if err := r.Stop(); err != nil {
		t.Errorf("Stop() ohne laufenden Prozess sollte kein Fehler sein, ist aber: %v", err)
	}
}

func TestRunner_DoubleStartFails(t *testing.T) {
	r := NewRunner()
	startWithEnv(t, r, "sleep", func(string) {}, func(error) {})
	defer r.Stop()

	configDir := t.TempDir()
	err := r.Start(fakeProcPath, filepath.Join(configDir, "telegraf.conf"), configDir, func(string) {}, func(error) {})
	if err == nil {
		t.Error("ein zweiter Start(), während der erste Prozess noch läuft, sollte fehlschlagen")
	}
}
