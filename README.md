# Brautomat Telegraf Wrapper

Minimale Wails-App (Go-Backend + einfaches HTML/JS-Frontend), die eine
Telegraf-Config aus einem Formular generiert und den Telegraf-Prozess
startet/stoppt. Läuft unter Linux, macOS und Windows.

## Verzeichnisstruktur

```
brautomat-telegraf-gui/
├── go.mod
├── main.go                          # Einstiegspunkt, --templates-dir Flag, embed der Frontend-Assets
├── app.go                           # An das Frontend gebundene Methoden (Start/Stop/Defaults)
├── wails.json                       # Wails-Projektkonfiguration
├── internal/
│   ├── config/
│   │   ├── config.go                # Config-Struct (Formularmodell)
│   │   ├── templates.go             # //go:embed + GetTemplatesFS (Default vs. --templates-dir)
│   │   ├── generator.go             # Rendert Templates -> telegraf.conf / telegraf.d/*.conf
│   │   └── templates/               # Eingebettete Standard-Templates
│   │       ├── telegraf.conf.tmpl
│   │       ├── outputs-csv.conf.tmpl
│   │       ├── outputs-influxdb.conf.tmpl
│   │       ├── outputs-postgres.conf.tmpl
│   │       └── outputs-mysql.conf.tmpl
│   └── process/
│       ├── runner.go                # Start/Stop/Log-Streaming (plattformneutraler Teil)
│       ├── process_unix.go          # SIGTERM/SIGKILL, Prozessgruppen (Linux/macOS)
│       └── process_windows.go       # Kill() (Windows)
├── frontend/
│   ├── index.html                   # Formular + Log-Fenster
│   └── src/
│       ├── main.js                  # Formular auslesen, Start/Stop, Events anzeigen
│       └── style.css
└── bin/                              # Hier die telegraf-Binary pro Zielplattform ablegen
```

## Templates: eingebettet vs. benutzerdefiniert

Die Standard-Templates liegen unter `internal/config/templates/*.tmpl` und
werden per `//go:embed templates/*.tmpl` fest in die Binary eingebettet
(siehe `internal/config/templates.go`). Damit funktioniert die App auch
als reine Einzeldatei ohne weitere Dateien auf der Platte.

Wer eigene Templates verwenden möchte (z. B. anderes DB-Schema, weitere
Tags, eigene Agent-Optionen), startet die App mit:

```
./brautomat-telegraf-gui --templates-dir /pfad/zu/eigenen/templates
```

Das Verzeichnis muss folgende Dateien enthalten (gleiche Namen wie die
Standard-Templates):

- `telegraf.conf.tmpl`
- `outputs-csv.conf.tmpl`
- `outputs-influxdb.conf.tmpl`
- `outputs-postgres.conf.tmpl`
- `outputs-mysql.conf.tmpl`

Fehlt eine Datei, meldet `GetTemplatesFS` beim Start einen klaren Fehler
statt später beim Rendern kryptisch abzubrechen.

Die Templates sind normale Go-`text/template`-Dateien und haben Zugriff
auf alle Felder von `config.Config` (z. B. `{{.DeviceURL}}`,
`{{.InfluxDB.Bucket}}`, `{{.Postgres.Password}}` etc.) — siehe
`internal/config/config.go` für das vollständige Modell.

## Bauen

Voraussetzungen: Go 1.22+, Node ist **nicht** nötig (kein JS-Build-Schritt,
das Frontend besteht aus reinem HTML/JS/CSS), sowie die Wails-CLI:

```
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Danach pro Zielplattform:

```
go mod tidy
wails build
```

Für Windows/macOS/Linux jeweils auf der Zielplattform bauen (oder mit
Wails' Cross-Compile-Unterstützung, siehe Wails-Dokumentation), und
vorher die passende Telegraf-Binary in `bin/` legen (siehe
`bin/README.md`).

## Entwicklung (Live-Reload)

```
wails dev
```

## Sicherheitshinweis zu Zugangsdaten

Aktuell werden die Zugangsdaten direkt in die generierte
`telegraf.d/outputs-*.conf` im temporären Arbeitsverzeichnis
(`os.MkdirTemp`) geschrieben, das beim Beenden der App wieder gelöscht
wird (`app.go`, `shutdown`). Für produktiven Einsatz empfiehlt es sich,
zusätzlich:

- die Dateirechte des Arbeitsverzeichnisses einzuschränken,
- optional einen Telegraf-Secret-Store (`secretstores.file` /
  `secretstores.os`) statt Klartext-Werten in den Templates zu nutzen,
- bzw. "Passwort merken" im Formular über die OS-Keychain zu realisieren
  (z. B. via `github.com/zalando/go-keyring`), statt Passwörter dauerhaft
  auf der Platte zu speichern.
