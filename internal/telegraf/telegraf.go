// Package telegraf kapselt alles, was mit dem Herunterladen einer
// offiziellen telegraf-Executable zu tun hat: URL-Ermittlung für die
// aktuell laufende Plattform, Download, Entpacken (zip unter Windows,
// tar.gz unter Linux/macOS) und das Auffinden der eigentlichen
// Executable im entpackten Archiv - deren Layout unterscheidet sich
// zwischen den offiziellen Release-Archiven, daher wird sie per
// Verzeichnis-Suche gefunden statt ein fester Pfad angenommen.
//
// Bewusst ohne Wails-Bezug und nur mit Go-Standardbibliothek umgesetzt,
// damit es unabhängig testbar bleibt (siehe internal/process,
// internal/config für dasselbe Prinzip).
package telegraf

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Version ist die telegraf-Version, die "telegraf herunterladen…" in
// der GUI installiert. Bei Bedarf hier zentral anheben.
//
// WICHTIG: Wird Version geändert, MUSS die checksums-Map weiter unten
// mit den neuen, offiziell veröffentlichten SHA256-Werten aktualisiert
// werden - sonst schlägt der Download für JEDE Plattform mit einem
// Prüfsummenfehler fehl (das ist beabsichtigt, siehe expectedChecksum:
// lieber "fail closed" als eine nicht verifizierte Datei auszuführen).
const Version = "1.39.1"

// checksums enthält die von InfluxData offiziell veröffentlichten
// SHA256-Prüfsummen für Version, übernommen aus der "Packages"-Tabelle
// des GitHub-Release: https://github.com/influxdata/telegraf/releases/tag/v1.39.1
// (Stand der Prüfung: die dortige Tabelle ist die verlässlichste Quelle -
// auf influxdata.com/downloads sowie docs.influxdata.com waren beim
// Nachprüfen widersprüchliche/falsch zugeordnete SHA256-Werte für
// dieselbe Datei zu finden, vermutlich durch fehlerhafte
// Seiten-Generierung; die GitHub-Tabelle war in sich konsistent).
var checksums = map[string]string{
	"linux_amd64":   "d9194fb73fadc18f88d7d6649a2e018168028bedec1fdbc5fb655aaed120647a",
	"linux_arm64":   "ed46395c24c47f8360db9d1f0c8684640368879d2aa7fc41e6fe0f8a878990cd",
	"windows_amd64": "b68a1cd98c933d02fc5c1adcc2c0e1f19078e692dd47c47cdc122e552cb3b465",
	"windows_arm64": "0d452cf167a6f1c2d82b27b52c4cdd9783aa342925c6138091da0dc5a7438d57",
	"darwin_amd64":  "653c8a4b5afe66b0a6223952853de9f4d9ad4387b62858248fcad1ff4021e060",
	"darwin_arm64":  "cb0be878c76cf64d26da63ef77f9fa683ede2b1a79bcbfcbfed836bad16200e0",
}

// expectedChecksum liefert die für die aktuell laufende Plattform
// hinterlegte SHA256-Prüfsumme. Gibt es dafür keinen Eintrag (z.B. eine
// von der App aktuell nicht unterstützte Architektur), wird der Download
// bewusst abgelehnt, statt unverifiziert fortzufahren.
func expectedChecksum() (string, error) {
	key := runtime.GOOS + "_" + runtime.GOARCH
	sum, ok := checksums[key]
	if !ok {
		return "", fmt.Errorf(
			"keine bekannte Prüfsumme für %s (Version %s) hinterlegt - Download wird aus Sicherheitsgründen abgelehnt",
			key, Version,
		)
	}
	return sum, nil
}

// verifyChecksum berechnet die SHA256-Prüfsumme von path und vergleicht
// sie mit expected. Bei einer Abweichung wird ein Fehler zurückgegeben -
// der Aufrufer MUSS die Datei dann verwerfen und darf sie nicht
// entpacken/ausführen (siehe DownloadAndExtract).
func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Datei %q konnte für die Prüfsummenkontrolle nicht geöffnet werden: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("Prüfsumme konnte nicht berechnet werden: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf(
			"Prüfsumme stimmt nicht überein (erwartet %s, erhalten %s) - Download wird verworfen, da die Datei möglicherweise beschädigt oder manipuliert wurde",
			expected, actual,
		)
	}
	return nil
}

