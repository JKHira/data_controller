//go:build gui
// +build gui

package main

import (
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/gui/app"
)

// createGUIApp creates and initializes the GUI application using the new modular structure
func createGUIApp(logger *zap.Logger, cfg *config.Config) error {
	// Create new modular application
	guiApp := app.NewApplication(logger, cfg)

	// Initialize the application
	if err := guiApp.Initialize(); err != nil {
		return err
	}

	// Run the application (this blocks until the window is closed)
	guiApp.Run()

	return nil
}