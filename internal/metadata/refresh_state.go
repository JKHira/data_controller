package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// RefreshState tracks last refresh timestamps for exchange metadata endpoints.
type RefreshState struct {
	mu        sync.RWMutex
	Exchanges map[string]map[string]time.Time
}

// refreshStateFileModel is a YAML-friendly representation of RefreshState.
type refreshStateFileModel struct {
	Exchanges map[string]map[string]string `yaml:"exchanges"`
}

// LoadRefreshState loads the refresh state from the given YAML file. If the file
// does not exist it returns an empty state without error.
func LoadRefreshState(path string) (*RefreshState, error) {
	rs := &RefreshState{
		Exchanges: make(map[string]map[string]time.Time),
	}

	if path == "" {
		return rs, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return rs, nil
	}
	if err != nil {
		return nil, err
	}

	var fileModel refreshStateFileModel
	if err := yaml.Unmarshal(data, &fileModel); err != nil {
		return nil, err
	}

	for exchange, endpoints := range fileModel.Exchanges {
		if rs.Exchanges[exchange] == nil {
			rs.Exchanges[exchange] = make(map[string]time.Time)
		}
		for endpoint, value := range endpoints {
			if value == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, value); err == nil {
				rs.Exchanges[exchange][endpoint] = ts
			}
		}
	}

	return rs, nil
}

// Save writes the refresh state to the provided path in YAML format.
func (rs *RefreshState) Save(path string) error {
	if path == "" {
		return errors.New("state path is empty")
	}

	rs.mu.RLock()
	defer rs.mu.RUnlock()

	fileModel := refreshStateFileModel{
		Exchanges: make(map[string]map[string]string, len(rs.Exchanges)),
	}

	for exchange, endpoints := range rs.Exchanges {
		fileModel.Exchanges[exchange] = make(map[string]string, len(endpoints))
		for endpoint, ts := range endpoints {
			fileModel.Exchanges[exchange][endpoint] = ts.UTC().Format(time.RFC3339)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(&fileModel)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// LastRefresh returns the timestamp of the last refresh for the given exchange
// and endpoint. The boolean indicates whether a timestamp was recorded.
func (rs *RefreshState) LastRefresh(exchange, endpoint string) (time.Time, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if endpoints, ok := rs.Exchanges[exchange]; ok {
		ts, exists := endpoints[endpoint]
		return ts, exists
	}
	return time.Time{}, false
}

// Update sets the last refresh timestamp for the given exchange and endpoint.
func (rs *RefreshState) Update(exchange, endpoint string, ts time.Time) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.Exchanges == nil {
		rs.Exchanges = make(map[string]map[string]time.Time)
	}
	if rs.Exchanges[exchange] == nil {
		rs.Exchanges[exchange] = make(map[string]time.Time)
	}
	rs.Exchanges[exchange][endpoint] = ts.UTC()
}

// Snapshot returns a deep copy of the refresh state for safe iteration.
func (rs *RefreshState) Snapshot() map[string]map[string]time.Time {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	snapshot := make(map[string]map[string]time.Time, len(rs.Exchanges))
	for exchange, endpoints := range rs.Exchanges {
		snapshot[exchange] = make(map[string]time.Time, len(endpoints))
		for endpoint, ts := range endpoints {
			snapshot[exchange][endpoint] = ts
		}
	}
	return snapshot
}
