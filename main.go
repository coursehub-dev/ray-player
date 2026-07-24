package main

import (
	"embed"
	"os"
	"runtime/debug"

	"ray-player1/internal/logx"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	debug.SetGCPercent(200)
	if path, err := logx.ConfigureFile(""); err != nil {
		appLog.W("file logging disabled: %v", err)
	} else {
		defer func() { _ = logx.CloseFile() }()
		appLog.I("file logging enabled path=%q", path)
	}
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Ray Player",
		Width:     1400,
		Height:    920,
		MinWidth:  980,
		MinHeight: 720,
		Frameless: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				FullSizeContent:            true,
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Acrylic,
		},
	})
	if err != nil {
		appLog.E("startup failed: %v", err)
		os.Exit(1)
	}
}
