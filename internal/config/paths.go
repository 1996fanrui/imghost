package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// ConfigRelPath is the path of the config file under the XDG config root.
const ConfigRelPath = "imghost/config.toml"

// StateDBName is the fixed bbolt database filename within state_dir.
const StateDBName = "imghost.db"

// stateRelPath is the path of the state db under the XDG state root.
const stateRelPath = "imghost/" + StateDBName

// configFilePath resolves the XDG config file path. xdg.ConfigFile creates
// parent directories if missing; this is acceptable during Load since any
// subsequent failure is surfaced explicitly.
func configFilePath() (string, error) {
	p, err := xdg.ConfigFile(ConfigRelPath)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return p, nil
}

// defaultStateDBPath returns the XDG state file for the bbolt database.
// It may create parent directories as a side effect (see design.md known
// side effects section).
func defaultStateDBPath() (string, error) {
	p, err := xdg.StateFile(stateRelPath)
	if err != nil {
		return "", fmt.Errorf("resolve state path: %w", err)
	}
	return p, nil
}

// expandTilde expands a leading "~" or "~/" to the user's home directory.
// It returns the input unchanged when no tilde is present.
func expandTilde(p string) (string, error) {
	if p == "" {
		return p, nil
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
