package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"brautomat-telegraf-gui/internal/config"
)

func TestResolveConfigPath_ExplicitPathWinsOverFlag(t *testing.T) {
	a := &App{configPathFlag: "/flag/path.json"}
	got, err := a.resolveConfigPath("/explicit/path.json")
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	if got != "/explicit/path.json" {
		t.Errorf("resolveConfigPath() = %q, want expliziten Pfad", got)
	}
}

func TestResolveConfigPath_FlagWinsOverDefault(t *testing.T) {
	a := &App{configPathFlag: "/flag/path.json"}
	got, err := a.resolveConfigPath("")
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	if got != "/flag/path.json" {
		t.Errorf("resolveConfigPath() = %q, want --config-Flag-Pfad", got)
	}
}

func TestResolveConfigPath_FallsBackToDefault(t *testing.T) {
	a := &App{}
	got, err := a.resolveConfigPath("")
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	want, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatalf("config.DefaultConfigPath: %v", err)
	}
	if got != want {
		t.Errorf("resolveConfigPath() = %q, want Standardpfad %q", got, want)
	}
}

func TestFindTelegrafBinary_FallsBackToBareName(t *testing.T) {
	want := "telegraf"
	if isWindows() {
		want = "telegraf.exe"
	}

	got := findTelegrafBinary()
	if got == want {
		return
	}

	// Nur ein echter Testfehler, wenn nicht zufällig ein bin/-Ordner
	// neben der (temporären) Test-Binary liegt - dann ist der Fund
	// korrekt, aber dieser Test prüft absichtlich den Fallback-Fall.
	if exe, err := os.Executable(); err == nil {
		if _, statErr := os.Stat(filepath.Join(filepath.Dir(exe), "bin", want)); statErr == nil {
			t.Skip("neben der Test-Binary liegt zufällig ein bin/-Ordner mit telegraf - Fallback-Fall hier nicht testbar")
		}
	}
	t.Errorf("findTelegrafBinary() = %q, want %q (PATH-Fallback ohne bin/-Ordner)", got, want)
}

func TestFindTelegrafBinary_PrefersBinDirNextToExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable() nicht verfügbar: %v", err)
	}

	name := "telegraf"
	if isWindows() {
		name = "telegraf.exe"
	}
	binDir := filepath.Join(filepath.Dir(exe), "bin")
	candidate := filepath.Join(binDir, name)

	if _, err := os.Stat(candidate); err == nil {
		t.Skip("candidate existiert bereits neben der Test-Binary - übersprungen, um nichts Fremdes zu überschreiben")
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Skipf("bin/-Ordner neben der Test-Binary konnte nicht angelegt werden (evtl. keine Schreibrechte): %v", err)
	}
	if err := os.WriteFile(candidate, []byte("fake"), 0o755); err != nil {
		t.Skipf("Test-Binary konnte nicht angelegt werden: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(candidate)
		os.Remove(binDir) // schlägt fehl, falls nicht leer - ok, dann war er vorher schon da
	})

	got := findTelegrafBinary()
	if got != candidate {
		t.Errorf("findTelegrafBinary() = %q, want %q", got, candidate)
	}
}

func TestTestDeviceConnection_Success(t *testing.T) {
	a := &App{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/telemetry" {
			t.Errorf("unerwarteter Pfad: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"t":1700000000,"mode":"idle"}`)
	}))
	defer srv.Close()

	if err := a.TestDeviceConnection(srv.URL); err != nil {
		t.Errorf("TestDeviceConnection: %v", err)
	}
}

func TestTestDeviceConnection_EmptyURL(t *testing.T) {
	a := &App{}
	if err := a.TestDeviceConnection(""); err == nil {
		t.Error("erwarte Fehler bei leerer Geräte-URL")
	}
}

func TestTestDeviceConnection_InvalidURL(t *testing.T) {
	a := &App{}
	if err := a.TestDeviceConnection("not-a-url"); err == nil {
		t.Error("erwarte Fehler bei URL ohne Schema/Host")
	}
}

func TestTestDeviceConnection_WrongStatus(t *testing.T) {
	a := &App{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := a.TestDeviceConnection(srv.URL); err == nil {
		t.Error("erwarte Fehler bei Status 500")
	}
}

func TestTestDeviceConnection_InvalidJSON(t *testing.T) {
	a := &App{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "das ist kein JSON")
	}))
	defer srv.Close()

	err := a.TestDeviceConnection(srv.URL)
	if err == nil {
		t.Fatal("erwarte Fehler bei ungültigem JSON")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Fehlermeldung erwähnt nicht JSON: %v", err)
	}
}

func TestTestDeviceConnection_MissingTField(t *testing.T) {
	a := &App{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"mode":"idle"}`)
	}))
	defer srv.Close()

	err := a.TestDeviceConnection(srv.URL)
	if err == nil {
		t.Fatal(`erwarte Fehler, wenn das Feld "t" fehlt`)
	}
	if !strings.Contains(err.Error(), `"t"`) {
		t.Errorf(`Fehlermeldung erwähnt nicht das fehlende Feld "t": %v`, err)
	}
}

func TestTestDeviceConnection_Unreachable(t *testing.T) {
	a := &App{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // sofort schließen: der Port ist danach nicht mehr erreichbar

	if err := a.TestDeviceConnection(url); err == nil {
		t.Error("erwarte Fehler, wenn das Gerät nicht erreichbar ist")
	}
}
