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
	}
}
