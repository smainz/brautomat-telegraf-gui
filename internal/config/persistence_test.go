package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func fullConfig() Config {
	return Config{
		DeviceURL:        "http://brautomat.local",
		Interval:         "30s",
		TemplatesDir:     "/custom/templates",
		TelegrafPath:     "/usr/local/bin/telegraf",
		TelegrafLogLevel: "debug",
		SavePasswords:    true,
		CSV:              CSVTarget{Enabled: true, Path: "brautomat.csv"},
		InfluxDB: InfluxDBTarget{
			Enabled: true, URL: "http://localhost:8086", Token: "influx-token",
			Org: "brautomat", Bucket: "brautomat",
		},
		Postgres: SQLTarget{
			Enabled: true, Host: "localhost", Port: "5432",
			Database: "brautomat", User: "brautomat", Password: "pg-secret",
		},
		MySQL: SQLTarget{
			Enabled: true, Host: "localhost", Port: "3306",
			Database: "brautomat", User: "brautomat", Password: "mysql-secret",
		},
		MQTT: MQTTTarget{
			Enabled: true, Server: "tcp://localhost:1883", Topic: "brautomat/telemetry",
			ClientID: "brautomat-gui", Username: "mqtt-user", Password: "mqtt-secret", QoS: "1",
		},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := fullConfig()
	path := filepath.Join(t.TempDir(), "config.json")

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !reflect.DeepEqual(cfg, loaded) {
		t.Errorf("Round-Trip veränderte die Config.\nvorher: %+v\nnachher: %+v", cfg, loaded)
	}
}

func TestSaveStripsSecretsWhenDisabled(t *testing.T) {
	cfg := fullConfig()
	cfg.SavePasswords = false
	path := filepath.Join(t.TempDir(), "config.json")

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.InfluxDB.Token != "" {
		t.Errorf("InfluxDB.Token wurde trotz SavePasswords=false gespeichert: %q", loaded.InfluxDB.Token)
	}
	if loaded.Postgres.Password != "" {
		t.Errorf("Postgres.Password wurde trotz SavePasswords=false gespeichert: %q", loaded.Postgres.Password)
	}
	if loaded.MySQL.Password != "" {
		t.Errorf("MySQL.Password wurde trotz SavePasswords=false gespeichert: %q", loaded.MySQL.Password)
	}
	if loaded.MQTT.Password != "" {
		t.Errorf("MQTT.Password wurde trotz SavePasswords=false gespeichert: %q", loaded.MQTT.Password)
	}

	// Alles andere soll unangetastet bleiben.
	if loaded.Postgres.User != cfg.Postgres.User || loaded.MQTT.Username != cfg.MQTT.Username {
		t.Errorf("nicht-geheime Felder wurden unerwartet verändert: %+v", loaded)
	}

	// Das Original darf durch Save() nicht mutiert werden (Save arbeitet
	// auf einer Kopie), da der Aufrufer cfg sonst z.B. noch im Formular
	// weiterverwendet.
	if cfg.Postgres.Password != "pg-secret" {
		t.Errorf("Save() hat das übergebene cfg-Original verändert: Postgres.Password = %q", cfg.Postgres.Password)
	}
}

func TestSaveFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-Dateirechte sind unter Windows nicht aussagekräftig")
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(fullConfig(), path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config.json hat Rechte %o, erwartet 0600 (Datei kann Zugangsdaten enthalten)", mode)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subdir", "config.json")
	if err := Save(fullConfig(), path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("erwarte angelegte Datei inkl. Elternverzeichnisse, Stat-Fehler: %v", err)
	}
}

func TestLoad_NotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := Load(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load(nicht existierender Pfad) = %v, want errors.Is(err, os.ErrNotExist)", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{ das ist kein JSON"), 0o600); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load mit ungültigem JSON sollte einen Fehler liefern")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Fehlermeldung erwähnt nicht JSON: %v", err)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	want := filepath.Join(".brautomat-telegraf-gui", "config.json")
	if !strings.HasSuffix(path, want) {
		t.Errorf("DefaultConfigPath() = %q, erwarte Suffix %q", path, want)
	}
}
