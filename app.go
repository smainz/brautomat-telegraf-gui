package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"brautomat-telegraf-gui/internal/config"
	"brautomat-telegraf-gui/internal/process"
	teledownload "brautomat-telegraf-gui/internal/telegraf"
)

// App ist an das Frontend gebunden (siehe main.go: options.App.Bind).
// Alle exportierten Methoden sind aus JavaScript aufrufbar.
type App struct {
	ctx            context.Context
	templatesDir   string // Wert von --templates-dir; nur Vorschlagswert fürs Formular (siehe GetDefaults), GUI kann ihn überschreiben
	configPathFlag string // Wert von --config; leer = ~/.brautomat-telegraf-gui/config.json verwenden
	workDir        string // temporäres Verzeichnis für generierte telegraf.conf / telegraf.d
	telegrafPath   string // Pfad zur telegraf-Binary
	runner         *process.Runner
}

func NewApp(templatesDir, configPath string) *App {
	return &App{
		templatesDir:   templatesDir,
		configPathFlag: configPath,
		runner:         process.NewRunner(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.initRuntimeState(); err != nil {
		runtime.LogErrorf(ctx, "Initialisierung fehlgeschlagen: %v", err)
		return
	}

	runtime.LogInfof(ctx, "Arbeitsverzeichnis: %s", a.workDir)
	runtime.LogInfof(ctx, "telegraf-Binary: %s", a.telegrafPath)
	if a.templatesDir != "" {
		runtime.LogInfof(ctx, "Benutzerdefinierte Templates: %s", a.templatesDir)
	}
	if resolvedConfigPath, err := a.resolveConfigPath(""); err == nil {
		runtime.LogInfof(ctx, "Konfigurationsdatei: %s", resolvedConfigPath)
	}
}

// initRuntimeState legt das temporäre Arbeitsverzeichnis an und ermittelt
// den Pfad zur telegraf-Binary. Bewusst getrennt von startup() und ohne
// jeden Wails-Bezug (kein a.ctx nötig), damit es auch im Headless-Modus
// (--start-headless, siehe main.go) verwendet werden kann, wo nie
// wails.Run() bzw. OnStartup läuft.
func (a *App) initRuntimeState() error {
	workDir, err := os.MkdirTemp("", "brautomat-telegraf-*")
	if err != nil {
		return fmt.Errorf("Arbeitsverzeichnis konnte nicht angelegt werden: %w", err)
	}
	a.workDir = workDir
	a.telegrafPath = findTelegrafBinary()
	return nil
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
// mit dem das Formular im Frontend beim Start befüllt wird. TemplatesDir
// wird dabei mit dem --templates-dir Flag vorbelegt (falls gesetzt), das
// Formular kann diesen Wert danach jederzeit überschreiben.
func (a *App) GetDefaults() TelegrafConfig {
	cfg := config.Default()
	cfg.TemplatesDir = a.templatesDir
	cfg.TelegrafPath = a.telegrafPath
	return cfg
}

// GetDefaultConfigPath liefert den effektiv verwendeten Pfad für die
// Konfiguration: das per --config gesetzte Flag, falls vorhanden, sonst
// den Standardpfad ~/.brautomat-telegraf-gui/config.json.
func (a *App) GetDefaultConfigPath() (string, error) {
	return a.resolveConfigPath("")
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
			return a.GetDefaults(), nil
		}
		return TelegrafConfig{}, err
	}
	return cfg, nil
}

// resolveConfigPath bestimmt den tatsächlich zu verwendenden
// Konfigurationspfad, in dieser Prioritätsreihenfolge:
//  1. explizit übergebener path (z.B. vom "Speichern unter..."-Dialog)
//  2. --config Flag beim Programmstart
//  3. Standardpfad ~/.brautomat-telegraf-gui/config.json
func (a *App) resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if a.configPathFlag != "" {
		return a.configPathFlag, nil
	}
	return config.DefaultConfigPath()
}

// deviceTestTimeout begrenzt, wie lange "Testen" auf eine Antwort des
// Geräts wartet, damit der Button nicht endlos hängt, wenn das Gerät
// nicht erreichbar ist.
const deviceTestTimeout = 5 * time.Second

