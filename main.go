//go:build !cli

package main

import (
	"embed"
	"os"

	"github.com/kbuckham/mmcd/internal/cli"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// If subcommands are provided, run CLI mode
	if len(os.Args) > 1 {
		cli.Execute()
		return
	}

	// Otherwise, launch the Wails desktop app
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "MMCD Datalogger",
		Width:  1024,
		Height: 768,
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
		println("Error:", err.Error())
	}
}
