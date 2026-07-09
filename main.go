package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
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
einen CLI-only-Modus zum Exportieren der Standard-Templates, ohne dass
dafür die GUI startet.

Verwendung:
  %s [Flags]

Flags:
`, os.Args[0])
	flag.PrintDefaults()
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

	if *exportTemplatesDir != "" {
		if err := config.ExportEmbeddedTemplates(*exportTemplatesDir); err != nil {
			log.Fatalf("Export der Templates fehlgeschlagen: %v", err)
		}
		fmt.Printf("Templates exportiert nach %s\n", *exportTemplatesDir)
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
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
