# telegraf-Binaries

Lege hier die passende Telegraf-Binary für die jeweilige Zielplattform ab:

- Linux:   `bin/telegraf`
- macOS:   `bin/telegraf`
- Windows: `bin/telegraf.exe`

`app.go` (`findTelegrafBinary`) sucht zuerst hier (relativ zur eigenen
Executable) und fällt sonst auf den PATH zurück, falls der Benutzer
Telegraf selbst installiert hat.

Offizielle Downloads: https://www.influxdata.com/downloads/

Da bin/ pro Plattform unterschiedliche Binaries enthält, baue/paketiere
die App separat für jede Zielplattform (jeweils mit der passenden
Telegraf-Binary in diesem Ordner).
