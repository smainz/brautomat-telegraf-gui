# CLAUDE.md

Anweisungen für Claude Code zu diesem Projekt.

## Arbeitsweise (zuerst lesen)

- Lies diese Datei vollständig, bevor du größere Änderungen vornimmst.
- Es gibt eine Go-Testsuite (`go test ./...`, siehe „Build & Test"). Nach
  jeder Go-Änderung mindestens `go build ./...` und `go test ./...`
  laufen lassen, bevor du die Aufgabe als erledigt meldest.
- README.md und CLAUDE.md beschreiben teils dieselben Dinge aus
  unterschiedlicher Perspektive (Nutzer- vs. Architektursicht).
  Änderst du CLI-Flags, CI-Pipelines, Template-Dateien oder
  Konfigurationsverhalten, halte alle betroffenen Dateien konsistent -
  in diesem Repo ist Doku-Drift zwischen ihnen bereits mehrfach
  vorgekommen (z. B. veraltete CI-Beschreibung, unvollständige
  `--help`-Texte).
- Bei Punkten, die unten explizit als „Offene Frage" markiert sind: nicht
  eigenmächtig entscheiden, sondern beim Nutzer nachfragen.
- Halte dich an die Konventionen weiter unten, bevor du eigene neue
  Muster einführst - bei Widerspruch zwischen „naheliegender Lösung" und
  einer hier dokumentierten Konvention hat die Konvention Vorrang (bzw.
  kurz nachfragen, falls die Konvention veraltet wirkt).

## Was ist das

Ein minimaler Wails-v2-Desktop-Wrapper (Go-Backend, schlichtes HTML/JS-Frontend
ohne Build-Tool) für Telegraf. Zweck: Ein Gerät namens Brautomat stellt unter
`http://<host>/telemetry` alle paar Sekunden einen JSON-Messpunkt bereit.
Der Wrapper hat ein Formular, aus dem eine Telegraf-Config generiert wird,
und startet/stoppt den Telegraf-Prozess als Kindprozess. Telegraf selbst
schreibt die Messdaten dann wahlweise nach CSV, InfluxDB v2, PostgreSQL
und/oder MariaDB/MySQL – je nachdem, welche Ziele im Formular aktiviert sind.

Zielplattformen: Linux, macOS, Windows – nichts plattformspezifisches
außer den beiden Dateien in `internal/process/` (siehe unten) und der
mitgelieferten `telegraf`-Binary in `bin/`.

## Architektur (wichtig für Änderungen)

```
main.go                    Flag-Parsing (--templates-dir, --config, --export-templates, --log-level, --start-headless), printUsage() als flag.Usage (deckt --help/-h UND ungültige Flags/Argumente ab), runHeadless() für --start-headless, embed der frontend/-Assets, wails.Run()
app.go                      An das Frontend gebundene API: StartTelegraf, StopTelegraf, IsRunning, GetDefaults, GetDefaultConfigPath, SaveConfig, LoadConfig, ChooseSaveConfigPath, ChooseOpenConfigPath, ChooseTemplatesDir, ChooseExportTemplatesDir, ExportTemplates, ChooseSaveLogPath, SaveLog, TestDeviceConnection, ChooseTelegrafPath, ChooseTelegrafDownloadDir, DownloadTelegraf. Intern (nicht gebunden): startTelegrafCore(), initRuntimeState() - ctx-frei, auch vom Headless-Modus genutzt (siehe main.go)
internal/config/
  config.go                 Config-Struct = 1:1 das Formularmodell (JSON-Tags = Feldnamen im Frontend)
  templates.go              go:embed der Default-Templates + GetTemplatesFS(customDir) für --templates-dir
  templates/*.tmpl           Die 7 eingebetteten Standard-Templates (text/template-Syntax)
  generator.go               Rendert Templates -> telegraf.conf + telegraf.d/outputs-*.conf + processors-rename.conf (immer, zielunabhängig)
  csv_header.go              csvColumns (MUSS mit outputs-csv.conf.tmpl synchron sein) + EnsureCSVHeader()
  persistence.go             DefaultConfigPath() (~/.brautomat-telegraf-gui/config.json) + Save()/Load() als JSON; Save() entfernt Passwörter/Token, wenn cfg.SavePasswords false ist (Default)
internal/process/
  runner.go                  Plattformneutrale Prozesssteuerung (Start/Stop/Log-Streaming)
  process_unix.go             Prozessgruppen + SIGTERM/SIGKILL (build tag: !windows)
  process_windows.go          cmd.Process.Kill() (build tag: windows)
internal/telegraf/
  telegraf.go                 Download/Entpacken (zip/tar.gz, stdlib-only) + Suche der telegraf-Executable im Archiv, genutzt von App.DownloadTelegraf()
frontend/
  index.html + src/main.js    Formular in zwei Top-Level-Tabs ("Main": Gerät/Start-Stop/Ausgabe; "Konfiguration": Ziele/Templates/Speichern), ruft generierte wailsjs-Bindings auf
  src/tabs.js                 Reine UI-Logik: zwei unabhängige Tab-Ebenen (Top-Level .top-tab-btn/.top-tab-panel + Ziele-Unter-Tabs .tab-btn/.tab-panel), unabhängig von main.js
bin/                          Hier liegt (nach Download) die telegraf-Binary pro Zielplattform
tools/
  mock-server/main.go         Eigenständiger /telemetry-Mock für die Entwicklung (reines stdlib, kein Wails-Import)
docker-compose.yml             Lokale MQTT/Postgres/MariaDB/InfluxDB/Grafana-Testinstanzen (Datenbank/User "brautomat", MQTT_Server erfordert keine User/Passwort, InfluxDB via DOCKER_INFLUXDB_INIT_* vorkonfiguriert mit Org/Bucket "brautomat" + Token "brautomat-token", Grafana-Login "brautomat"/"brautomat" via GF_SECURITY_ADMIN_*, anonyme Volumes)
docker/grafana/provisioning/   Grafana-Provisioning: datasources/datasources.yml (Postgres/MySQL/InfluxDB vorkonfiguriert mit denselben Zugangsdaten wie in docker-compose.yml - bei Änderung der dortigen Zugangsdaten/Portnummern MUSS diese Datei mitgezogen werden, sonst driften Grafana-Datasources und tatsächliche Container-Credentials auseinander) + dashboards/files/*.json (je ein analoges Beispiel-Dashboard für Postgres/MySQL/InfluxDB, siehe Konventionen unten)
.woodpecker/
  build.yaml                   CI: Push -> "go test ./..." + Build-Check für linux/amd64 und windows/amd64 (kein telegraf, kein Upload). darwin/amd64 wird NICHT gebaut (Wails v2 kann von Linux aus nicht für macOS cross-kompilieren) - nur der plattformunabhängige tools/mock-server wird zusätzlich für alle drei Zielplattformen cross-kompiliert, als reine Zusatzprüfung
  release.yaml                 CI: Tag-Push (Branch main) -> "go test ./..." + Build für linux/amd64 + windows/amd64, packt je ein Archiv (tar.gz/zip) mit Binary + README.md und lädt beide als Forgejo-Release (git.mainz.ws) hoch. Enthält AKTUELL keinen telegraf-Download/Bundling-Schritt - die Release-Archive liefern also keine telegraf-Binary mit
```

**Datenfluss beim Klick auf "Start":**
1. Frontend sammelt Formularwerte in ein `Config`-Objekt (siehe `collectConfig()` in `main.js`) - inkl. `templatesDir` (leer, wenn "Eigene Templates verwenden" nicht angehakt ist)
2. `StartTelegraf(cfg)` in `app.go` wird aufgerufen (Wails-Binding) und ruft intern `startTelegrafCore()` auf
3. `startTelegrafCore()`: `config.GetTemplatesFS(cfg.TemplatesDir)` liefert entweder die eingebetteten Templates (leerer String) oder das im Formular angegebene Verzeichnis - das `--templates-dir`-Flag wirkt nur noch als initialer Vorschlagswert (siehe `GetDefaults`), nicht mehr als fixe Vorgabe
4. `config.Generate()` rendert `telegraf.conf` + je aktiviertem Ziel eine Datei unter `telegraf.d/`
5. `process.Runner.Start()` startet die `telegraf`-Binary mit `--config` und `--config-directory`
6. `StartTelegraf()` übergibt `startTelegrafCore()` dabei Callbacks, die stdout/stderr zeilenweise als `telegraf:log`-Event ans Frontend streamen

**Headless-Modus (`--start-headless`, `runHeadless()` in `main.go`):**
nutzt dieselbe `App`, aber `wails.Run()` läuft nie - es gibt also nie
einen echten `a.ctx`. Deshalb:
- `app.initRuntimeState()` (statt `startup()`) legt `workDir`/`telegrafPath`
  an, komplett ohne Wails-Bezug
- `app.startTelegrafCore()` wird direkt mit eigenen Callbacks aufgerufen
  (stdout/`log` statt `runtime.EventsEmit`), NICHT `StartTelegraf()`
  selbst, da das `runtime.EventsEmit(a.ctx, ...)` mit nil-Kontext aufrufen
  würde
- **Wichtig bei künftigen Änderungen an `StartTelegraf`/`startup`:** jede
  Logik, die auch im Headless-Modus gebraucht wird, gehört in
  `startTelegrafCore()`/`initRuntimeState()` (ctx-frei), nicht in
  `StartTelegraf()`/`startup()` selbst (die dürfen `a.ctx`
  voraussetzen, da sie nur über Wails aufgerufen werden)

**CLI-Sonderfall `--export-templates <pfad>`:** wird dieses Flag gesetzt,
exportiert `main.go` vor `wails.Run()` die eingebetteten Templates via
`config.ExportEmbeddedTemplates()` in das angegebene Verzeichnis und
beendet das Programm mit `os.Exit(0)`, ohne die GUI zu starten. Gedacht
als Ausgangspunkt für eigene Templates (danach anpassen und per
`--templates-dir` oder GUI-Feld referenzieren).

**Wichtig:** Deaktivierte Ziele werden vor dem Rendern explizit aus `telegraf.d/`
gelöscht (siehe `Generate()` in `generator.go`), da Telegraf jede `.conf`-Datei
im Verzeichnis liest – sonst würde ein zuvor aktiviertes Ziel weiterlaufen,
obwohl der Benutzer es im Formular abgewählt hat.

**Auflösung des Konfigurationspfads** (`resolveConfigPath` in `app.go`),
in dieser Priorität:
1. explizit übergebener Pfad (z.B. aus dem "Speichern unter…"-Dialog)
2. `--config <pfad>` beim Programmstart (`a.configPathFlag`)
3. `config.DefaultConfigPath()` = `~/.brautomat-telegraf-gui/config.json`

Wird dieselbe Logik in `GetDefaultConfigPath()`, `SaveConfig()`,
`LoadConfig()` sowie als Vorschlagswert in beiden Dateidialogen
verwendet – bei Änderungen an der Priorisierung bitte alle Stellen im
Blick behalten, da sie bewusst konsistent sein sollen.

## Build & Test

```bash
go mod tidy          # Abhängigkeiten holen (braucht Internet)
wails dev            # Live-Reload für Entwicklung
wails build           # Produktions-Binary bauen (Wails-CLI muss installiert sein)
```

Wails-CLI installieren, falls nicht vorhanden (Version an das in `go.mod`
gepinnte `github.com/wailsapp/wails/v2` angleichen, aktuell v2.13.0 - siehe
auch README.md/CI, die dieselbe Version verwenden):
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

Für einen lauffähigen Build muss vorher eine passende `telegraf`- bzw.
`telegraf.exe`-Binary unter `bin/` liegen.
Offizielle Downloads: https://www.influxdata.com/downloads/.
Es gibt auch eine Funktion im Programm, die die passende Version von telegraf runterlädt und entpackt.

```bash
go test ./...          # komplette Testsuite
go test ./... -short   # ohne den ~5s-SIGKILL-Eskalationstest (unix-only)
```

Deckt `internal/config`, `internal/telegraf`, `internal/process` sowie
`app.go` ab (Template-Rendering inkl. TOML-Escaping, CSV-Header, Config
speichern/laden, Zip-Slip-Schutz, Prozess-Lebenszyklus, Geräteverbindungstest)
- alles ohne Wails-Abhängigkeit, daher ohne besonderes Setup lauffähig.
`internal/process` baut sich dafür per `go build` einen winzigen
Test-Helferprozess (`internal/process/testdata/fakeproc`); `go` muss also
im `PATH` sein. Läuft auch in `.woodpecker/build.yaml`/`release.yaml` vor
dem eigentlichen Build.

**Ohne echtes Gerät entwickeln/testen:** `go run ./tools/mock-server`
startet einen minimalen `/telemetry`-Mock (reines stdlib, kein Wails-Bezug,
läuft unabhängig von `wails dev`/`wails build`). Danach in der GUI als
Geräte-URL z.B. `http://localhost:8080` eintragen. Nützlich, um Start/Stop,
Config-Rendering und alle Ziel-Tabs durchzutesten, ohne einen echten
Brautomat erreichbar zu haben.

## Konventionen / worauf beim Ändern zu achten ist

- **Feldnamen-Umbenennung** (`processors-rename.conf.tmpl`): benennt die
  kurzen Geräte-JSON-Felder (`m`, `mt`, `mp`, `s`, `st`, `sp`, `h`, `ht`,
  `hp`, `f`, `ft`) global für alle Ziele in sprechende Namen um (z.B.
  `m` -> `mash_temperature`). Wird in `generator.go` **immer** gerendert,
  unabhängig davon, welche Ziele aktiviert sind - anders als die
  `outputs-*.conf.tmpl`-Dateien, die nur bei aktiviertem Ziel entstehen.
  `t` ist bewusst NICHT Teil dieser Umbenennung, da `json_time_key = "t"`
  in `telegraf.conf.tmpl` das Feld bereits vorher als Zeitstempel der
  Metrik konsumiert - es existiert danach kein Feld `t` mehr.

- **CSV-Spaltenreihenfolge/-Header**: `outputs-csv.conf.tmpl` setzt
  `csv_columns` auf eine feste Liste (statt telegrafs alphabetischer
  Default-Sortierung) und `csv_header = false`. Die App schreibt den
  passenden Header stattdessen selbst, einmalig vor jedem Start (siehe
  `EnsureCSVHeader()` in `csv_header.go`, aufgerufen aus
  `startTelegrafCore()` in `app.go`, direkt nach `config.Generate()`).
  **Diese beiden Spaltenlisten müssen manuell synchron gehalten
  werden** - ändert sich eine, muss die andere entsprechend mitgezogen
  werden, sonst driften Header und tatsächliche Datenspalten auseinander.

- **Neues CLI-Flag hinzufügen**: reicht i.d.R. eine `flag.String/Bool/...`-
  Definition in `main()` mit gutem, mehrzeiligem Usage-Text (Konvention:
  `"\n"+` verkettete Strings wie bei den bestehenden Flags) - taucht dann
  automatisch in `--help` auf, da `printUsage()` am Ende `flag.PrintDefaults()`
  aufruft. Der einleitende Beschreibungstext in `printUsage()` muss nur
  angepasst werden, wenn sich das grundsätzliche Verwendungsmuster des
  Programms ändert, nicht bei jedem neuen Flag.

- **`--log-level` steuert nur die Wails-eigene Konsolenausgabe**
  (`options.App.LogLevel`/`LogLevelProduction`, siehe `parseLogLevel()`
  in `main.go`), nicht die Telegraf-Ausgabe im Log-Fenster der GUI - die
  läuft komplett separat über das `telegraf:log`-Event (`app.go`,
  `process.Runner`) und wird nie gefiltert. Bei Verwechslungsgefahr in
  Doku/Code bitte diese Trennung explizit machen.

- **Neues Ausgabeziel hinzufügen** (z. B. ein weiteres, hier noch nicht
  genanntes Ziel):
  1. Neues Template unter `internal/config/templates/outputs-<name>.conf.tmpl` anlegen
  2. Passendes Feld in `Config` (config.go) ergänzen (JSON-Tag beachten)
  3. Eintrag in der `targets`-Liste in `generator.go` (`Generate()`) ergänzen
  4. Neuen Tab-Button + Tab-Panel im "Ziele"-Bereich in `frontend/index.html` ergänzen (innerhalb des "Konfiguration"-Top-Level-Tabs, gleiches Muster wie CSV/InfluxDB/Postgres/MySQL/MQTT: `data-tab`/`data-tab-panel`, Checkbox mit Klasse `enable-toggle`) - NICHT mit den Top-Level-Tabs (`data-top-tab`/`data-top-tab-panel`) verwechseln
  5. Neuen Eintrag in `enabledCheckboxIdByTab` in `tabs.js` ergänzen, damit der Enabled-Indikator auch für das neue Ziel funktioniert
  6. `collectConfig()`/`applyConfig()` in `main.js` ergänzen
  7. `requiredTemplateFiles` in `templates.go` erweitern, damit `--templates-dir`-Validierung greift

- **Templates sind normale `text/template`-Dateien** mit Zugriff auf alle
  Felder von `config.Config` (z. B. `{{.InfluxDB.Bucket}}`). Nicht mit
  Wails-eigenen Template-Mechanismen verwechseln – das ist reines Go stdlib.

- **Plattformspezifischer Code gehört ausschließlich** in
  `process_unix.go` / `process_windows.go` (per Build-Tag getrennt). Bitte
  keine `runtime.GOOS`-Verzweigungen in `runner.go` einbauen, das würde
  die saubere Trennung aufweichen.

- **Kein Frontend-Build-Tool** (bewusst kein Vite/Webpack/npm-Toolchain).
  Reines HTML/JS/CSS unter `frontend/`. Die `wailsjs/`-Bindings werden von
  `wails build`/`wails dev` automatisch generiert – vor dem ersten Build
  existieren sie noch nicht, das ist kein Fehler.

- **Zugangsdaten** landen aktuell im Klartext in der generierten Config im
  temporären Arbeitsverzeichnis (`os.MkdirTemp`, wird beim Beenden gelöscht,
  siehe `shutdown()` in `app.go`) **und**, falls die Checkbox "Passwörter
  speichern" aktiviert ist (`cfg.SavePasswords`, Default `false`), in der
  persistierten `~/.brautomat-telegraf-gui/config.json` (0600-Rechte,
  siehe `persistence.go`). Betroffen: InfluxDB-Token, Postgres-/MySQL-/
  MQTT-Passwort (siehe `stripSecrets` in `persistence.go`). Die
  Durchsetzung von `SavePasswords` sitzt bewusst in `Save()` selbst, nicht
  nur im Frontend - bei Änderungen an dieser Logik nicht versehentlich nur
  die Frontend-Seite anpassen. Bei weiteren Änderungen an diesem Bereich
  die Sicherheitshinweise in `README.md` beachten (Secret-Store,
  OS-Keychain).

- **Telegraf-Pfad** (`cfg.TelegrafPath`): hat in `startTelegrafCore()`
  Vorrang vor der beim Start automatisch ermittelten `a.telegrafPath`
  (`findTelegrafBinary()`). Leerer String im Formular = automatische
  Erkennung verwenden. `App.ChooseTelegrafDownloadDir()` fragt per
  Dialog nach einem Zielverzeichnis (Vorschlag: `teledownload.InstallDir()`
  = `~/.brautomat-telegraf-gui/telegraf`), `App.DownloadTelegraf(destDir)`
  nutzt `internal/telegraf` zum Herunterladen/Entpacken dorthin und
  liefert den gefundenen Executable-Pfad zurück, den `main.js` dann
  selbst ins Formularfeld einträgt - das Backend setzt `cfg.TelegrafPath`
  nicht selbst. Zwischenzustände (Download/Entpacken/Suche) und
  Byte-Fortschritt laufen währenddessen über die Events
  `telegraf-download:status` / `telegraf-download:progress` (analog zu
  `telegraf:log`/`telegraf:status` beim Start von telegraf selbst) -
  `internal/telegraf.DownloadAndExtract()` nimmt dafür `StatusFunc`/
  `ProgressFunc`-Callbacks entgegen, die in `app.go` auf
  `runtime.EventsEmit` gemappt werden. Nach dem Download wird die
  SHA256-Prüfsumme des Archivs gegen `checksums` in `telegraf.go`
  geprüft (fail closed: unbekannte Plattform/Version -> Download wird
  abgelehnt; Mismatch -> Datei wird verworfen, nicht entpackt). **Wird
  `Version` in `telegraf.go` angehoben, MUSS `checksums` mit den neuen,
  offiziellen SHA256-Werten aus der GitHub-Release-Tabelle
  (`https://github.com/influxdata/telegraf/releases/tag/vX.Y.Z`)
  synchron aktualisiert werden** - sonst schlägt der Download für jede
  Plattform fehl. Nicht die Werte von influxdata.com/downloads oder
  docs.influxdata.com übernehmen, ohne sie gegen die GitHub-Tabelle
  gegenzuprüfen - dort wurden beim Erstellen dieser Prüfung
  widersprüchliche/falsch zugeordnete Werte für dieselbe Datei
  gefunden.

- **TelegrafLogLevel** (`cfg.TelegrafLogLevel`, Werte `"quiet"`/`"info"`/
  `"debug"`, Default `"info"`): steuert telegrafs eigene
  Log-Ausführlichkeit im `[agent]`-Block, gemappt auf `debug`/`quiet` in
  `telegraf.conf.tmpl` (telegraf hat kein einzelnes "Level"-Feld). NICHT
  verwechseln mit `--log-level` in `main.go`, das ausschließlich Wails'
  eigene Konsolenausgabe betrifft (`options.App.LogLevel`) - beide
  Settings sind unabhängig voneinander und beeinflussen unterschiedliche
  Ausgaben.

- **Grafana-Dashboards** (`docker/grafana/provisioning/dashboards/files/*.json`):
  drei inhaltlich analoge Dashboards (Postgres/MySQL/InfluxDB), referenzieren
  ihre Datasource über die feste `uid` aus `datasources.yml` (`postgres-brautomat`/
  `mysql-brautomat`/`influxdb-brautomat`) - bei Änderung dieser uids müssen alle
  drei JSONs mitgezogen werden. Postgres/MySQL nutzen `outputs.sql` und damit
  dieselbe Tabellenstruktur (Spalte `timestamp`, siehe README.md); die
  Modus-/Rastschritt-Wechsel-Annotation dort nutzt `LAG()` (Postgres: `IS DISTINCT
  FROM`, MySQL/MariaDB: `<=>`, da `IS DISTINCT FROM` dort nicht existiert -
  MySQL-seitig zudem Backtick- statt Anführungszeichen-Quoting, da Grafanas
  MySQL-Verbindung anders als Telegrafs `outputs.sql`-Verbindung kein
  `ANSI_QUOTES` gesetzt hat). Das InfluxDB-Dashboard nutzt Flux-Queries
  (`from(bucket: v.bucket) |> range(...) |> filter(...)`) und hat **bewusst
  keine** entsprechende Annotation, da Flux keine einfache Standardfunktion
  bietet, um Tag-Wechsel zwischen aufeinanderfolgenden Punkten zu erkennen
  (das State-Timeline-Panel oben zeigt Modus/Rastschritt trotzdem korrekt,
  da es dafür keine Wechsel-Erkennung braucht). Alle Zeitreihen-Panels setzen
  `fieldConfig.defaults.custom.axisWidth` fest (statt "Auto"), damit die
  Graphen unabhängig von der Textlänge der Achsenbeschriftung untereinander
  bündig sind - das State-Timeline-Panel unterstützt `axisWidth` nicht
  (Grafana-Einschränkung), bleibt daher unangepasst.

- **"Testen"-Button/Verbindungstest** (`TestDeviceConnection` in `app.go`):
  führt einen echten HTTP-GET gegen `<deviceUrl>/telemetry` aus (Timeout
  `deviceTestTimeout`, aktuell 5s) und liefert bei Erfolg `nil`, sonst
  einen für Menschen lesbaren Fehlertext. Das Frontend zeigt Erfolg
  inline neben dem Button, Fehler im generischen Pop-up-Modal
  (`#errorModalOverlay` in `index.html`, `showErrorModal()`/
  `hideErrorModal()` in `main.js`). Das Modal ist bewusst generisch
  gehalten - für künftige Fehlermeldungen, die ein Pop-up statt einer
  Logzeile verdienen, dieselben Funktionen wiederverwenden statt ein
  neues Modal zu bauen.

- **Offene Frage (noch nicht entschieden):** Wie sich `--start-headless`
  verhalten soll, wenn die geladene Konfiguration keine Passwörter enthält
  (weil "Passwörter speichern" beim letzten Speichern aus war) - aktuell
  startet telegraf einfach mit leeren Werten, ohne Warnung oder
  Sonderbehandlung. Vor einer Änderung hier Rücksprache halten, das ist
  bewusst offengelassen worden.

- **CI-Pipelines** (`.woodpecker/build.yaml`, `.woodpecker/release.yaml`):
  Beide Dateien bestehen jeweils aus **einem** Workflow mit einem
  einzigen `build`-Schritt auf demselben `golang:1.25-bookworm`-Image
  (kein `platform`-Feld, keine separaten Windows/macOS-Agenten - es
  steht laut den Kommentaren in `release.yaml` nur ein Linux-Docker-Agent
  zur Verfügung). Dieser eine Schritt ruft `wails build -platform ...`
  innerhalb desselben Containers nacheinander für `linux/amd64` und
  `windows/amd64` auf (Cross-Compiling für darwin/macOS wird von Wails v2
  nicht unterstützt und daher nirgends für die eigentliche App
  durchgeführt - `build.yaml` cross-kompiliert zusätzlich nur den
  plattformunabhängigen `tools/mock-server` für alle drei Plattformen,
  als reine Zusatzprüfung ohne echten GUI-Build). `release.yaml` lädt
  KEIN telegraf herunter und bündelt keines - es packt lediglich die
  gebaute Binary + `README.md` pro Plattform in ein Archiv (tar.gz/zip)
  und lädt beide über `woodpeckerci/plugin-release` auf `git.mainz.ws`
  hoch; der benötigte Forgejo-Token wird als Woodpecker-Secret
  `forgejo_token` erwartet (Name bei Bedarf in den `publish`-Schritten
  anpassen).

## Nicht tun

- Keine Business-Logik in `main.go` – das bleibt reines Wiring (Flags, Embed, `wails.Run`).
- Kein `localStorage`/`sessionStorage` im Frontend (in Wails-Webviews nicht zuverlässig nutzbar) – Zustand lebt im DOM bzw. wird bei Bedarf über `GetDefaults()`/Backend gehalten.
- `internal/config` und `internal/process` sollten nicht von Wails-spezifischen Paketen abhängen – das hält sie unabhängig testbar und wiederverwendbar (z. B. für eine spätere CLI-only-Variante ohne GUI).
