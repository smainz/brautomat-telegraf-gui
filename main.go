package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"brautomat-telegraf-gui/internal/config"
)

// assetsRaw enthält den kompletten frontend/-Ordner (index.html, src/*).
// Es gibt hier bewusst keinen JS-Build-Schritt (kein Vite/Webpack) -
// die Dateien werden 1:1 eingebettet und ausgeliefert.
//
//go:embed all:frontend
var assetsRaw embed.FS

// printUsage gibt eine kurze Beschreibung des Programms sowie alle Flags
// aus. Das flag-Paket ruft diese Funktion (über flag.Usage) automatisch
// in zwei Fällen selbstständig auf, ganz ohne dass wir "--help" selbst
// als eigenes Flag definieren müssten:
//  1. bei "--help" bzw. "-h" (Programm beendet sich danach mit Exit-Code 0)
//  2. bei einem unbekannten/ungültigen Flag (Exit-Code 2, inkl.
//     Fehlermeldung zum ungültigen Flag direkt davor)
func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `Brautomat Telegraf Wrapper

Ein Desktop-Wrapper, der aus einer grafischen Oberfläche eine
Telegraf-Konfiguration generiert und den Telegraf-Prozess startet bzw.
stoppt. Ein Gerät namens Brautomat stellt unter http://<host>/telemetry
alle paar Sekunden einen JSON-Messpunkt bereit; Telegraf schreibt diese
Messdaten wahlweise nach CSV, InfluxDB v2, PostgreSQL, MariaDB/MySQL
und/oder MQTT - je nachdem, welche Ziele im Formular aktiviert sind.

Normalerweise wird das Programm ohne Flags gestartet und öffnet direkt
die GUI. Die Flags unten passen das Startverhalten an bzw. ermöglichen
CLI-only-Modi ohne GUI: Standard-Templates exportieren
(--export-templates) oder telegraf direkt mit der eingelesenen
Konfiguration starten (--start-headless).

Verwendung:
  %s [Flags]

Flags:
`, os.Args[0])
	flag.PrintDefaults()
}

// validLogLevels in der Reihenfolge steigender Schwere - wird sowohl für
// die Validierung als auch für die Fehlermeldung/Hilfetext verwendet.
var validLogLevels = []string{"trace", "debug", "info", "warning", "error"}

// parseLogLevel wandelt den Wert von --log-level in den von Wails
// erwarteten logger.LogLevel-Typ um. Groß-/Kleinschreibung ist egal;
// "warn" wird als Alias für "warning" akzeptiert.
func parseLogLevel(s string) (logger.LogLevel, error) {
	switch strings.ToLower(s) {
	case "trace":
		return logger.TRACE, nil
	case "debug":
		return logger.DEBUG, nil
	case "info":
		return logger.INFO, nil
	case "warning", "warn":
		return logger.WARNING, nil
	case "error":
		return logger.ERROR, nil
	default:
		return logger.INFO, fmt.Errorf("ungültiges Log-Level %q (gültig: %s)", s, strings.Join(validLogLevels, ", "))
	}
}

// runHeadless liest die Konfiguration (gleiche Auflösung wie beim
// normalen GUI-Start: --config bzw. Standardpfad) und startet telegraf
// direkt damit, ohne jemals wails.Run() aufzurufen - es gibt also kein
// Fenster und keinen a.ctx. Ausgabezeilen landen auf stdout statt als
// Wails-Event. Blockiert, bis Ctrl+C gedrückt wird, und stoppt telegraf
// dann sauber, bevor die Funktion zurückkehrt.
func runHeadless(app *App) error {
	if err := app.initRuntimeState(); err != nil {
		return fmt.Errorf("Initialisierung fehlgeschlagen: %w", err)
	}
	defer app.shutdown(context.Background())

	configPath, err := app.GetDefaultConfigPath()
	if err != nil {
		return fmt.Errorf("Konfigurationspfad konnte nicht ermittelt werden: %w", err)
	}

	cfg, err := app.LoadConfig("")
	if err != nil {
		return fmt.Errorf("Konfiguration konnte nicht geladen werden (%s): %w", configPath, err)
	}
	log.Printf("Konfiguration geladen: %s", configPath)

	onLine := func(line string) {
		fmt.Println(line)
	}
	onExit := func(err error) {
		if err != nil {
			log.Printf("telegraf beendet mit Fehler: %v", err)
		} else {
			log.Println("telegraf beendet")
		}
	}

	if err := app.startTelegrafCore(cfg, onLine, onExit); err != nil {
		return fmt.Errorf("telegraf konnte nicht gestartet werden: %w", err)
	}
	log.Println("telegraf läuft im Headless-Modus. Zum Beenden Ctrl+C drücken.")

	// Auf Ctrl+C warten - os.Interrupt ist der einzige Signal-Wert, den
	// Go plattformübergreifend garantiert (Linux/macOS/Windows).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh

	log.Println("Beende telegraf...")
	return app.StopTelegraf()
}