// downloadTimeout begrenzt, wie lange der gesamte Download (inkl.
// Lesen des Response-Bodys) maximal dauern darf, damit der Vorgang
// nicht unbegrenzt hängt, falls dl.influxdata.com nicht erreichbar
// ist oder die Verbindung abbricht. Bewusst großzügig bemessen, da
// die Archive ~100+ MB groß sein können.
const downloadTimeout = 10 * time.Minute

// StatusFunc empfängt kurze, für Menschen lesbare Meldungen zu den
// einzelnen Arbeitsschritten (z.B. "Lade herunter…", "Entpacke…"), die
// z.B. in einem Fortschrittsfenster angezeigt werden können.
type StatusFunc func(message string)

// ProgressFunc empfängt den Download-Fortschritt in Bytes, nach jedem
// gelesenen Chunk. total ist 0, falls der Server keine
// Content-Length-Angabe liefert - der Aufrufer kann dann keinen
// Prozentwert berechnen und sollte stattdessen z.B. nur die
// heruntergeladene Menge anzeigen.
type ProgressFunc func(downloaded, total int64)

// DownloadURL liefert die offizielle telegraf-Download-URL für das
// aktuell laufende Betriebssystem/Architektur.
func DownloadURL() (string, error) {
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("https://dl.influxdata.com/telegraf/releases/telegraf-%s_linux_%s.tar.gz", Version, arch), nil
	case "windows":
		return fmt.Sprintf("https://dl.influxdata.com/telegraf/releases/telegraf-%s_windows_%s.zip", Version, arch), nil
	case "darwin":
		return fmt.Sprintf("https://dl.influxdata.com/telegraf/releases/telegraf-%s_darwin_%s.tar.gz", Version, arch), nil
	default:
		return "", fmt.Errorf("nicht unterstütztes Betriebssystem %q für den telegraf-Download", runtime.GOOS)
	}
}

// InstallDir liefert das Verzeichnis, in das "telegraf herunterladen…"
// installiert: ~/.brautomat-telegraf-gui/telegraf
// (unter Windows entsprechend %USERPROFILE%\.brautomat-telegraf-gui\telegraf).
func InstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Home-Verzeichnis konnte nicht ermittelt werden: %w", err)
	}
	return filepath.Join(home, ".brautomat-telegraf-gui"), nil
}

// executableName ist der Dateiname der telegraf-Executable auf dem
// aktuell laufenden Betriebssystem.
func executableName() string {
	if runtime.GOOS == "windows" {
		return "telegraf.exe"
	}
	return "telegraf"
}

// DownloadAndExtract lädt telegraf für die aktuelle Plattform herunter,
// entpackt es nach destDir und liefert den Pfad zur eigentlichen
// Executable zurück. onStatus und onProgress dürfen jeweils nil sein.
func DownloadAndExtract(destDir string, onStatus StatusFunc, onProgress ProgressFunc) (string, error) {
	if onStatus == nil {
		onStatus = func(string) {}
	}
	if onProgress == nil {
		onProgress = func(int64, int64) {}
	}

	url, err := DownloadURL()
	if err != nil {
		return "", err
	}

	// Vor dem Download prüfen, ob für diese Plattform/Version überhaupt
	// eine Prüfsumme hinterlegt ist - lieber gar nicht erst herunterladen
	// als später eine unverifizierte Datei wieder zu verwerfen.
	expectedSum, err := expectedChecksum()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("Verzeichnis %q konnte nicht angelegt werden: %w", destDir, err)
	}

	archivePath := filepath.Join(os.TempDir(), filepath.Base(url))
	onStatus(fmt.Sprintf("Lade %s herunter…", filepath.Base(url)))
	if err := download(url, archivePath, onProgress); err != nil {
		return "", err
	}
	defer os.Remove(archivePath)

	onStatus("Prüfe Prüfsumme…")
	if err := verifyChecksum(archivePath, expectedSum); err != nil {
		return "", err
	}

	onStatus("Entpacke Archiv…")
	if err := extract(archivePath, destDir); err != nil {
		return "", err
	}

	onStatus("Suche telegraf-Executable im entpackten Archiv…")
	execPath, err := findExecutable(destDir, executableName())
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(execPath, 0o755); err != nil {
			return "", fmt.Errorf("Ausführungsrechte für %q konnten nicht gesetzt werden: %w", execPath, err)
		}
	}

	onStatus("Fertig.")
	return execPath, nil
}

