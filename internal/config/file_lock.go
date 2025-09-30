package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// FileLock provides file-based mutual exclusion for config updates
type FileLock struct {
	path     string
	file     *os.File
	mu       sync.Mutex
	lockInfo LockInfo
}

// LockInfo contains metadata about the current lock holder
type LockInfo struct {
	LockedBy  string    `json:"locked_by"`
	LockedAt  time.Time `json:"locked_at"`
	Operation string    `json:"operation"`
	PID       int       `json:"pid"`
}

// NewFileLock creates a new file lock
func NewFileLock(lockDir string) *FileLock {
	lockPath := filepath.Join(lockDir, "update.lock")
	return &FileLock{
		path: lockPath,
	}
}

// Lock acquires an exclusive lock with timeout
func (fl *FileLock) Lock(operation string, timeout time.Duration) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(fl.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	// Open or create lock file
	file, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire lock with timeout
	deadline := time.Now().Add(timeout)
	for {
		// Try non-blocking lock
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			// Lock acquired
			fl.file = file
			fl.lockInfo = LockInfo{
				LockedBy:  "data-controller",
				LockedAt:  time.Now(),
				Operation: operation,
				PID:       os.Getpid(),
			}

			// Write lock info
			if err := fl.writeLockInfo(); err != nil {
				fl.Unlock()
				return fmt.Errorf("write lock info: %w", err)
			}

			return nil
		}

		// Check timeout
		if time.Now().After(deadline) {
			file.Close()
			return fmt.Errorf("lock timeout after %v", timeout)
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}
}

// Unlock releases the lock
func (fl *FileLock) Unlock() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file == nil {
		return nil // Not locked
	}

	// Release flock
	if err := unix.Flock(int(fl.file.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}

	// Close file
	if err := fl.file.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}
	fl.file = nil

	// Remove lock file
	os.Remove(fl.path)

	return nil
}

// writeLockInfo writes lock metadata to the file
func (fl *FileLock) writeLockInfo() error {
	if fl.file == nil {
		return fmt.Errorf("no lock file")
	}

	// Truncate file
	if err := fl.file.Truncate(0); err != nil {
		return err
	}

	// Seek to beginning
	if _, err := fl.file.Seek(0, 0); err != nil {
		return err
	}

	// Write JSON
	encoder := json.NewEncoder(fl.file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fl.lockInfo); err != nil {
		return err
	}

	// Sync to disk
	return fl.file.Sync()
}

// WithLock executes a function while holding the lock
func WithLock(lockDir, operation string, timeout time.Duration, fn func() error) error {
	lock := NewFileLock(lockDir)

	if err := lock.Lock(operation, timeout); err != nil {
		return err
	}
	defer lock.Unlock()

	return fn()
}