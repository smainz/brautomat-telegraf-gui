package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// csvColumns MUSS exakt mit der Spaltenreihenfolge übereinstimmen, die
// outputs-csv.conf.tmpl über csv_columns festlegt (siehe dort). Wird
// stattdessen ein eigenes CSV-Template mit abweichenden Spalten
// verwendet (--templates-dir bzw. das Templates-Feld in der GUI), passt
// dieser vorab geschriebene Header nicht mehr automatisch dazu - dann
// müsste diese Liste hier entsprechend angepasst werden.
var csvColumns = []string{
	"timestamp",
	"mode",
	"stepName",
	"mash_temperature",
	"mash_target_temperature",
	"mash_power_percent",
	"boil_kettle_temperature",
	"boil_kettle_target_temperature",
	"boil_kettle_power_percent",
	"hlt_temperature",
	"hlt_target_temperature",
	"hlt_power_percent",
	"fermenter_temperature",
	"fermenter_target_temperature",
}

// EnsureCSVHeader schreibt die CSV-Kopfzeile in cfg.CSV.Path, falls das
// CSV-Ziel aktiviert ist und die Datei entweder noch nicht existiert
// oder leer ist (Größe 0). Ist die Datei bereits vorhanden und nicht
// leer, wird nichts verändert (kein doppelter Header, keine Daten
// überschrieben).
//
// telegraf selbst schreibt bewusst KEINEN Header (csv_header = false in
// outputs-csv.conf.tmpl) - würde man csv_header = true setzen, würde
// telegraf den Header bei jedem Flush erneut einfügen. Die einmalige
// Kopfzeile übernimmt stattdessen diese Funktion, aufgerufen bevor
// telegraf gestartet wird (siehe startTelegrafCore in app.go).
func EnsureCSVHeader(cfg Config) error {
	if !cfg.CSV.Enabled || cfg.CSV.Path == "" {
		return nil
	}

	needsHeader := true
	if info, err := os.Stat(cfg.CSV.Path); err == nil {
		needsHeader = info.Size() == 0
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("CSV-Datei %q konnte nicht geprüft werden: %w", cfg.CSV.Path, err)
	}

	if !needsHeader {
		return nil
	}

	if dir := filepath.Dir(cfg.CSV.Path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("Verzeichnis für CSV-Datei %q konnte nicht angelegt werden: %w", cfg.CSV.Path, err)
		}
	}

	f, err := os.OpenFile(cfg.CSV.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("CSV-Datei %q konnte nicht geöffnet werden: %w", cfg.CSV.Path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(strings.Join(csvColumns, ",") + "\n"); err != nil {
		return fmt.Errorf("CSV-Header konnte nicht in %q geschrieben werden: %w", cfg.CSV.Path, err)
	}
	return nil
}