func download(url, destPath string, onProgress ProgressFunc) error {
	client := http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Download von %s fehlgeschlagen: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Download von %s fehlgeschlagen: Status %s", url, resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Datei %q konnte nicht angelegt werden: %w", destPath, err)
	}
	defer f.Close()

	total := resp.ContentLength
	if total < 0 {
		total = 0 // unbekannt - Server liefert keine Content-Length
	}

	reader := &progressReader{r: resp.Body, onProgress: onProgress, total: total}
	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("Download konnte nicht gespeichert werden: %w", err)
	}
	return nil
}

// progressReader umhüllt einen io.Reader und meldet nach jedem Read()
// den bisher insgesamt gelesenen Byte-Wert an onProgress - so lässt
// sich der Download-Fortschritt verfolgen, ohne den kompletten Body
// vorher in den Speicher zu laden.
type progressReader struct {
	r          io.Reader
	onProgress ProgressFunc
	total      int64
	read       int64
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.read += int64(n)
		p.onProgress(p.read, p.total)
	}
	return n, err
}

func extract(archivePath, destDir string) error {
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGz(archivePath, destDir)
	default:
		return fmt.Errorf("unbekanntes Archivformat: %s", archivePath)
	}
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("Archiv %q konnte nicht geöffnet werden: %w", archivePath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		destPath := filepath.Join(destDir, f.Name)
		if !isWithinDir(destDir, destPath) {
			return fmt.Errorf("Archiv enthält einen unsicheren Pfad: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		if err := extractZipFile(f, destPath); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("Eintrag %q im Archiv konnte nicht geöffnet werden: %w", f.Name, err)
	}
	defer rc.Close()

	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("Datei %q konnte nicht angelegt werden: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("Eintrag %q konnte nicht entpackt werden: %w", f.Name, err)
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("Archiv %q konnte nicht geöffnet werden: %w", archivePath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip-Stream konnte nicht gelesen werden: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar-Eintrag konnte nicht gelesen werden: %w", err)
		}

		destPath := filepath.Join(destDir, hdr.Name)
		if !isWithinDir(destDir, destPath) {
			return fmt.Errorf("Archiv enthält einen unsicheren Pfad: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}
			if err := extractTarFile(tr, destPath, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractTarFile(tr *tar.Reader, destPath string, mode os.FileMode) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("Datei %q konnte nicht angelegt werden: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, tr); err != nil {
		return fmt.Errorf("Datei %q konnte nicht entpackt werden: %w", destPath, err)
	}
	return nil
}

// isWithinDir schützt vor "Zip Slip" (Archiveinträgen mit "../", die
// außerhalb von dir schreiben würden).
func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// findExecutable sucht rekursiv unter root nach einer Datei namens name
// und liefert den ersten Treffer zurück. Robust gegenüber
// unterschiedlichem Archiv-Layout zwischen Plattformen/Versionen.
func findExecutable(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if found != "" {
			return filepath.SkipAll
		}
		if !d.IsDir() && d.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("Suche nach %q in %q fehlgeschlagen: %w", name, root, err)
	}
	if found == "" {
		return "", fmt.Errorf("%q wurde im entpackten Archiv nicht gefunden", name)
	}
	return found, nil
}
