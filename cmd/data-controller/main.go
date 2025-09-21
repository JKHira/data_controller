package main

import (
	"flag"

	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	noGUI := flag.Bool("nogui", false, "Run without GUI")
	flag.Parse()

	if *noGUI {
		// Run NoGUI version
		app, err := NewNoGUIApplication(*configPath)
		if err != nil {
			panic(err)
		}

		if err := app.Run(); err != nil {
			app.logger.Fatal("Application failed", zap.Error(err))
		}
	} else {
		// Run Fyne GUI version
		app, err := NewFyneGUIApplication(*configPath)
		if err != nil {
			panic(err)
		}

		app.Run()
	}
}