// TestDeviceConnection prüft, ob unter deviceURL + "/telemetry" ein
// plausibler Messpunkt abrufbar ist. Liefert nil bei Erfolg; der Fehler
// bei Misserfolg ist bewusst als für Menschen verständlicher Text
// formuliert, da das Frontend ihn direkt in einem Pop-up anzeigt.
func (a *App) TestDeviceConnection(deviceURL string) error {
	deviceURL = strings.TrimSpace(deviceURL)
	if deviceURL == "" {
		return fmt.Errorf("keine Geräte-URL angegeben")
	}

	parsed, err := url.Parse(deviceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("ungültige URL %q (Format z.B. http://brautomat.local)", deviceURL)
	}

	telemetryURL := strings.TrimSuffix(deviceURL, "/") + "/telemetry"

	client := http.Client{Timeout: deviceTestTimeout}
	resp, err := client.Get(telemetryURL)
	if err != nil {
		return fmt.Errorf("Verbindung zu %s fehlgeschlagen: %w", telemetryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s antwortete mit Status %s (erwartet: 200 OK)", telemetryURL, resp.Status)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("Antwort von %s ist kein gültiges JSON: %w", telemetryURL, err)
	}

	// Grober Plausibilitätscheck statt vollständiger Schema-Validierung:
	// das Feld "t" (Device-Unixzeit) sollte in jedem echten Messpunkt
	// vorhanden sein.
	if _, ok := payload["t"]; !ok {
		return fmt.Errorf("Antwort von %s enthält kein Feld \"t\" - ist das wirklich der Brautomat-Telemetrie-Endpunkt?", telemetryURL)
	}

	return nil
}

// ChooseTelegrafPath öffnet einen nativen Datei-Dialog, damit der
// Benutzer eine vorhandene telegraf-Executable auswählen kann, statt
// den Pfad von Hand einzutippen. Bricht der Benutzer ab, wird ein
// leerer String ohne Fehler zurückgegeben.
func (a *App) ChooseTelegrafPath() (string, error) {
	filters := []runtime.FileFilter{
		{DisplayName: "Alle Dateien", Pattern: "*"},
	}
	if isWindows() {
		filters = []runtime.FileFilter{
			{DisplayName: "Programme (*.exe)", Pattern: "*.exe"},
		}
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "telegraf-Executable auswählen",
		Filters: filters,
	})
}

// ChooseTelegrafDownloadDir öffnet einen nativen Verzeichnis-Dialog für
// das Zielverzeichnis von "telegraf herunterladen…". Als Vorschlag dient
// der Standard-Installationsort (~/.brautomat-telegraf-gui/telegraf).
// Bricht der Benutzer ab, wird ein leerer String ohne Fehler
// zurückgegeben.
func (a *App) ChooseTelegrafDownloadDir() (string, error) {
	defaultDir, err := teledownload.InstallDir()
	if err != nil {
		return "", err
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Zielverzeichnis für telegraf-Download wählen",
		DefaultDirectory: defaultDir,
	})
}

// DownloadTelegraf lädt telegraf für die aktuell laufende Plattform
// herunter und entpackt es nach destDir. Ist destDir leer, wird der
// Standardort (~/.brautomat-telegraf-gui/telegraf) verwendet.
//
// Da der Rückgabewert (Pfad zur Executable) erst nach vollständigem
// Abschluss vorliegt, werden Zwischenzustände währenddessen per Event
// an das Frontend gestreamt - "telegraf-download:status" mit einer
// Textmeldung ("Lade herunter…", "Entpacke…", ...) und
// "telegraf-download:progress" mit {downloaded, total} in Bytes
// (total = 0, falls unbekannt). Das Frontend zeigt beides in einem
// Fortschrittsfenster an (siehe downloadTelegrafBtn-Handler in
// main.js).
func (a *App) DownloadTelegraf(destDir string) (string, error) {
	if destDir == "" {
		var err error
		destDir, err = teledownload.InstallDir()
		if err != nil {
			return "", err
		}
	}

	onStatus := func(msg string) {
		runtime.EventsEmit(a.ctx, "telegraf-download:status", msg)
	}
	onProgress := func(downloaded, total int64) {
		runtime.EventsEmit(a.ctx, "telegraf-download:progress", map[string]int64{
			"downloaded": downloaded,
			"total":      total,
		})
	}

	return teledownload.DownloadAndExtract(destDir, onStatus, onProgress)
}

// ChooseSaveConfigPath öffnet einen nativen "Speichern unter"-Dialog und
// liefert den vom Benutzer gewählten Pfad zurück. Bricht der Benutzer ab,
// wird ein leerer String ohne Fehler zurückgegeben.
func (a *App) ChooseSaveConfigPath() (string, error) {
	suggestedPath, err := a.resolveConfigPath("")
	if err != nil {
		return "", err
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Konfiguration speichern unter",
		DefaultDirectory: filepath.Dir(suggestedPath),
		DefaultFilename:  filepath.Base(suggestedPath),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON-Dateien (*.json)", Pattern: "*.json"},
		},
	})
}

// ChooseOpenConfigPath öffnet einen nativen "Öffnen"-Dialog und liefert
// den vom Benutzer gewählten Pfad zurück. Bricht der Benutzer ab, wird
// ein leerer String ohne Fehler zurückgegeben.
func (a *App) ChooseOpenConfigPath() (string, error) {
	suggestedPath, err := a.resolveConfigPath("")
	if err != nil {
		return "", err
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Konfiguration öffnen",
		DefaultDirectory: filepath.Dir(suggestedPath),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON-Dateien (*.json)", Pattern: "*.json"},
		},
	})
}

// ChooseExportTemplatesDir öffnet einen nativen Verzeichnis-Dialog für
// das Zielverzeichnis des Template-Exports.
func (a *App) ChooseExportTemplatesDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Zielverzeichnis für Template-Export wählen",
	})
}

