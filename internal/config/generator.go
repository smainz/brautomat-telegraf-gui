package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// outputTarget beschreibt ein optionales Output-Template zusammen mit der
// Bedingung, unter der es gerendert werden soll, und dem Dateinamen im
// telegraf.d/-Verzeichnis.
type outputTarget struct {
	enabled      bool
	templateFile string
	outputFile   string
}

// Generate rendert telegraf.conf (Agent- und Input-Sektion) sowie für jedes
// aktivierte Ziel die passende telegraf.d/outputs-*.conf aus dem
// übergebenen Template-Dateisystem (eingebettet oder benutzerdefiniert)
// und schreibt alles unterhalb von outDir.
func Generate(tmplFS fs.FS, cfg Config, outDir string) error {
	confDir := filepath.Join(outDir, "telegraf.d")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return fmt.Errorf("telegraf.d konnte nicht angelegt werden: %w", err)
	}

	if err := renderToFile(tmplFS, "telegraf.conf.tmpl", filepath.Join(outDir, "telegraf.conf"), cfg); err != nil {
		return fmt.Errorf("telegraf.conf: %w", err)
	}

	// Gilt immer, unabhängig davon, welche Ziele aktiviert sind - siehe
	// processors-rename.conf.tmpl für die Begründung.
	if err := renderToFile(tmplFS, "processors-rename.conf.tmpl", filepath.Join(confDir, "processors-rename.conf"), cfg); err != nil {
		return fmt.Errorf("processors-rename.conf: %w", err)
	}

	targets := []outputTarget{
		{cfg.CSV.Enabled, "outputs-csv.conf.tmpl", "outputs-csv.conf"},
		{cfg.InfluxDB.Enabled, "outputs-influxdb.conf.tmpl", "outputs-influxdb.conf"},
		{cfg.Postgres.Enabled, "outputs-postgres.conf.tmpl", "outputs-postgres.conf"},
		{cfg.MySQL.Enabled, "outputs-mysql.conf.tmpl", "outputs-mysql.conf"},
		{cfg.MQTT.Enabled, "outputs-mqtt.conf.tmpl", "outputs-mqtt.conf"},
	}

	// Zuerst alle zuvor generierten Output-Configs entfernen, damit
	// deaktivierte Ziele beim nächsten Start nicht versehentlich
	// weiterlaufen (Telegraf liest jede *.conf im Verzeichnis).
	for _, t := range targets {
		_ = os.Remove(filepath.Join(confDir, t.outputFile))
	}

	for _, t := range targets {
		if !t.enabled {
			continue
		}
		dest := filepath.Join(confDir, t.outputFile)
		if err := renderToFile(tmplFS, t.templateFile, dest, cfg); err != nil {
			return fmt.Errorf("%s: %w", t.templateFile, err)
		}
	}

	return nil
}

func renderToFile(tmplFS fs.FS, templateName, destPath string, cfg Config) error {
	tmpl, err := template.New(templateName).ParseFS(tmplFS, templateName)
	if err != nil {
		return fmt.Errorf("Template konnte nicht geladen werden: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Datei konnte nicht erstellt werden: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("Template-Rendering fehlgeschlagen: %w", err)
	}
	return nil
}
