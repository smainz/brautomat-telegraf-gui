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
// der GUI installiert. Bei Bedarf hier zentral anheben - im Idealfall
// synchron mit TELEGRAF_VERSION in .woodpecker/release.yaml halten,
// auch wenn beide unabhängig voneinander funktionieren.
const Version = "1.39.1"

// downloadTimeout begrenzt, wie lange der Download maximal dauern darf,
// damit der Button nicht unbegrenzt hängt, falls dl.influxdata.com nicht
// erreichbar ist.
const downloadTimeout = 2 * time.Minute

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
	return filepath.Join(home, ".brautomat-telegraf-gui", "telegraf"), nil
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
// Executable zurück.
func DownloadAndExtract(destDir string) (string, error) {
	url, err := DownloadURL()
	if err != nil {
		return "", err
	}

	fmt.Println("DestDir: " + destDir)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("Verzeichnis %q konnte nicht angelegt werden: %w", destDir, err)
	}

	archivePath := filepath.Join(os.TempDir(), filepath.Base(url))
	if err := download(url, archivePath); err != nil {
		return "", err
	}
	fmt.Println("url: " + url)
	fmt.Println("archivePath: " + archivePath)
	//defer os.Remove(archivePath)

	if err := extract(archivePath, destDir); err != nil {
		return "", err
	}

	execPath, err := findExecutable(destDir, executableName())
	if err != nil {
		return "", err
	}
	fmt.Println("execPath: " + execPath)

	if runtime.GOOS != "windows" {
		if err := os.Chmod(execPath, 0o755); err != nil {
			return "", fmt.Errorf("Ausführungsrechte für %q konnten nicht gesetzt werden: %w", execPath, err)
		}
	}

	return execPath, nil
}

func download(url, destPath string) error {
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

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("Download konnte nicht gespeichert werden: %w", err)
	}
	return nil
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
