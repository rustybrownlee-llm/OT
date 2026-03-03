package cli

import (
	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
)

// resolveDBPathFromConfig reads the monitor config at configPath and returns
// the event_db_path value. Returns the default path if the config cannot be read.
func resolveDBPathFromConfig(configPath string) string {
	cfg, err := configparse.ParseLenient(configPath)
	if err != nil {
		return configparse.DefaultEventDBPath
	}
	if cfg.EventDBPath != "" {
		return cfg.EventDBPath
	}
	return configparse.DefaultEventDBPath
}

// effectiveDBPath returns the explicit DBPath if set, otherwise reads the
// event_db_path from the monitor config file.
func effectiveDBPath(g Globals) string {
	if g.DBPath != "" {
		return g.DBPath
	}
	return resolveDBPathFromConfig(g.ConfigPath)
}
