// Package web provides the HTTP server for the admin web dashboard.
package web

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DesignCache holds summarised metadata from the design library YAML files.
// Loaded once at startup; read-only after initialisation.
//
// PROTOTYPE-DEBT: [td-admin-111] No refresh mechanism. File changes require restart.
type DesignCache struct {
	Devices      []DeviceSummary
	Networks     []NetworkSummary
	Environments []EnvSummary
	PortDevices  []PortDevice
}

// DeviceSummary holds the display fields extracted from a device atom YAML.
type DeviceSummary struct {
	ID       string
	Vendor   string
	Model    string
	Category string
	Vintage  int
}

// NetworkSummary holds the display fields extracted from a network atom YAML.
type NetworkSummary struct {
	ID     string
	Type   string
	Subnet string
}

// EnvSummary holds the display fields extracted from an environment directory.
type EnvSummary struct {
	ID          string
	Name        string
	DeviceCount int
	NetworkCount int
	HasProcess  bool
}

// PortDevice maps one Modbus TCP port to the placement that exposes it.
// Used by health cards to label ports by device ID rather than port number.
type PortDevice struct {
	Port     int
	PlaceID  string
	DeviceID string
	Category string
}

// minimal YAML structs used only for metadata extraction.

type rawDeviceAtom struct {
	Device struct {
		ID       string `yaml:"id"`
		Vendor   string `yaml:"vendor"`
		Model    string `yaml:"model"`
		Category string `yaml:"category"`
		Vintage  int    `yaml:"vintage"`
	} `yaml:"device"`
}

type rawNetworkAtom struct {
	Network struct {
		ID   string `yaml:"id"`
		Type string `yaml:"type"`
	} `yaml:"network"`
	Properties struct {
		Subnet string `yaml:"subnet"`
	} `yaml:"properties"`
}

type rawEnvironment struct {
	Environment struct {
		ID   string `yaml:"id"`
		Name string `yaml:"name"`
	} `yaml:"environment"`
	Networks []struct {
		Ref string `yaml:"ref"`
	} `yaml:"networks"`
	Placements []struct {
		ID         string `yaml:"id"`
		Device     string `yaml:"device"`
		ModbusPort int    `yaml:"modbus_port"`
	} `yaml:"placements"`
}

// loadDesignCache scans the design directory and builds a DesignCache.
// If designDir is empty or does not exist, an empty cache is returned.
func loadDesignCache(designDir string) *DesignCache {
	c := &DesignCache{}
	if designDir == "" {
		return c
	}
	if _, err := os.Stat(designDir); errors.Is(err, os.ErrNotExist) {
		slog.Warn("design directory not found; design library unavailable", "dir", designDir)
		return c
	}

	deviceCategories := loadDeviceSummaries(c, filepath.Join(designDir, "devices"))
	loadNetworkSummaries(c, filepath.Join(designDir, "networks"))
	loadEnvSummaries(c, filepath.Join(designDir, "environments"), deviceCategories)

	sort.Slice(c.Devices, func(i, j int) bool { return c.Devices[i].ID < c.Devices[j].ID })
	sort.Slice(c.Networks, func(i, j int) bool { return c.Networks[i].ID < c.Networks[j].ID })
	sort.Slice(c.Environments, func(i, j int) bool { return c.Environments[i].ID < c.Environments[j].ID })
	sort.Slice(c.PortDevices, func(i, j int) bool { return c.PortDevices[i].Port < c.PortDevices[j].Port })

	slog.Info("design cache loaded",
		"devices", len(c.Devices),
		"networks", len(c.Networks),
		"environments", len(c.Environments),
		"port_mappings", len(c.PortDevices),
	)
	return c
}

// loadDeviceSummaries reads device atom YAML files and populates c.Devices.
// Returns a map of device ID to category for use in environment scanning.
func loadDeviceSummaries(c *DesignCache, devicesDir string) map[string]string {
	cats := make(map[string]string)
	entries, err := os.ReadDir(devicesDir)
	if err != nil {
		slog.Warn("cannot read devices directory", "path", devicesDir, "error", err)
		return cats
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(devicesDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("cannot read device atom", "path", path, "error", err)
			continue
		}
		var atom rawDeviceAtom
		if err := yaml.Unmarshal(raw, &atom); err != nil {
			slog.Warn("malformed device YAML; skipping", "path", path, "error", err)
			continue
		}
		if atom.Device.ID == "" {
			atom.Device.ID = strings.TrimSuffix(e.Name(), ".yaml")
		}
		c.Devices = append(c.Devices, DeviceSummary{
			ID:       atom.Device.ID,
			Vendor:   atom.Device.Vendor,
			Model:    atom.Device.Model,
			Category: atom.Device.Category,
			Vintage:  atom.Device.Vintage,
		})
		cats[atom.Device.ID] = atom.Device.Category
	}
	return cats
}

// loadNetworkSummaries reads network atom YAML files and populates c.Networks.
func loadNetworkSummaries(c *DesignCache, networksDir string) {
	entries, err := os.ReadDir(networksDir)
	if err != nil {
		slog.Warn("cannot read networks directory", "path", networksDir, "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(networksDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("cannot read network atom", "path", path, "error", err)
			continue
		}
		var atom rawNetworkAtom
		if err := yaml.Unmarshal(raw, &atom); err != nil {
			slog.Warn("malformed network YAML; skipping", "path", path, "error", err)
			continue
		}
		if atom.Network.ID == "" {
			atom.Network.ID = strings.TrimSuffix(e.Name(), ".yaml")
		}
		subnet := atom.Properties.Subnet
		if subnet == "" {
			subnet = "-"
		}
		c.Networks = append(c.Networks, NetworkSummary{
			ID:     atom.Network.ID,
			Type:   atom.Network.Type,
			Subnet: subnet,
		})
	}
}

// loadEnvSummaries reads environment directories and populates c.Environments
// and c.PortDevices (for health card port-to-device mapping).
func loadEnvSummaries(c *DesignCache, envsDir string, deviceCategories map[string]string) {
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		slog.Warn("cannot read environments directory", "path", envsDir, "error", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		envPath := filepath.Join(envsDir, e.Name(), "environment.yaml")
		raw, err := os.ReadFile(envPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("cannot read environment.yaml", "path", envPath, "error", err)
			}
			continue
		}
		var env rawEnvironment
		if err := yaml.Unmarshal(raw, &env); err != nil {
			slog.Warn("malformed environment YAML; skipping", "path", envPath, "error", err)
			continue
		}
		if env.Environment.ID == "" {
			env.Environment.ID = e.Name()
		}
		processPath := filepath.Join(envsDir, e.Name(), "process.yaml")
		_, processErr := os.Stat(processPath)
		hasProcess := processErr == nil

		c.Environments = append(c.Environments, EnvSummary{
			ID:           env.Environment.ID,
			Name:         env.Environment.Name,
			DeviceCount:  len(env.Placements),
			NetworkCount: len(env.Networks),
			HasProcess:   hasProcess,
		})
		for _, p := range env.Placements {
			if p.ModbusPort == 0 {
				continue
			}
			cat := deviceCategories[p.Device]
			c.PortDevices = append(c.PortDevices, PortDevice{
				Port:     p.ModbusPort,
				PlaceID:  p.ID,
				DeviceID: p.Device,
				Category: cat,
			})
		}
	}
}

// PortDevice finds the PortDevice entry for the given port, or nil if not found.
func (c *DesignCache) lookupPort(port int) *PortDevice {
	for i := range c.PortDevices {
		if c.PortDevices[i].Port == port {
			return &c.PortDevices[i]
		}
	}
	return nil
}