// ExportTemplates exportiert die eingebetteten Standard-Templates
// unverändert nach destDir - das GUI-Äquivalent zum "--export-templates"
// CLI-Flag (siehe main.go), nur ohne die GUI dafür zu verlassen. Gedacht
// als bequemer Ausgangspunkt, um die Templates anschließend anzupassen
// und über das Textfeld/"Durchsuchen…" im Templates-Panel wieder zu
// verwenden.
func (a *App) ExportTemplates(destDir string) error {
	return config.ExportEmbeddedTemplates(destDir)
}

// ChooseTemplatesDir öffnet einen nativen Verzeichnis-Dialog, damit der
// Benutzer den Pfad zu eigenen Templates nicht von Hand eintippen muss.
// Bricht der Benutzer ab, wird ein leerer String ohne Fehler
// zurückgegeben.
func (a *App) ChooseTemplatesDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Verzeichnis mit eigenen Templates wählen",
	})
}

// ChooseSaveLogPath öffnet einen nativen "Speichern unter"-Dialog für die
// Ausgabe des Log-Fensters. Bricht der Benutzer ab, wird ein leerer
// String ohne Fehler zurückgegeben.
func (a *App) ChooseSaveLogPath() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Ausgabe speichern unter",
		DefaultFilename: "brautomat-telegraf.log",
		Filters: []runtime.FileFilter{
			{DisplayName: "Log-Dateien (*.log)", Pattern: "*.log"},
			{DisplayName: "Textdateien (*.txt)", Pattern: "*.txt"},
			{DisplayName: "Alle Dateien (*.*)", Pattern: "*.*"},
		},
	})
}

// SaveLog schreibt content (den aktuellen Inhalt des Ausgabefensters) als
// reinen Text nach path. Das Frontend übergibt den Text, da das
// Log-Fenster rein clientseitig geführt wird und dem Backend nicht
// bekannt ist.
func (a *App) SaveLog(content, path string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("Ausgabe konnte nicht gespeichert werden: %w", err)
	}
	return nil
}

// StartTelegraf generiert die Telegraf-Config aus den Formulardaten und
// startet den telegraf-Prozess. Ausgabezeilen werden per Event
// "telegraf:log" an das Frontend gestreamt, Statuswechsel per
// "telegraf:status" ("running" / "stopped").
func (a *App) StartTelegraf(cfg TelegrafConfig) error {
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

	if err := a.startTelegrafCore(cfg, onLine, onExit); err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, "telegraf:status", "running")
	return nil
}

// startTelegrafCore enthält die eigentliche, GUI-unabhängige Logik zum
// Rendern der Telegraf-Config und Starten des Prozesses. Sowohl
// StartTelegraf (GUI, Ausgabe per Wails-Event) als auch der
// Headless-Modus (--start-headless in main.go, Ausgabe auf der Konsole)
// rufen diese Methode auf und übergeben dafür jeweils passende
// onLine/onExit-Callbacks - so bleibt die eigentliche Start-Logik an
// genau einer Stelle.
func (a *App) startTelegrafCore(cfg TelegrafConfig, onLine func(string), onExit func(error)) error {
	if a.runner.IsRunning() {
		return fmt.Errorf("telegraf läuft bereits")
	}

	// cfg.TemplatesDir kommt aus dem Formular (leer = interne Templates)
	// und hat damit Vorrang vor dem --templates-dir Flag, das nur als
	// initialer Vorschlagswert dient (siehe GetDefaults/LoadConfig).
	tmplFS, err := config.GetTemplatesFS(cfg.TemplatesDir)
	if err != nil {
		return fmt.Errorf("Templates konnten nicht geladen werden: %w", err)
	}

	confDir := filepath.Join(a.workDir, "telegraf.d")
	mainConfPath := filepath.Join(a.workDir, "telegraf.conf")

	if err := config.Generate(tmplFS, cfg, a.workDir); err != nil {
		return fmt.Errorf("Config-Generierung fehlgeschlagen: %w", err)
	}

	// Schreibt die CSV-Kopfzeile einmalig, falls die Zieldatei noch
	// nicht existiert oder leer ist. Muss vor dem Start von telegraf
	// passieren, da telegraf selbst (bewusst) keinen Header schreibt -
	// siehe csv_header = false in outputs-csv.conf.tmpl.
	if err := config.EnsureCSVHeader(cfg); err != nil {
		return fmt.Errorf("CSV-Header konnte nicht vorbereitet werden: %w", err)
	}

	// cfg.TelegrafPath kommt aus dem Formular und hat damit Vorrang vor
	// der beim Start automatisch ermittelten a.telegrafPath (siehe
	// findTelegrafBinary) - leer = automatische Erkennung verwenden.
	telegrafPath := cfg.TelegrafPath
	if telegrafPath == "" {
		telegrafPath = a.telegrafPath
	}

	return a.runner.Start(telegrafPath, mainConfPath, confDir, onLine, onExit)
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
