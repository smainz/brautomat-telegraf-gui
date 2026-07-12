package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// templateFuncs stellt Hilfsfunktionen bereit, die in JEDEM Template
// (eingebettet oder eigenes via --templates-dir) per Pipe nutzbar sind:
//
//	{{.MySQL.Password | toml}}   - sicher in einen TOML-String einbetten
//	{{.MQTT.QoS | qos}}          - auf einen gültigen QoS-Wert (0/1/2) begrenzen
//
// WICHTIG (Sicherheit): Config-Werte kommen aus dem Formular ODER aus
// einer per "Konfiguration laden…" importierten config.json - letztere
// kann von woanders stammen (geteilt, heruntergeladen) und ist damit
// NICHT vertrauenswürdig. Ohne Escaping kann z.B. ein Passwort mit einem
// eingebetteten " aus seinem TOML-String ausbrechen und beliebiges TOML
// einschleusen - inklusive neuer Plugin-Blöcke wie [[outputs.exec]], die
// beim telegraf-Start beliebige Kommandos ausführen. JEDER Wert, der in
// einen "..."-String eines Templates eingesetzt wird, MUSS durch "| toml"
// laufen. Werte, die unquotiert stehen (wie qos), MÜSSEN stattdessen auf
// eine feste Wertemenge validiert werden statt escaped zu werden.
var templateFuncs = template.FuncMap{
	"toml": tomlQuote,
	"qos":  sanitizeQoS,
}

// tomlQuote escaped s für die Verwendung innerhalb eines TOML
// "basic string" (also zwischen doppelten Anführungszeichen). Deckt die
// Fälle ab, die für eine Injection relevant sind: Backslash,
// Anführungszeichen und Zeilenumbrüche/Steuerzeichen (die in einem
// einzeiligen TOML-String ohnehin nicht erlaubt sind und sonst die
// Zeile "aufbrechen" würden).
func tomlQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// sanitizeQoS lässt nur die drei gültigen MQTT-QoS-Werte durch. qos wird
// im Template bewusst UNQUOTIERT als TOML-Integer gerendert (siehe
// outputs-mqtt.conf.tmpl) - Escaping wie bei tomlQuote würde hier nicht
// helfen (ein String wäre der falsche TOML-Typ), daher stattdessen eine
// strikte Whitelist. Alles andere fällt sicher auf "0" zurück.
func sanitizeQoS(s string) string {
	switch strings.TrimSpace(s) {
	case "0", "1", "2":
		return s
	default:
		return "0"
	}
}

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
	tmpl, err := template.New(templateName).Funcs(templateFuncs).ParseFS(tmplFS, templateName)
	if err != nil {
		return fmt.Errorf("Template konnte nicht geladen werden: %w", err)
	}

	// 0600 statt os.Create()'s Default (0666 & umask, meist 0644): diese
	// Dateien enthalten zur Laufzeit von telegraf Klartext-Zugangsdaten
	// (DB-Passwörter, InfluxDB-Token, MQTT-Passwort). Das schützende
	// Arbeitsverzeichnis (os.MkdirTemp, siehe initRuntimeState in app.go)
	// ist zwar schon 0700, 0600 ist zusätzliche Verteidigungstiefe.
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("Datei konnte nicht erstellt werden: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("Template-Rendering fehlgeschlagen: %w", err)
	}
	return nil
}
