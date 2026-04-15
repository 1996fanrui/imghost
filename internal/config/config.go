// Package config loads and validates process configuration from environment.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/1996fanrui/imghost/internal/permission"
)

// DefaultDBPath is the hardcoded bbolt location per REQ-10R9.
const DefaultDBPath = "/var/lib/imghost/imghost.db"

const reservedDataDir = "/var/lib/imghost"

type Config struct {
	APIKey        string
	DefaultAccess permission.Access
	DataDir       string
	Port          int
	DBPath        string
}

func Load() (*Config, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY is required")
	}

	defAccessStr := os.Getenv("DEFAULT_ACCESS")
	if defAccessStr == "" {
		defAccessStr = string(permission.Public)
	}
	defAccess, err := permission.Parse(defAccessStr)
	if err != nil {
		return nil, fmt.Errorf("DEFAULT_ACCESS: %w", err)
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	if err := validateDataDir(dataDir); err != nil {
		return nil, err
	}

	port := 34286
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("PORT %q: not a number", v)
		}
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("PORT %d: must be in range 1..65535", p)
		}
		port = p
	}

	return &Config{
		APIKey:        apiKey,
		DefaultAccess: defAccess,
		DataDir:       dataDir,
		Port:          port,
		DBPath:        DefaultDBPath,
	}, nil
}

func validateDataDir(dir string) error {
	clean := filepath.Clean(dir)
	// Reject when DATA_DIR overlaps the hardcoded bbolt directory.
	if clean == reservedDataDir || strings.HasPrefix(clean, reservedDataDir+string(filepath.Separator)) {
		return fmt.Errorf("DATA_DIR %q must not equal or be under %s", dir, reservedDataDir)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("DATA_DIR %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("DATA_DIR %q is not a directory", dir)
	}
	f, err := os.CreateTemp(clean, ".imghost-write-probe-*")
	if err != nil {
		return fmt.Errorf("DATA_DIR %q not writable: %w", dir, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}
