// Command mock-server ist ein eigenständiger, minimaler Ersatz für ein
// echtes Brautomat-Gerät während der Entwicklung. Er beantwortet
// GET /telemetry mit demselben JSON-Format wie das echte Gerät (siehe
// README.md im Projekt-Root) und verändert die Werte bei jedem Aufruf
// ein Stück weiter, damit man Telegraf/die GUI ohne echte Hardware
// testen kann.
//
// Die genaue Simulationslogik (wann welcher Modus/Schritt aktiv ist, wie
// sich Temperaturen entwickeln) ist bewusst simpel gehalten und nicht als
// realistisches Brauprofil gedacht - es geht nur darum, dass sich Werte
// sichtbar verändern.
//
// Aufruf:
//
//	go run ./tools/mock-server
//	go run ./tools/mock-server --addr :9090
package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// telemetry entspricht 1:1 dem JSON-Format, das /telemetry auf dem
// echten Gerät liefert.
type telemetry struct {
	T        int64   `json:"t"`
	Mode     string  `json:"mode"`
	Step     int     `json:"step"`
	StepName string  `json:"stepName"`
	M        float64 `json:"m"`
	MT       float64 `json:"mt"`
	MP       int     `json:"mp"`
	S        float64 `json:"s"`
	ST       float64 `json:"st"`
	SP       int     `json:"sp"`
	H        float64 `json:"h"`
	HT       float64 `json:"ht"`
	HP       int     `json:"hp"`
	F        float64 `json:"f"`
	FT       float64 `json:"ft"`
}

type mashStep struct {
	name   string
	target float64
}

var mashSteps = []mashStep{
	{"Einmaischen 50°C", 50},
	{"Rast 62°C", 62},
	{"Rast 72°C", 72},
	{"Abmaischen 78°C", 78},
}

var modes = []string{"idle", "mash", "fermenter", "manual", "autotune"}

// simulator hält den simulierten Gerätezustand zwischen Requests.
type simulator struct {
	mu sync.Mutex

	tick    int64
	modeIdx int

	m, s, h, f float64 // aktuelle Temperaturen, entwickeln sich langsam Richtung Zielwert
}

func newSimulator() *simulator {
	return &simulator{m: 20, s: 20, h: 20, f: 20}
}

// approach nähert current in kleinen Schritten an target an (einfacher
// exponentieller Regler) und legt etwas Rauschen drauf, damit die Werte
// nicht "clean" aussehen.
func approach(current, target, rate float64) float64 {
	next := current + (target-current)*rate + (rand.Float64()-0.5)*0.3
	return math.Round(next*10000) / 10000
}

// powerFor simuliert eine simple Bang-Bang-Regelung: volle Leistung
// solange current spürbar unter target liegt, sonst aus. Ohne Zielwert
// (target <= 0) ist der Aktor aus.
func powerFor(current, target float64) int {
	if target <= 0 {
		return 0
	}
	if current < target-0.2 {
		return 100
	}
	return 0
}

// next berechnet den nächsten simulierten Messpunkt. Der Modus wechselt
// alle 20 Ticks reihum durch modes; innerhalb von "mash" wird alle 5
// Ticks der nächste Rastschritt aktiv.
func (s *simulator) next() telemetry {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tick++
	if s.tick%20 == 1 {
		s.modeIdx = (s.modeIdx + 1) % len(modes)
	}
	mode := modes[s.modeIdx]

	step := -1
	stepName := ""
	mt, st, ht, ft := 0.0, 0.0, 0.0, 0.0

	switch mode {
	case "mash":
		idx := int((s.tick / 5) % int64(len(mashSteps)))
		step = idx
		stepName = mashSteps[idx].name
		mt = mashSteps[idx].target
	case "fermenter":
		ft = 20
	}

	s.m = approach(s.m, mt, 0.08)
	s.s = approach(s.s, st, 0.05)
	s.h = approach(s.h, ht, 0.05)
	s.f = approach(s.f, ft, 0.02)

	return telemetry{
		T:        time.Now().Unix(),
		Mode:     mode,
		Step:     step,
		StepName: stepName,
		M:        s.m,
		MT:       mt,
		MP:       powerFor(s.m, mt),
		S:        s.s,
		ST:       st,
		SP:       powerFor(s.s, st),
		H:        s.h,
		HT:       ht,
		HP:       powerFor(s.h, ht),
		F:        s.f,
		FT:       ft,
	}
}

func main() {
	addr := flag.String(
		"addr",
		":8080",
		"Adresse, auf der der Mock-Server lauscht (z.B. :8080 oder 127.0.0.1:8080).",
	)
	flag.Parse()

	sim := newSimulator()

	mux := http.NewServeMux()
	mux.HandleFunc("/telemetry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "nur GET erlaubt", http.StatusMethodNotAllowed)
			return
		}

		data := sim.next()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Fehler beim Schreiben der Antwort: %v", err)
			return
		}
		log.Printf("GET /telemetry -> mode=%s step=%d stepName=%q t=%d", data.Mode, data.Step, data.StepName, data.T)
	})

	log.Printf("Mock-Brautomat-Server läuft auf http://localhost%s/telemetry", *addr)
	log.Printf("In der GUI als Geräte-URL z.B. http://localhost%s eintragen.", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
