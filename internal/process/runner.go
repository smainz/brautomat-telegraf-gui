package process

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Runner verwaltet genau einen telegraf-Kindprozess.
type Runner struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}

func NewRunner() *Runner {
	return &Runner{}
}

// Start startet telegrafPath mit der übergebenen Hauptconfig und dem
// Config-Verzeichnis. onLine wird für jede Zeile aufgerufen, die der
// Prozess auf stdout/stderr schreibt; onExit einmalig, sobald der
// Prozess beendet ist (nil-Fehler = sauberes Ende).
func (r *Runner) Start(telegrafPath, configPath, configDir string, onLine func(string), onExit func(error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("telegraf läuft bereits")
	}

	cmd := exec.Command(telegrafPath,
		"--config", configPath,
		"--config-directory", configDir,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	setPlatformProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("telegraf konnte nicht gestartet werden (Pfad: %s): %w", telegrafPath, err)
	}

	r.cmd = cmd
	r.running = true

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLines(stdout, onLine, &wg)
	go streamLines(stderr, onLine, &wg)

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()
		r.mu.Lock()
		r.running = false
		r.cmd = nil
		r.mu.Unlock()
		onExit(waitErr)
	}()

	return nil
}

func streamLines(rc io.ReadCloser, onLine func(string), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
}

// Stop beendet den laufenden telegraf-Prozess, falls vorhanden. Die
// eigentliche Terminierungslogik ist plattformspezifisch (siehe
// process_unix.go / process_windows.go).
func (r *Runner) Stop() error {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return stopProcess(cmd)
}

// IsRunning meldet, ob aktuell ein telegraf-Prozess aktiv ist.
func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}
