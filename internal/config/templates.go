package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
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
