package restapi

import (
	"os"
)

// createDirIfNotExists creates directory if it doesn't exist
func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// writeFile writes data to file
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
