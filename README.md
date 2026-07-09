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
│   │   ├── persistence.go           # Speichern/Laden der Config als JSON, Default-Pfad im Home-Verzeichnis
│   │   └── templates/               # Eingebettete Standard-Templates
│   │       ├── telegraf.conf.tmpl
│   │       ├── outputs-csv.conf.tmpl
│   │       ├── outputs-influxdb.conf.tmpl
│   │       ├── outputs-postgres.conf.tmpl
│   │       ├── outputs-mysql.conf.tmpl
│   │       └── outputs-mqtt.conf.tmpl
│   └── process/
│       ├── runner.go                # Start/Stop/Log-Streaming (plattformneutraler Teil)
│       ├── process_unix.go          # SIGTERM/SIGKILL, Prozessgruppen (Linux/macOS)
│       └── process_windows.go       # Kill() (Windows)
├── frontend/
│   ├── index.html                   # Formular (Ziele als Tabs) + Log-Fenster
│   └── src/
│       ├── main.js                  # Formular auslesen, Start/Stop, Speichern/Laden, Events anzeigen
│       ├── tabs.js                  # Reine UI-Logik: Tab-Umschaltung + Enabled-Indikator pro Ziel-Tab
│       └── style.css
└── bin/                              # Hier die telegraf-Binary pro Zielplattform ablegen
```

## Ausgabe leeren/speichern

Über dem Log-Fenster stehen zwei Buttons zur Verfügung:

- **Ausgabe leeren**: löscht den Inhalt des Log-Fensters rein clientseitig
  (kein Backend-Aufruf nötig).
- **Ausgabe speichern…**: öffnet einen nativen "Speichern unter"-Dialog
  und schreibt den kompletten aktuellen Inhalt des Log-Fensters als
  Textdatei (`SaveLog` in `app.go`).

## Log-Level für Wails-Konsolenausgaben

```
./brautomat-telegraf-gui --log-level debug
```

Steuert, ab welcher Schwere Wails-eigene Meldungen (Fenster-Lifecycle,
IPC-Bindings, interne Fehler etc.) auf der Konsole ausgegeben werden.
Gültige Werte: `trace`, `debug`, `info` (Default), `warning`, `error`
(Groß-/Kleinschreibung egal, `warn` als Alias für `warning`). Ein
ungültiger Wert zeigt - wie bei den anderen Flags - den Hilfetext und
beendet das Programm mit Exit-Code 2.

Das betrifft ausschließlich die Wails-Konsolenausgabe, **nicht** die
Telegraf-Ausgabe im Log-Fenster der GUI - die wird unabhängig vom
Log-Level immer vollständig angezeigt (siehe `telegraf:log`-Event in
`app.go`).

## Hilfe / verfügbare Flags

```
./brautomat-telegraf-gui --help
```

zeigt eine kurze Beschreibung des Programms sowie alle verfügbaren Flags
(`--templates-dir`, `--config`, `--export-templates`). Derselbe Hilfetext
erscheint automatisch auch bei einem ungültigen oder unbekannten Flag
bzw. Argument (Exit-Code 2 statt 0 in dem Fall) - man muss sich die
Optionen also nicht separat merken.

## Templates: eingebettet vs. benutzerdefiniert

Die Standard-Templates liegen unter `internal/config/templates/*.tmpl` und
werden per `//go:embed templates/*.tmpl` fest in die Binary eingebettet
(siehe `internal/config/templates.go`). Damit funktioniert die App auch
als reine Einzeldatei ohne weitere Dateien auf der Platte.

**In der GUI** gibt es dafür ein eigenes "Templates"-Panel:

- Checkbox **"Eigene Templates verwenden"** deaktiviert: es werden immer
  die eingebetteten Standard-Templates verwendet (Textfeld ist gesperrt).
- Checkbox aktiviert: das Verzeichnis im Textfeld (oder per
  **"Durchsuchen…"**-Button gewählt) wird stattdessen verwendet.
- Der gewählte Pfad ist Teil der Konfiguration (`templatesDir`-Feld) und
  wird beim "Speichern" mit gespeichert bzw. beim "Laden" mit geladen.
- Beim Programmstart wird das Feld mit dem Wert von `--templates-dir`
  vorbelegt (falls gesetzt) - das Formular kann diesen Wert danach
  jederzeit ändern, ohne die App neu zu starten.

Das Verzeichnis muss folgende Dateien enthalten (gleiche Namen wie die
Standard-Templates):

- `telegraf.conf.tmpl`
- `outputs-csv.conf.tmpl`
- `outputs-influxdb.conf.tmpl`
- `outputs-postgres.conf.tmpl`
- `outputs-mysql.conf.tmpl`
- `outputs-mqtt.conf.tmpl`

Fehlt eine Datei, meldet `GetTemplatesFS` beim Start von Telegraf einen
klaren Fehler statt später beim Rendern kryptisch abzubrechen.

**Eigene Templates als Ausgangspunkt exportieren:** Statt bei Null
anzufangen, lassen sich die eingebetteten Standard-Templates per
CLI-Kommando in ein Verzeichnis exportieren. Diese Variante startet
**nicht** die GUI, sondern exportiert nur und beendet sich sofort:

```
./brautomat-telegraf-gui --export-templates /pfad/zu/eigenen/templates
```

Anschließend die exportierten `.tmpl`-Dateien nach Bedarf anpassen und
entweder mit `--templates-dir` beim Start referenzieren oder direkt in
der GUI über "Durchsuchen…" auswählen.

```
./brautomat-telegraf-gui --templates-dir /pfad/zu/eigenen/templates
```

(Kombinierbar mit `--config`, siehe Abschnitt "Konfiguration speichern/laden" unten. Der Wert dient nur als initialer Vorschlag - die GUI kann ihn überschreiben.)

Die Templates sind normale Go-`text/template`-Dateien und haben Zugriff
auf alle Felder von `config.Config` (z. B. `{{.DeviceURL}}`,
`{{.InfluxDB.Bucket}}`, `{{.Postgres.Password}}` etc.) — siehe
`internal/config/config.go` für das vollständige Modell.

## Konfiguration speichern/laden

Das Formular kann als JSON gespeichert und wieder geladen werden (Panel
"Konfiguration"):

- **Konfiguration speichern**: schreibt unter den zuletzt verwendeten Pfad
  (beim allerersten Mal der effektive Standardpfad, siehe unten).
- **Konfiguration speichern unter…**: öffnet einen nativen Dateidialog,
  damit ein beliebiger Pfad gewählt werden kann.
- **Konfiguration laden…**: öffnet einen nativen Dateidialog zum Öffnen
  einer bestehenden `config.json`.

Beim Programmstart wird automatisch versucht, die Konfiguration vom
effektiven Standardpfad zu laden; existiert dort keine Datei, wird
stattdessen `config.Default()` verwendet (kein Fehler).

### Passwörter speichern

Standardmäßig **nicht** angehakt. Ist die Checkbox **"Passwörter
speichern"** deaktiviert, entfernt `Save()` (`internal/config/persistence.go`)
das InfluxDB-Token sowie die Postgres-/MySQL-Passwörter vor dem
Schreiben aus der Konfiguration - unabhängig davon, was im Formular
eingetragen war. Diese Durchsetzung sitzt bewusst im Backend, nicht nur
im Frontend.

Nach dem Laden einer so gespeicherten `config.json` sind die
entsprechenden Passwortfelder leer und müssen vor "Start" erneut
eingetragen werden. Ist die Checkbox aktiviert, werden alle Felder
inklusive Zugangsdaten im Klartext mitgespeichert (siehe Sicherheitshinweis
unten).

Der **effektive Standardpfad** ergibt sich in dieser Reihenfolge:

1. `--config <pfad>` beim Programmstart, falls gesetzt
2. sonst plattformübergreifend `~/.brautomat-telegraf-gui/config.json`
   (unter Windows entsprechend `%USERPROFILE%\.brautomat-telegraf-gui\config.json`,
   ermittelt über `os.UserHomeDir()` in `internal/config/persistence.go`)

```
./brautomat-telegraf-gui --config /pfad/zu/meiner/config.json
```

Das ist z. B. nützlich, um mehrere Geräte/Profile mit unterschiedlichen
Konfigurationsdateien zu betreiben, ohne jedes Mal manuell "Laden…"
anklicken zu müssen. Fehlende Verzeichnisse werden beim Speichern
automatisch angelegt.

**Hinweis:** Da die Konfiguration ggf. Klartext-Zugangsdaten enthält
(DB-Passwörter, InfluxDB-Token), wird die Datei mit den Rechten `0600`
(nur Besitzer lesbar/schreibbar) angelegt. Für höhere Sicherheit käme
später eine Verschlüsselung oder Anbindung an die OS-Keychain in Frage
(siehe Hinweis weiter unten).

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

Erste Absicherung: die "Passwörter speichern"-Checkbox (Default: aus) -
solange sie nicht aktiviert wird, landen InfluxDB-Token und DB-Passwörter
gar nicht erst in der persistierten `config.json` (siehe Abschnitt
"Konfiguration speichern/laden" oben).

Aktiviert der Benutzer diese Option dennoch, werden die Zugangsdaten
direkt in die generierte `telegraf.d/outputs-*.conf` im temporären
Arbeitsverzeichnis (`os.MkdirTemp`, wird beim Beenden der App wieder
gelöscht, siehe `shutdown` in `app.go`) sowie in die persistierte
`config.json` (Rechte `0600`) geschrieben. Für produktiven Einsatz
empfiehlt es sich zusätzlich:

- die Dateirechte des Arbeitsverzeichnisses einzuschränken,
- optional einen Telegraf-Secret-Store (`secretstores.file` /
  `secretstores.os`) statt Klartext-Werten in den Templates zu nutzen,
- bzw. "Passwort merken" im Formular über die OS-Keychain zu realisieren
  (z. B. via `github.com/zalando/go-keyring`), statt Passwörter dauerhaft
  auf der Platte zu speichern.
