# ToDo

- Ergänze einen Kommando `--start-headless`, dass nach dem Lesen der Konfiguration
  gleich startet, ohne die GUI anzuzeigen. Dabei soll der telegraf Prozess
  in der Konfiguration gestartet werden, die eingelesen wurde. Was passiert, wenn
  keine Passwörter gespeichert sind, überlege ich mir später.





## Done
- `--config` Flag einbauen
- Die Ziele sollen jeweils in einem eigenen Tab konfiguriert werden können. Die Funktionalität, mehrere Ziele einzuschalten soll beibehalten werden.
- Ergänze in der Oberfläche eine Konfiguration, um den Pfad zu den externen Templates anzugeben. Beachte, dass es eine möglichkeit geben muss, die interne Konfiguration zu verwenden. Diese Funktion soll auch per cli Kommando mit zusätzlicher Pfadangabe zugänglich sein. Wird dieses cli Komando verwendet, wird nur die templates exportiert und die GUI nicht gestartet.
- Füge eine Funktion hinzu, die Templates zu exportieren, damit man sie verändern kann.
- Stelle in der Oberfläche klar, dass Speichern, Speicher unter und Laden sich auf die Konfiguration beziehen.
- Ergänze eine Checkbox: "Passwörter speichern". Ist diese Checkbox gesetzt, dürfen die Passwörter in config.json gspeichert werden, ist es nicht gesetzt, werden die Passwörter nicht mit gespeichert. Default der Checkbox ist unchecked
- Füge ein Ziel MQTT hinzu, bei dem die Daten an einen MQTT-Server geschickt werden. Das umfasst: neue Konfigurationseinstellungen, neuens Konfigurations-Tab, neues telegraf Template
- Füge ein '--help' Flag hizu. Falls ein ungültiges Flag / Kommando in der cli angegeben wird, zeige den Hilfetext ebenfalls. Der Hilfetext oll nicht nur die Flags / cli Kommandos erklären, sondern auch kurz die Verwendung des Programms.
- Füge einen Button ein, mit dem man den Inhalt des Ausgabefensters löschen kann
- Füge einen Button ein, mit de man den Inhalt des Ausgabefensters speichern kann
- Flag für Log-Level hinzufügen, mit dem festgelegt werden kann, welche wails Log-Meldungen
  auf der Konsole ausgegeben werden.
- Füge einen Knopf zum exportieren der Templates hinzu
- Schreibe einen einfachen Mock-Server, der in der Entwicklung genutzt werden kann,
  um die Daten abzugragen. Timestamp soll sich hochzählen, einige Werte sich verändern. Die genaue Systematik ist dabei irrelevant.