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
├── bin/                              # Hier die telegraf-Binary pro Zielplattform ablegen
└── tools/
    └── mock-server/
        └── main.go                   # Eigenständiger Mock für /telemetry (Entwicklung ohne echtes Gerät)
```

## Aufbau der Oberfläche

Die GUI ist in zwei Top-Level-Tabs aufgeteilt:

- **Main**: Geräte-URL/Abrufintervall (mit **"Testen"**-Button, der
  einen echten Request gegen `<Geräte-URL>/telemetry` ausführt - Erfolg
  wird inline angezeigt, ein Fehler öffnet ein Pop-up mit der Ursache),
  Start/Stop, Ausgabefenster (mit "Ausgabe leeren"/"Ausgabe speichern…"
  darunter). Das ist der Tab für den laufenden Betrieb.
- **Konfiguration**: alles, was seltener angefasst wird, in dieser
  Reihenfolge: Ziele (CSV/InfluxDB/PostgreSQL/MySQL/MQTT als
  Unter-Tabs), Templates-Konfiguration, Konfiguration speichern/laden.

Die Umschaltung sitzt in `frontend/src/tabs.js` (Top-Level-Tabs:
`.top-tab-btn`/`.top-tab-panel`; Ziele-Unter-Tabs: `.tab-btn`/`.tab-panel`
- bewusst getrennte Klassennamen, damit sich beide Tab-Ebenen nicht
  gegenseitig beeinflussen).

## Headless-Modus (ohne GUI)

```
./brautomat-telegraf-gui --start-headless
```

Liest die Konfiguration mit derselben Priorität wie beim normalen
GUI-Start (`--config` bzw. `~/.brautomat-telegraf-gui/config.json`,
existiert dort noch keine Datei: `config.Default()`) und startet
telegraf sofort damit - **ohne** dass ein Fenster erscheint. Die
Telegraf-Ausgabe landet direkt auf stdout, eigene Statusmeldungen (z.B.
"telegraf läuft im Headless-Modus...") auf stderr über das normale
`log`-Paket.

Das Programm läuft im Vordergrund, bis es mit **Ctrl+C** beendet wird;
dabei wird telegraf sauber gestoppt (Details siehe
`internal/process/process_unix.go` bzw. `process_windows.go`), bevor
sich das Programm beendet. Gedacht z.B. für den Betrieb auf einem
Server/Raspberry Pi ohne Desktop-Umgebung, oder um die aktuell
gespeicherte Konfiguration ohne GUI-Interaktion zu starten (etwa aus
einem eigenen Systemd-Unit/Autostart-Skript heraus).

Kombinierbar mit `--config` und `--templates-dir`:

```
./brautomat-telegraf-gui --start-headless --config /pfad/zu/meiner/config.json
```

**Zu Passwörtern:** Wurden beim letzten "Speichern" keine Passwörter mit
gespeichert (Checkbox "Passwörter speichern" war aus, siehe Abschnitt
"Konfiguration speichern/laden"), enthält die geladene Konfiguration für
die entsprechenden Ziele leere Passwortfelder - telegraf startet dann
mit diesen leeren Werten. Ein spezielles Verhalten für diesen Fall (z.B.
Abbruch mit Fehlermeldung, interaktive Passwortabfrage) ist aktuell noch
nicht vorgesehen.

## Mock-Server für die Entwicklung

Unter `tools/mock-server` liegt ein eigenständiger, minimaler Ersatz für
ein echtes Brautomat-Gerät. Er beantwortet `GET /telemetry` mit
demselben JSON-Format wie das echte Gerät, mit einem hochzählenden
Zeitstempel und Werten, die sich von Aufruf zu Aufruf sichtbar verändern
(Modus- und Rastschritt-Wechsel alle paar Ticks, Temperaturen nähern sich
langsam mit etwas Rauschen ihrem jeweiligen Zielwert an). Die genaue
Simulationslogik ist dabei bewusst simpel und nicht als realistisches
Brauprofil gedacht.

Start:

```
go run ./tools/mock-server
```

Optional ein anderer Port/Adresse:

```
go run ./tools/mock-server --addr :9090
```

Danach in der GUI als Geräte-URL `http://localhost:8080` (bzw. den
gewählten Port) eintragen. Läuft komplett unabhängig von der Wails-App -
kein `wails build`/`wails dev` nötig, nur `go run`.

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

**In der GUI** (Tab "Konfiguration") gibt es dafür ein eigenes
"Templates-Konfiguration"-Panel:

- Checkbox **"Eigene Templates verwenden"** deaktiviert: es werden immer
  die eingebetteten Standard-Templates verwendet; Pfad-Textfeld,
  "Durchsuchen…" und "Templates exportieren…" sind dabei komplett
  ausgeblendet.
- Checkbox aktiviert: Pfad-Textfeld, "Durchsuchen…" und "Templates
  exportieren…" werden eingeblendet; das Verzeichnis im Textfeld (oder
  per **"Durchsuchen…"**-Button gewählt) wird stattdessen verwendet.
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
anzufangen, lassen sich die eingebetteten Standard-Templates exportieren
- entweder direkt in der GUI über den Button **"Templates
exportieren…"** im Templates-Konfiguration-Panel (sichtbar bei
aktivierter "Eigene Templates verwenden"-Checkbox, öffnet einen nativen
Verzeichnis-Dialog), oder per CLI-Kommando. Die CLI-Variante startet
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

Das Formular kann als JSON gespeichert und wieder geladen werden (Tab
"Konfiguration", Panel "Konfiguration speichern"):

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
