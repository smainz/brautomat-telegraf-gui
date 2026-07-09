package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"brautomat-telegraf-gui/internal/config"
	"brautomat-telegraf-gui/internal/process"
)

// App ist an das Frontend gebunden (siehe main.go: options.App.Bind).
// Alle exportierten Methoden sind aus JavaScript aufrufbar.
type App struct {
	ctx          context.Context
	templatesDir string // Wert von --templates-dir; leer = eingebettete Defaults verwenden
	workDir      string // temporäres Verzeichnis für generierte telegraf.conf / telegraf.d
	telegrafPath string // Pfad zur telegraf-Binary
	runner       *process.Runner
}

func NewApp(templatesDir string) *App {
	return &App{
		templatesDir: templatesDir,
		runner:       process.NewRunner(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	workDir, err := os.MkdirTemp("", "brautomat-telegraf-*")
	if err != nil {
		runtime.LogErrorf(ctx, "Konnte Arbeitsverzeichnis nicht anlegen: %v", err)
		return
	}
	a.workDir = workDir
	a.telegrafPath = findTelegrafBinary()

	runtime.LogInfof(ctx, "Arbeitsverzeichnis: %s", a.workDir)
	runtime.LogInfof(ctx, "telegraf-Binary: %s", a.telegrafPath)
	if a.templatesDir != "" {
		runtime.LogInfof(ctx, "Benutzerdefinierte Templates: %s", a.templatesDir)
	}
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.runner.Stop()
	if a.workDir != "" {
		_ = os.RemoveAll(a.workDir)
	}
}

// TelegrafConfig entspricht 1:1 config.Config; als eigener Name, um die
// JS-seitige API vom internen Template-Modell zu entkoppeln.
type TelegrafConfig = config.Config

// GetDefaults liefert ein mit Beispielwerten vorbelegtes Config-Objekt,
// mit dem das Formular im Frontend beim Start befüllt wird.
func (a *App) GetDefaults() TelegrafConfig {
	return config.Default()
}

// GetDefaultConfigPath liefert den Standardpfad, unter dem die
// Konfiguration gespeichert/geladen wird, wenn kein eigener Pfad gewählt
// wurde: ~/.brautomat-telegraf-gui/config.json
func (a *App) GetDefaultConfigPath() (string, error) {
	return config.DefaultConfigPath()
}

// SaveConfig speichert cfg als JSON. Ist path leer, wird der Standardpfad
// (siehe GetDefaultConfigPath) verwendet. Liefert den tatsächlich
// verwendeten Pfad zurück, damit das Frontend ihn z.B. im Log anzeigen
// oder für ein späteres "Speichern" (ohne erneuten Dialog) merken kann.
func (a *App) SaveConfig(cfg TelegrafConfig, path string) (string, error) {
	resolved, err := a.resolveConfigPath(path)
	if err != nil {
		return "", err
	}
	if err := config.Save(cfg, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// LoadConfig lädt eine zuvor gespeicherte Config. Ist path leer, wird der
// Standardpfad verwendet. Existiert dort noch keine Datei (z.B. beim
// allerersten Start der App), wird Default() zurückgegeben statt eines
// Fehlers - der Benutzer sieht dann einfach das vorbelegte Formular.
func (a *App) LoadConfig(path string) (TelegrafConfig, error) {
	resolved, err := a.resolveConfigPath(path)
	if err != nil {
		return TelegrafConfig{}, err
	}

	cfg, err := config.Load(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return config.Default(), nil
		}
		return TelegrafConfig{}, err
	}
	return cfg, nil
}

func (a *App) resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	return config.DefaultConfigPath()
}

// ChooseSaveConfigPath öffnet einen nativen "Speichern unter"-Dialog und
// liefert den vom Benutzer gewählten Pfad zurück. Bricht der Benutzer ab,
// wird ein leerer String ohne Fehler zurückgegeben.
func (a *App) ChooseSaveConfigPath() (string, error) {
	defaultPath, err := config.DefaultConfigPath()
	if err != nil {
		return "", err
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Konfiguration speichern unter",
		DefaultDirectory: filepath.Dir(defaultPath),
		DefaultFilename:  filepath.Base(defaultPath),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON-Dateien (*.json)", Pattern: "*.json"},
		},
	})
}

// ChooseOpenConfigPath öffnet einen nativen "Öffnen"-Dialog und liefert
// den vom Benutzer gewählten Pfad zurück. Bricht der Benutzer ab, wird
// ein leerer String ohne Fehler zurückgegeben.
func (a *App) ChooseOpenConfigPath() (string, error) {
	defaultPath, err := config.DefaultConfigPath()
	if err != nil {
		return "", err
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Konfiguration öffnen",
		DefaultDirectory: filepath.Dir(defaultPath),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON-Dateien (*.json)", Pattern: "*.json"},
		},
	})
}

// StartTelegraf generiert die Telegraf-Config aus den Formulardaten und
// startet den telegraf-Prozess. Ausgabezeilen werden per Event
// "telegraf:log" an das Frontend gestreamt, Statuswechsel per
// "telegraf:status" ("running" / "stopped").
func (a *App) StartTelegraf(cfg TelegrafConfig) error {
	if a.runner.IsRunning() {
		return fmt.Errorf("telegraf läuft bereits")
	}

	tmplFS, err := config.GetTemplatesFS(a.templatesDir)
	if err != nil {
		return fmt.Errorf("Templates konnten nicht geladen werden: %w", err)
	}

	confDir := filepath.Join(a.workDir, "telegraf.d")
	mainConfPath := filepath.Join(a.workDir, "telegraf.conf")

	if err := config.Generate(tmplFS, cfg, a.workDir); err != nil {
		return fmt.Errorf("Config-Generierung fehlgeschlagen: %w", err)
	}

	onLine := func(line string) {
		runtime.EventsEmit(a.ctx, "telegraf:log", line)
	}
	onExit := func(err error) {
		if err != nil {
			runtime.EventsEmit(a.ctx, "telegraf:log", "[Prozess beendet mit Fehler] "+err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "telegraf:log", "[Prozess beendet]")
		}
		runtime.EventsEmit(a.ctx, "telegraf:status", "stopped")
	}

	if err := a.runner.Start(a.telegrafPath, mainConfPath, confDir, onLine, onExit); err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, "telegraf:status", "running")
	return nil
}

// StopTelegraf beendet den laufenden telegraf-Prozess (falls vorhanden).
func (a *App) StopTelegraf() error {
	return a.runner.Stop()
}

// IsRunning meldet, ob aktuell ein telegraf-Prozess läuft.
func (a *App) IsRunning() bool {
	return a.runner.IsRunning()
}

// findTelegrafBinary sucht zuerst eine mitgelieferte Binary im
// bin/-Unterordner neben der eigenen Executable (für ein "fertiges"
// Paket ohne separate Telegraf-Installation), und fällt sonst auf den
// PATH zurück (Benutzer hat Telegraf selbst installiert).
func findTelegrafBinary() string {
	name := "telegraf"
	if isWindows() {
		name = "telegraf.exe"
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "bin", name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}

	return name
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
