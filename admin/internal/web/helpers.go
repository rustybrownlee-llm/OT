package web

import (
	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
)

// effectiveDBPath returns the explicit DBPath from globals if set, otherwise
// reads event_db_path from the monitor config file.
func effectiveDBPath(g Globals) string {
	if g.DBPath != "" {
		return g.DBPath
	}
	return resolveDBPathFromConfig(g.ConfigPath)
}

// resolveDBPathFromConfig reads the monitor config at configPath and returns
// the event_db_path value. Returns the default path if the config is unreadable.
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

// retentionDays returns the configured event retention window in days.
// Falls back to the default if the config cannot be read.
func retentionDays(g Globals) int {
	cfg, err := configparse.ParseLenient(g.ConfigPath)
	if err != nil {
		return configparse.DefaultEventRetentionDays
	}
	if cfg.EventRetentionDays > 0 {
		return cfg.EventRetentionDays
	}
	return configparse.DefaultEventRetentionDays
}
