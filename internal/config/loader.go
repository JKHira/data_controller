package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads YAML config at cfgPath, validates against schemaPath (JSON Schema),
// applies environment variable overrides and returns resulting map[string]interface{}.
//
// envMapping is optional; when nil a default mapping will be used.
func LoadConfig(cfgPath, schemaPath string, envMapping map[string]string) (map[string]interface{}, error) {
	// read YAML file
	yb, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// convert YAML -> JSON for validation
	var doc interface{}
	if err := yaml.Unmarshal(yb, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	// yaml.Unmarshal produces map[interface{}]interface{}; convert it
	jsonCompatible, err := toJSONCompatible(doc)
	if err != nil {
		return nil, fmt.Errorf("convert yaml->json compatible: %w", err)
	}
	jb, err := json.Marshal(jsonCompatible)
	if err != nil {
		return nil, fmt.Errorf("marshal to json: %w", err)
	}

	// validate against schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	documentLoader := gojsonschema.NewBytesLoader(jb)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}
	if !result.Valid() {
		var sb strings.Builder
		for _, e := range result.Errors() {
			sb.WriteString("- ")
			sb.WriteString(e.String())
			sb.WriteString("\n")
		}
		return nil, fmt.Errorf("config validation failed:\n%s", sb.String())
	}

	// unmarshal to map[string]interface{} for overrides
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(yb, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml to map: %w", err)
	}

	// default env mapping if nil
	if envMapping == nil {
		envMapping = map[string]string{
			"WEBSOCKET_URL":     "websocket.url",
			"STORAGE_BASE_PATH": "storage.base_path",
			"LOG_LEVEL":         "application.log_level",
			"PROM_PORT":         "monitoring.prometheus.port",
		}
	}

	applyEnvOverrides(cfg, envMapping)

	return cfg, nil
}

// applyEnvOverrides reads environment variables per mapping and sets dotted-paths in cfg.
func applyEnvOverrides(cfg map[string]interface{}, mapping map[string]string) {
	for env, path := range mapping {
		if v, ok := os.LookupEnv(env); ok && v != "" {
			// try to coerce numeric strings into numbers for port-like fields
			// but keep everything as string unless it clearly parses as int
			if i, err := tryParseInt(v); err == nil {
				setNestedField(cfg, path, i)
			} else {
				setNestedField(cfg, path, v)
			}
		}
	}
}

// setNestedField sets value at dotted path (e.g. "monitoring.prometheus.port") creating maps as needed.
func setNestedField(m map[string]interface{}, dotted string, value interface{}) {
	parts := strings.Split(dotted, ".")
	last := len(parts) - 1
	cur := m
	for i, p := range parts {
		if i == last {
			cur[p] = value
			return
		}
		next, exists := cur[p]
		if !exists {
			nm := make(map[string]interface{})
			cur[p] = nm
			cur = nm
			continue
		}
		switch typed := next.(type) {
		case map[string]interface{}:
			cur = typed
		default:
			// overwrite non-map with map to set deeper values
			nm := make(map[string]interface{})
			cur[p] = nm
			cur = nm
		}
	}
}

// tryParseInt attempts to parse string to int; returns error on failure.
func tryParseInt(s string) (int64, error) {
	s = strings.TrimSpace(s)
	// try plain integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	// try parsing as float and convert if it's an integer value like "123.0"
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if float64(int64(f)) == f {
			return int64(f), nil
		}
	}
	// fallback: attempt JSON number unmarshal (rare)
	var v int64
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v, nil
	}
	return 0, fmt.Errorf("not int")
}

// toJSONCompatible converts yaml-parsed structures (with map[interface{}]interface{}) into map[string]interface{} recursively.
func toJSONCompatible(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(val))
		for k, vv := range val {
			ks := fmt.Sprintf("%v", k)
			conv, err := toJSONCompatible(vv)
			if err != nil {
				return nil, err
			}
			m[ks] = conv
		}
		return m, nil
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, vv := range val {
			conv, err := toJSONCompatible(vv)
			if err != nil {
				return nil, err
			}
			arr[i] = conv
		}
		return arr, nil
	default:
		return val, nil
	}
}

