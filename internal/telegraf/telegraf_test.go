package telegraf

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsWithinDir(t *testing.T) {
	dir := filepath.Join("some", "dir")
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"direktes Kind", filepath.Join(dir, "file.txt"), true},
		{"verschachteltes Kind", filepath.Join(dir, "sub", "file.txt"), true},
		{"identisch mit dir", dir, true},
		{"Zip-Slip via ..", filepath.Join(dir, "..", "evil.txt"), false},
		{"Zip-Slip via mehrere ..", filepath.Join(dir, "..", "..", "evil.txt"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWithinDir(dir, tt.path); got != tt.want {
				t.Errorf("isWithinDir(%q, %q) = %v, want %v", dir, tt.path, got, tt.want)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.bin")
	content := []byte("hello telegraf")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])

	if err := verifyChecksum(path, expected); err != nil {
		t.Errorf("verifyChecksum mit korrekter Prüfsumme lieferte einen Fehler: %v", err)
	}

	if err := verifyChecksum(path, strings.Repeat("0", 64)); err == nil {
		t.Error("verifyChecksum mit falscher Prüfsumme sollte fehlschlagen")
	}
}

func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Zip-Datei konnte nicht angelegt werden: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("Zip-Eintrag %q: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("Zip-Eintrag %q schreiben: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Zip schließen: %v", err)
	}
}

func TestExtractZip_NormalFile(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.zip")
	writeZip(t, archivePath, map[string]string{
		"telegraf-1.0/telegraf": "fake binary content",
	})

	destDir := filepath.Join(dir, "dest")
	if err := extractZip(archivePath, destDir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "telegraf-1.0", "telegraf"))
	if err != nil {
		t.Fatalf("entpackte Datei konnte nicht gelesen werden: %v", err)
	}
	if string(data) != "fake binary content" {
		t.Errorf("entpackter Inhalt = %q, want %q", string(data), "fake binary content")
	}
}

func TestExtractZip_ZipSlipProtection(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "evil.zip")
	writeZip(t, archivePath, map[string]string{
		"../evil.txt": "pwned",
	})

	destDir := filepath.Join(dir, "dest")
	err := extractZip(archivePath, destDir)
	if err == nil {
		t.Fatal("extractZip sollte bei einem Archiv mit \"../\"-Pfad einen Fehler liefern")
	}
	if !strings.Contains(err.Error(), "unsicheren Pfad") {
		t.Errorf("Fehlermeldung erwähnt nicht den unsicheren Pfad: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "evil.txt")); statErr == nil {
		t.Error("Zip-Slip-Datei wurde trotzdem außerhalb von destDir angelegt")
	}
}

func writeTarGz(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("tar.gz-Datei konnte nicht angelegt werden: %v", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar-Header %q: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar-Eintrag %q schreiben: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar schließen: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip schließen: %v", err)
	}
}

func TestExtractTarGz_NormalFile(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	writeTarGz(t, archivePath, map[string]string{
		"telegraf-1.0/telegraf": "fake binary content",
	})

	destDir := filepath.Join(dir, "dest")
	if err := extractTarGz(archivePath, destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "telegraf-1.0", "telegraf"))
	if err != nil {
		t.Fatalf("entpackte Datei konnte nicht gelesen werden: %v", err)
	}
	if string(data) != "fake binary content" {
		t.Errorf("entpackter Inhalt = %q, want %q", string(data), "fake binary content")
	}
}

func TestExtractTarGz_ZipSlipProtection(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "evil.tar.gz")
	writeTarGz(t, archivePath, map[string]string{
		"../evil.txt": "pwned",
	})

	destDir := filepath.Join(dir, "dest")
	err := extractTarGz(archivePath, destDir)
	if err == nil {
		t.Fatal("extractTarGz sollte bei einem Archiv mit \"../\"-Pfad einen Fehler liefern")
	}
	if !strings.Contains(err.Error(), "unsicheren Pfad") {
		t.Errorf("Fehlermeldung erwähnt nicht den unsicheren Pfad: %v", err)
	}
}

func TestExtract_UnknownFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.rar")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}
	if err := extract(path, filepath.Join(dir, "dest")); err == nil {
		t.Error("extract() mit unbekanntem Archivformat sollte fehlschlagen")
	}
}

func TestFindExecutable_Found(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}
	target := filepath.Join(nested, "telegraf")
	if err := os.WriteFile(target, []byte("bin"), 0o755); err != nil {
		t.Fatalf("Vorbereitung: %v", err)
	}

	got, err := findExecutable(dir, "telegraf")
	if err != nil {
		t.Fatalf("findExecutable: %v", err)
	}
	if got != target {
		t.Errorf("findExecutable() = %q, want %q", got, target)
	}
}

func TestFindExecutable_NotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := findExecutable(dir, "telegraf"); err == nil {
		t.Error("findExecutable sollte fehlschlagen, wenn die Datei nicht existiert")
	}
}

func TestDownloadURL_CurrentPlatform(t *testing.T) {
	url, err := DownloadURL()
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		if err != nil {
			t.Fatalf("DownloadURL: %v", err)
		}
		if !strings.Contains(url, "dl.influxdata.com") {
			t.Errorf("DownloadURL() = %q, erwarte dl.influxdata.com", url)
		}
		if !strings.Contains(url, Version) {
			t.Errorf("DownloadURL() = %q, erwarte Version %q enthalten", url, Version)
		}
	default:
		if err == nil {
			t.Errorf("DownloadURL() auf nicht unterstütztem GOOS %q sollte einen Fehler liefern", runtime.GOOS)
		}
	}
}

func TestExpectedChecksum_CurrentPlatform(t *testing.T) {
	key := runtime.GOOS + "_" + runtime.GOARCH
	want, known := checksums[key]

	sum, err := expectedChecksum()
	if !known {
		if err == nil {
			t.Errorf("expectedChecksum() für unbekannte Plattform %q sollte fehlschlagen (fail closed)", key)
		}
		return
	}
	if err != nil {
		t.Fatalf("expectedChecksum(): %v", err)
	}
	if sum != want {
		t.Errorf("expectedChecksum() = %q, want %q", sum, want)
	}
}
