package main

import (
	"flag"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
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
		// Run GUI version using new modular structure
		cfg, err := config.Load(*configPath)
		if err != nil {
			panic(err)
		}

		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}

		if err := createGUIApp(logger, cfg); err != nil {
			logger.Fatal("GUI application failed", zap.Error(err))
		}
	}
}