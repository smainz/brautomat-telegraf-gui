package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfigPath liefert den Standardpfad für die gespeicherte
// Konfiguration im Home-Verzeichnis des Benutzers:
//
//	~/.brautomat-telegraf-gui/config.json
//
// os.UserHomeDir() liefert dabei plattformspezifisch das richtige
// Verzeichnis (z.B. $HOME unter Linux/macOS, %USERPROFILE% unter
// Windows), sodass der Pfad ohne weitere Anpassung auf allen drei
// Zielplattformen funktioniert.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Home-Verzeichnis konnte nicht ermittelt werden: %w", err)
	}
	return filepath.Join(home, ".brautomat-telegraf-gui", "config.json"), nil
}

// Save schreibt cfg als JSON nach path. Fehlende Elternverzeichnisse
// werden automatisch angelegt (relevant für den Default-Pfad, dessen
// Unterordner beim allerersten Speichern noch nicht existiert).
//
// Die Datei wird mit den Rechten 0600 (nur Besitzer darf lesen/schreiben)
// angelegt, da die Konfiguration ggf. Klartext-Zugangsdaten enthält
// (DB-Passwörter, InfluxDB-Token).
func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("Verzeichnis %q konnte nicht angelegt werden: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("Config konnte nicht serialisiert werden: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("Datei %q konnte nicht geschrieben werden: %w", path, err)
	}
	return nil
}

// Load liest eine zuvor gespeicherte Config von path.
//
// Existiert die Datei nicht, wird der ursprüngliche os.ErrNotExist-Fehler
// unverändert durchgereicht (per errors.Is prüfbar), damit der Aufrufer
// - z.B. beim allerersten Start der App - sauber auf Default()
// zurückfallen kann, statt einen harten Fehler anzuzeigen.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("Config %q ist kein gültiges JSON: %w", path, err)
	}
	return cfg, nil
}
