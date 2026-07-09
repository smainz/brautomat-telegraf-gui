package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// defaultTemplates bettet die Standard-Templates zur Kompilierzeit in die
// Binary ein. Damit läuft die App auch ohne separates Templates-Verzeichnis
// auf der Platte (wichtig für Windows/macOS/Linux-Distribution als
// Einzeldatei).
//
//go:embed templates/*.tmpl
var defaultTemplates embed.FS

// requiredTemplateFiles müssen in einem benutzerdefinierten
// Templates-Verzeichnis vorhanden sein, damit Generate() später nicht
// mit einem kryptischen Fehler abbricht.
var requiredTemplateFiles = []string{
	"telegraf.conf.tmpl",
	"outputs-csv.conf.tmpl",
	"outputs-influxdb.conf.tmpl",
	"outputs-postgres.conf.tmpl",
	"outputs-mysql.conf.tmpl",
	"outputs-mqtt.conf.tmpl",
}

// GetTemplatesFS liefert das Dateisystem, aus dem Templates geladen werden.
//
//   - customDir == ""      -> eingebettete Standard-Templates (embed.FS)
//   - customDir != ""      -> Verzeichnis auf der Platte (os.DirFS),
//     z.B. via --templates-dir gesetzt
//
// So kann ein Benutzer ohne Neubau der Binary eigene Templates verwenden,
// solange die Dateinamen den Standard-Templates entsprechen.
func GetTemplatesFS(customDir string) (fs.FS, error) {
	if customDir == "" {
		return fs.Sub(defaultTemplates, "templates")
	}

	info, err := os.Stat(customDir)
	if err != nil {
		return nil, fmt.Errorf("Templates-Verzeichnis %q nicht lesbar: %w", customDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q ist kein Verzeichnis", customDir)
	}

	dirFS := os.DirFS(customDir)
	for _, name := range requiredTemplateFiles {
		if _, err := fs.Stat(dirFS, name); err != nil {
			return nil, fmt.Errorf("Template %q fehlt in %q", name, customDir)
		}
	}

	return dirFS, nil
}

// ExportEmbeddedTemplates schreibt alle eingebetteten Standard-Templates
// unverändert nach destDir (wird bei Bedarf angelegt). Das ist der
// vorgesehene Weg, um an einen editierbaren Ausgangspunkt für
// --templates-dir zu kommen, ohne die Quellen dieses Programms zu
// besitzen: "--export-templates <pfad>" ruft dies auf und beendet das
// Programm, ohne die GUI zu starten (siehe main.go).
func ExportEmbeddedTemplates(destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("Zielverzeichnis %q konnte nicht angelegt werden: %w", destDir, err)
	}

	entries, err := fs.ReadDir(defaultTemplates, "templates")
	if err != nil {
		return fmt.Errorf("eingebettete Templates konnten nicht gelesen werden: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := fs.ReadFile(defaultTemplates, path.Join("templates", entry.Name()))
		if err != nil {
			return fmt.Errorf("Template %q konnte nicht gelesen werden: %w", entry.Name(), err)
		}

		destPath := filepath.Join(destDir, entry.Name())
		if err := os.WriteFile(destPath, data, 0o644); err != nil {
			return fmt.Errorf("Template %q konnte nicht geschrieben werden: %w", destPath, err)
		}
	}

	return nil
}
