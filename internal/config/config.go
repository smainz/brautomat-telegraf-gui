package config

// Config bündelt alle Werte, die der Benutzer im GUI-Formular einträgt.
// Die JSON-Tags bestimmen, wie das Objekt auf der JS-Seite aussieht
// (Wails serialisiert Go-Structs automatisch zu/von JSON).
type Config struct {
	DeviceURL string `json:"deviceUrl"`
	Interval  string `json:"interval"` // z.B. "30s"

	// TemplatesDir: Pfad zu einem Verzeichnis mit eigenen Telegraf-Config-
	// Templates. Leerer String = eingebettete Standard-Templates
	// verwenden (siehe internal/config/templates.go, GetTemplatesFS).
	TemplatesDir string `json:"templatesDir"`

	// SavePasswords steuert, ob Save() Passwörter/Token (InfluxDB-Token,
	// Postgres-/MySQL-Passwort) mit in die Konfigurationsdatei schreibt.
	// Zero-Value ist false, d.h. Passwörter werden standardmäßig NICHT
	// gespeichert (siehe Save() in persistence.go).
	SavePasswords bool `json:"savePasswords"`

	CSV      CSVTarget      `json:"csv"`
	InfluxDB InfluxDBTarget `json:"influxdb"`
	Postgres SQLTarget      `json:"postgres"`
	MySQL    SQLTarget      `json:"mysql"`
	MQTT     MQTTTarget     `json:"mqtt"`
}

type CSVTarget struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type InfluxDBTarget struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	Org     string `json:"org"`
	Bucket  string `json:"bucket"`
}

// SQLTarget wird sowohl für Postgres als auch für MariaDB/MySQL verwendet;
// die Templates bauen daraus jeweils den passenden DSN-String.
type SQLTarget struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// MQTTTarget beschreibt einen MQTT-Broker als Ziel. QoS bleibt bewusst
// als String (statt int) im Modell, da es 1:1 aus einem Formularfeld
// kommt und unverändert - ohne Anführungszeichen - ins TOML-Template
// eingesetzt wird (siehe outputs-mqtt.conf.tmpl); Default() sorgt dafür,
// dass hier nie ein leerer String ankommt.
type MQTTTarget struct {
	Enabled  bool   `json:"enabled"`
	Server   string `json:"server"` // z.B. tcp://localhost:1883
	Topic    string `json:"topic"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	QoS      string `json:"qos"` // "0", "1" oder "2"
}

// Default liefert ein Config mit sinnvollen Platzhalterwerten für den
// initialen Formularzustand im Frontend.
func Default() Config {
	return Config{
		DeviceURL: "http://brautomat.local",
		Interval:  "30s",
		CSV: CSVTarget{
			Enabled: true,
			Path:    "brautomat.csv",
		},
		InfluxDB: InfluxDBTarget{
			URL:    "http://localhost:8086",
			Bucket: "brautomat",
		},
		Postgres: SQLTarget{
			Port:     "5432",
			Database: "brautomat",
			User:     "brautomat",
		},
		MySQL: SQLTarget{
			Port:     "3306",
			Database: "brautomat",
			User:     "brautomat",
		},
		MQTT: MQTTTarget{
			Server: "tcp://localhost:1883",
			Topic:  "brautomat/telemetry",
			QoS:    "0",
		},
	}
}
