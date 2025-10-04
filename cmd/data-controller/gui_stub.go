//go:build !gui
// +build !gui

package main

import (
	"errors"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

// createGUIApp is a stub function when GUI is not enabled
func createGUIApp(logger *zap.Logger, cfg *config.Config) error {
	return errors.New("GUI support is not enabled in this build")
}
