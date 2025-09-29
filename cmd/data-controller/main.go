package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	noGUI := flag.Bool("nogui", false, "Run without GUI")
	flag.Parse()

	resolvedPath, err := resolveConfigPath(*configPath)
	if err != nil {
		panic(fmt.Errorf("resolve config path: %w", err))
	}

	if *noGUI {
		// Run NoGUI version
		app, err := NewNoGUIApplication(resolvedPath)
		if err != nil {
			panic(err)
		}

		if err := app.Run(); err != nil {
			app.logger.Fatal("Application failed", zap.Error(err))
		}
	} else {
		// Run GUI version using new modular structure
		cfg, err := config.Load(resolvedPath)
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

// resolveConfigPath attempts to locate the configuration file using a couple of
// sensible fallbacks so that relocating configs under ./config/ works without
// additional CLI flags.
func resolveConfigPath(path string) (string, error) {
	// Try explicit path first (absolute or relative to CWD)
	if exists(path) {
		return path, nil
	}

	// Try ./config/<path>
	if candidate := filepath.Join("config", path); exists(candidate) {
		return candidate, nil
	}

	// Try alongside the executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, path)
		if exists(candidate) {
			return candidate, nil
		}
		candidate = filepath.Join(execDir, "config", path)
		if exists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("config file %s not found", path)
}

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