func main() {
	flag.Usage = printUsage

	templatesDir := flag.String(
		"templates-dir",
		"",
		"Pfad zu einem Verzeichnis mit eigenen Telegraf-Config-Templates.\n"+
			"Muss die Dateien telegraf.conf.tmpl, outputs-csv.conf.tmpl,\n"+
			"outputs-influxdb.conf.tmpl, outputs-postgres.conf.tmpl und\n"+
			"outputs-mysql.conf.tmpl enthalten. Wird nichts angegeben,\n"+
			"werden die im Programm eingebetteten Standard-Templates verwendet.",
	)
	configPath := flag.String(
		"config",
		"",
		"Pfad zur Konfigurationsdatei (JSON), die beim Start geladen wird,\n"+
			"falls sie existiert, und auf die sich 'Speichern' im Formular\n"+
			"standardmäßig bezieht. Wird nichts angegeben, wird\n"+
			"~/.brautomat-telegraf-gui/config.json verwendet.",
	)
	exportTemplatesDir := flag.String(
		"export-templates",
		"",
		"Exportiert die eingebetteten Standard-Templates unverändert in das\n"+
			"angegebene Verzeichnis und beendet das Programm sofort, OHNE die\n"+
			"GUI zu starten. Gedacht als Ausgangspunkt, um anschließend mit\n"+
			"--templates-dir (bzw. dem Templates-Feld in der GUI) eigene\n"+
			"Templates zu verwenden.",
	)
	logLevel := flag.String(
		"log-level",
		"info",
		"Legt fest, ab welcher Schwere Wails-eigene Log-Meldungen (Start/Stop\n"+
			"von Fenstern, IPC-Bindings etc.) auf der Konsole ausgegeben werden.\n"+
			"Gültige Werte: trace, debug, info, warning, error (Groß-/\n"+
			"Kleinschreibung egal). Betrifft nicht die telegraf-Ausgabe im\n"+
			"Log-Fenster der GUI, die immer vollständig angezeigt wird.\n"+
			"Default: info.",
	)
	startHeadless := flag.Bool(
		"start-headless",
		false,
		"Liest die Konfiguration genau wie beim normalen Start (--config\n"+
			"bzw. ~/.brautomat-telegraf-gui/config.json) und startet telegraf\n"+
			"sofort damit, OHNE die GUI anzuzeigen. Läuft im Vordergrund, bis\n"+
			"das Programm mit Ctrl+C beendet wird - telegraf wird dabei sauber\n"+
			"gestoppt. Nützlich z.B. für den Betrieb ohne Desktop-Umgebung.",
	)
	flag.Parse()

	// flag.Parse() meldet nur ungültige *Flags* selbstständig (siehe
	// printUsage-Kommentar). Ein unbekanntes Positionsargument/Kommando
	// (z.B. ein Tippfehler ohne führendes "-") würde sonst stillschweigend
	// ignoriert - das behandeln wir hier explizit genauso wie ein
	// ungültiges Flag: Fehlermeldung, Hilfetext, Exit-Code 2.
	if args := flag.Args(); len(args) > 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "Unbekanntes Argument: %s\n\n", strings.Join(args, " "))
		flag.Usage()
		os.Exit(2)
	}

	parsedLogLevel, err := parseLogLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "%v\n\n", err)
		flag.Usage()
		os.Exit(2)
	}

	if *exportTemplatesDir != "" {
		if err := config.ExportEmbeddedTemplates(*exportTemplatesDir); err != nil {
			log.Fatalf("Export der Templates fehlgeschlagen: %v", err)
		}
		fmt.Printf("Templates exportiert nach %s\n", *exportTemplatesDir)
		os.Exit(0)
	}

	if *startHeadless {
		app := NewApp(*templatesDir, *configPath)
		if err := runHeadless(app); err != nil {
			log.Fatalf("Headless-Start fehlgeschlagen: %v", err)
		}
		os.Exit(0)
	}

	// "frontend" als Wurzel des Asset-Servers verwenden, damit
	// index.html direkt unter "/" erreichbar ist.
	assets, err := fs.Sub(assetsRaw, "frontend")
	if err != nil {
		log.Fatalf("Frontend-Assets konnten nicht geladen werden: %v", err)
	}

	app := NewApp(*templatesDir, *configPath)

	err = wails.Run(&options.App{
		Title:  "Brautomat Telegraf Wrapper",
		Width:  960,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// LogLevel gilt für Entwicklungs-Builds, LogLevelProduction für
		// mit "wails build" gebaute Binaries - beide auf denselben Wert
		// aus --log-level setzen, damit das Flag unabhängig von der
		// Build-Art funktioniert.
		LogLevel:           parsedLogLevel,
		LogLevelProduction: parsedLogLevel,
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
