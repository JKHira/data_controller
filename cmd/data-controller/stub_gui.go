//go:build !gui
// +build !gui

package main

import (
	"fmt"
)

type FyneGUIApplication struct{}

func NewFyneGUIApplication(configPath string) (*FyneGUIApplication, error) {
	return &FyneGUIApplication{}, nil
}

func (a *FyneGUIApplication) Run() {
	fmt.Println("GUI mode not available in this build. Use -nogui flag or build with -tags gui")
}