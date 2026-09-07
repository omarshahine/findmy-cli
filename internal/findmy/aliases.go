package findmy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func aliasPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "findmy-cli", "aliases.json")
}

func LoadAliases() map[string]string {
	data, err := os.ReadFile(aliasPath())
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil {
		return map[string]string{}
	}
	return m
}

func SaveAliases(m map[string]string) error {
	p := aliasPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// ResolveAlias returns the device name for a given alias (case-insensitive).
// If no alias matches, returns the input unchanged.
func ResolveAlias(input string) string {
	m := LoadAliases()
	key := strings.ToLower(strings.TrimSpace(input))
	for k, v := range m {
		if strings.ToLower(k) == key {
			return v
		}
	}
	return input
}
