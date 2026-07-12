package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCSVHeader_Disabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brautomat.csv")
	cfg := Config{CSV: CSVTarget{Enabled: false, Path: path}}

	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("erwarte, dass bei deaktiviertem CSV-Ziel keine Datei angelegt wird, Stat-Fehler: %v", err)
	}
}

func TestEnsureCSVHeader_EmptyPath(t *testing.T) {
	cfg := Config{CSV: CSVTarget{Enabled: true, Path: ""}}
	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader mit leerem Pfad sollte kein Fehler sein, ist aber: %v", err)
	}
}

func TestEnsureCSVHeader_FileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brautomat.csv")
	cfg := Config{CSV: CSVTarget{Enabled: true, Path: path}}

	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Datei konnte nicht gelesen werden: %v", err)
	}
	want := strings.Join(csvColumns, ",") + "\n"
	if string(data) != want {
		t.Errorf("Header = %q, want %q", string(data), want)
	}
}

func TestEnsureCSVHeader_FileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brautomat.csv")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("Vorbereitung: leere Datei konnte nicht angelegt werden: %v", err)
	}
	cfg := Config{CSV: CSVTarget{Enabled: true, Path: path}}

	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Datei konnte nicht gelesen werden: %v", err)
	}
	want := strings.Join(csvColumns, ",") + "\n"
	if string(data) != want {
		t.Errorf("Header = %q, want %q", string(data), want)
	}
}

func TestEnsureCSVHeader_FileWithContentUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brautomat.csv")
	existing := "timestamp,mode\n2024-01-01T00:00:00Z,idle\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("Vorbereitung: Datei konnte nicht angelegt werden: %v", err)
	}
	cfg := Config{CSV: CSVTarget{Enabled: true, Path: path}}

	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Datei konnte nicht gelesen werden: %v", err)
	}
	if string(data) != existing {
		t.Errorf("bestehender Dateiinhalt wurde verändert: %q, want %q", string(data), existing)
	}
}

func TestEnsureCSVHeader_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "subdir", "brautomat.csv")
	cfg := Config{CSV: CSVTarget{Enabled: true, Path: path}}

	if err := EnsureCSVHeader(cfg); err != nil {
		t.Fatalf("EnsureCSVHeader: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("erwarte angelegte Datei inkl. Elternverzeichnisse, Stat-Fehler: %v", err)
	}
}
