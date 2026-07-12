package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// outputConfPath ist der Pfad, unter dem Generate() eine Ziel-Konfiguration
// ablegt bzw. entfernt.
func outputConfPath(outDir, name string) string {
	return filepath.Join(outDir, "telegraf.d", name)
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("erwarte, dass %q existiert, aber: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("erwarte, dass %q NICHT existiert", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("unerwarteter Fehler beim Prüfen von %q: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Datei %q konnte nicht gelesen werden: %v", path, err)
	}
	return string(data)
}

func TestGenerate_NoTargetsEnabled(t *testing.T) {
	tmplFS, err := GetTemplatesFS("")
	if err != nil {
		t.Fatalf("GetTemplatesFS: %v", err)
	}

	cfg := Config{DeviceURL: "http://example.invalid", Interval: "30s"}
	outDir := t.TempDir()

	if err := Generate(tmplFS, cfg, outDir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	mustExist(t, filepath.Join(outDir, "telegraf.conf"))
	mustExist(t, outputConfPath(outDir, "processors-rename.conf"))

	for _, name := range []string{
		"outputs-csv.conf",
		"outputs-influxdb.conf",
		"outputs-postgres.conf",
		"outputs-mysql.conf",
		"outputs-mqtt.conf",
	} {
		mustNotExist(t, outputConfPath(outDir, name))
	}
}

func TestGenerate_EnabledTargetsCreateExpectedFiles(t *testing.T) {
	tmplFS, err := GetTemplatesFS("")
	if err != nil {
		t.Fatalf("GetTemplatesFS: %v", err)
	}

	cfg := Config{
		DeviceURL: "http://example.invalid",
		Interval:  "30s",
		CSV:       CSVTarget{Enabled: true, Path: "brautomat.csv"},
		Postgres: SQLTarget{
			Enabled: true, Host: "localhost", Port: "5432",
			Database: "brautomat", User: "brautomat", Password: "secret",
		},
		MQTT: MQTTTarget{
			Enabled: true, Server: "tcp://localhost:1883",
			Topic: "brautomat/telemetry", QoS: "1",
		},
	}
	outDir := t.TempDir()

	if err := Generate(tmplFS, cfg, outDir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	mustExist(t, outputConfPath(outDir, "outputs-csv.conf"))
	mustExist(t, outputConfPath(outDir, "outputs-postgres.conf"))
	mustExist(t, outputConfPath(outDir, "outputs-mqtt.conf"))
	mustNotExist(t, outputConfPath(outDir, "outputs-influxdb.conf"))
	mustNotExist(t, outputConfPath(outDir, "outputs-mysql.conf"))

	csvContent := readFile(t, outputConfPath(outDir, "outputs-csv.conf"))
	if !strings.Contains(csvContent, "brautomat.csv") {
		t.Errorf("outputs-csv.conf enthält nicht den konfigurierten Pfad:\n%s", csvContent)
	}

	pgContent := readFile(t, outputConfPath(outDir, "outputs-postgres.conf"))
	if !strings.Contains(pgContent, "brautomat") || !strings.Contains(pgContent, "localhost") {
		t.Errorf("outputs-postgres.conf enthält nicht die erwarteten Verbindungsdaten:\n%s", pgContent)
	}

	mqttContent := readFile(t, outputConfPath(outDir, "outputs-mqtt.conf"))
	if !strings.Contains(mqttContent, "qos = 1") {
		t.Errorf("outputs-mqtt.conf enthält nicht den erwarteten QoS-Wert:\n%s", mqttContent)
	}
}

func TestGenerate_DisablingRemovesStaleOutputFile(t *testing.T) {
	tmplFS, err := GetTemplatesFS("")
	if err != nil {
		t.Fatalf("GetTemplatesFS: %v", err)
	}
	outDir := t.TempDir()

	enabled := Config{CSV: CSVTarget{Enabled: true, Path: "brautomat.csv"}}
	if err := Generate(tmplFS, enabled, outDir); err != nil {
		t.Fatalf("Generate (enabled): %v", err)
	}
	mustExist(t, outputConfPath(outDir, "outputs-csv.conf"))

	disabled := Config{CSV: CSVTarget{Enabled: false, Path: "brautomat.csv"}}
	if err := Generate(tmplFS, disabled, outDir); err != nil {
		t.Fatalf("Generate (disabled): %v", err)
	}
	mustNotExist(t, outputConfPath(outDir, "outputs-csv.conf"))
}

func TestGenerate_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-Dateirechte sind unter Windows nicht aussagekräftig")
	}

	tmplFS, err := GetTemplatesFS("")
	if err != nil {
		t.Fatalf("GetTemplatesFS: %v", err)
	}
	outDir := t.TempDir()
	cfg := Config{Postgres: SQLTarget{Enabled: true, Password: "secret"}}

	if err := Generate(tmplFS, cfg, outDir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, path := range []string{
		filepath.Join(outDir, "telegraf.conf"),
		outputConfPath(outDir, "outputs-postgres.conf"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q): %v", path, err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%q hat Rechte %o, erwartet 0600 (Datei enthält ggf. Zugangsdaten)", path, mode)
		}
	}
}

func TestTomlQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"leer", "", ""},
		{"unauffällig", "brautomat", "brautomat"},
		{"doppeltes Anführungszeichen", "secret\"password", "secret\\\"password"},
		{"Backslash", "C:\\path\\to\\thing", "C:\\\\path\\\\to\\\\thing"},
		{"Newline", "line1\nline2", "line1\\nline2"},
		{"Carriage Return", "a\rb", "a\\rb"},
		{"Tab", "a\tb", "a\\tb"},
		// Steuerzeichen < 0x20 werden als \u00XX escaped (siehe tomlQuote).
		{"Steuerzeichen", "a\x01b", "a\\u0001b"},
		{
			"TOML/Injection-Versuch",
			"\" [[outputs.exec]] commands=[\"rm -rf /\"] #",
			"\\\" [[outputs.exec]] commands=[\\\"rm -rf /\\\"] #",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := tomlQuote(tt.input)
			if escaped != tt.want {
				t.Errorf("tomlQuote(%q) = %q, want %q", tt.input, escaped, tt.want)
			}
			// Sicherstellen, dass das Ergebnis keine unescapten " enthält,
			// die einen TOML-String vorzeitig beenden könnten.
			for i := 0; i < len(escaped); i++ {
				if escaped[i] == '"' && (i == 0 || escaped[i-1] != '\\') {
					t.Errorf("tomlQuote(%q) liefert ein unescaptes \" im Ergebnis: %q", tt.input, escaped)
				}
			}
		})
	}
}

func TestSanitizeQoS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0", "0"},
		{"1", "1"},
		{"2", "2"},
		{" 1 ", "1"},
		{"3", "0"},
		{"", "0"},
		{"abc", "0"},
		{"-1", "0"},
	}

	for _, tt := range tests {
		if got := sanitizeQoS(tt.input); got != tt.want {
			t.Errorf("sanitizeQoS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
