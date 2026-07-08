package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// assetsRaw enthält den kompletten frontend/-Ordner (index.html, src/*).
// Es gibt hier bewusst keinen JS-Build-Schritt (kein Vite/Webpack) -
// die Dateien werden 1:1 eingebettet und ausgeliefert.
//
//go:embed all:frontend
var assetsRaw embed.FS

func main() {
	templatesDir := flag.String(
		"templates-dir",
		"",
		"Pfad zu einem Verzeichnis mit eigenen Telegraf-Config-Templates.\n"+
			"Muss die Dateien telegraf.conf.tmpl, outputs-csv.conf.tmpl,\n"+
			"outputs-influxdb.conf.tmpl, outputs-postgres.conf.tmpl und\n"+
			"outputs-mysql.conf.tmpl enthalten. Wird nichts angegeben,\n"+
			"werden die im Programm eingebetteten Standard-Templates verwendet.",
	)
	flag.Parse()

	// "frontend" als Wurzel des Asset-Servers verwenden, damit
	// index.html direkt unter "/" erreichbar ist.
	assets, err := fs.Sub(assetsRaw, "frontend")
	if err != nil {
		log.Fatalf("Frontend-Assets konnten nicht geladen werden: %v", err)
	}

	app := NewApp(*templatesDir)

	err = wails.Run(&options.App{
		Title:  "Brautomat Telegraf Wrapper",
		Width:  960,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
