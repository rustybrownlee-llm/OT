package dashboard

import (
	"sort"
	"strings"
)

// TopologyData is the template data for the topology visualization page.
//
// PROTOTYPE-DEBT: [td-046] Layout positions are CSS flex/grid, not computed
// coordinates. Limits control over exact device placement.
type TopologyData struct {
	EnvID      string
	EnvName    string
	Archetype  string // "modern-segmented", "legacy-flat", "hybrid"
	EraSpan    string
	Levels     []TopologyLevel
	Boundaries []TopologyBoundary
	Bridges    []TopologyBridge
	AllEnvs    []EnvSummary // for environment selector
}

// TopologyLevel groups networks and devices that share a Purdue level or plane.
type TopologyLevel struct {
	Name     string           // "Level 3 - Site Operations", "Flat (Unclassified)", etc.
	PLevel   int              // for sort ordering: 99=WAN, 3,2,1=Purdue, 0=unclassified, -1=flat
	Networks []TopologyNetwork
	Devices  []TopologyDevice
}

// TopologyNetwork describes one network segment in the topology.
type TopologyNetwork struct {
	ID      string
	Name    string
	Type    string // "ethernet", "serial-rs485"
	Subnet  string
	Managed bool
	SPAN    bool
}

// TopologyDevice describes one placed device in the topology.
//
// PROTOTYPE-DEBT: [td-047] Era badge uses decade-based neutral blue hue scale.
// May not scale to environments spanning more than four decades.
type TopologyDevice struct {
	ID         string
	DeviceType string // atom model, e.g. "SLC-500/05"
	Role       string
	IP         string // empty for serial devices
	Port       int
	Serial     bool
	SerialAddr int
	GatewayID  string
	Installed  *int
	EraClass   string // "era-1990s", "era-2000s", "era-2010s", "era-2020s"
}

// TopologyBoundary describes a segmentation boundary between two level zones.
type TopologyBoundary struct {
	FromLevel      string
	ToLevel        string
	State          string // "enforced", "intended", "absent"
	Infrastructure string
	Installed      *int
	CSSClass       string // "boundary-enforced", "boundary-intended", "boundary-absent"
}

// TopologyBridge describes a gateway device that bridges two networks.
type TopologyBridge struct {
	DeviceID    string
	FromNetwork string
	ToNetwork   string
}

// EnvSummary is used in the environment selector on the topology page.
type EnvSummary struct {
	ID        string
	Name      string
	Archetype string
}

// topologyPageData is the template data for the full topology page.
type topologyPageData struct {
	Title      string
	ActivePage string
	Topology   *TopologyData
}

// BuildTopologyData assembles TopologyData from an environment definition and
// its resolved network atoms from the design library. Returns nil if def is nil.
// Exported for testing.
func BuildTopologyData(def *EnvironmentDef, lib *DesignLibrary) *TopologyData {
	if def == nil {
		return nil
	}

	archetype := def.Env.Archetype
	if archetype == "" {
		archetype = "legacy-flat" // FR-016: default for environments missing archetype
	}

	td := &TopologyData{
		EnvID:     def.Env.ID,
		EnvName:   def.Env.Name,
		Archetype: archetype,
		EraSpan:   def.Env.EraSpan,
		AllEnvs:   buildEnvSummaries(lib),
	}

	// Step 1: Infer level info for each network in the environment.
	levelMap := inferNetworkLevels(def, lib, archetype)

	// Step 2: Build TopologyLevel list from unique levels.
	td.Levels = buildLevels(def, lib, levelMap, archetype)

	// Step 3: Build boundary list from environment boundary definitions.
	td.Boundaries = buildBoundaries(def.Boundaries, levelMap)

	// Step 4: Build bridge list from placement bridge definitions.
	td.Bridges = buildBridges(def.Placements)

	// Step 5: Sort levels (WAN=99 top, then L3, L2, L1, flat=-1 bottom).
	sortLevels(td.Levels)

	return td
}

// inferNetworkLevels returns a map of network ID to level info for all networks
// in the environment.
func inferNetworkLevels(def *EnvironmentDef, lib *DesignLibrary, archetype string) map[string]networkLevelInfo {
	levelMap := make(map[string]networkLevelInfo, len(def.Networks))
	for _, ref := range def.Networks {
		var net *NetworkAtom
		if lib != nil {
			net = lib.Networks[ref.Ref]
		}
		levelMap[ref.Ref] = inferPurdueLevel(ref.Ref, archetype, net)
	}
	return levelMap
}

// buildLevels groups networks and devices by inferred Purdue level.
func buildLevels(def *EnvironmentDef, lib *DesignLibrary, levelMap map[string]networkLevelInfo, archetype string) []TopologyLevel {
	type levelKey struct {
		label  string
		plevel int
	}
	levelIndex := make(map[levelKey]int)
	var levels []TopologyLevel

	getOrAddLevel := func(info networkLevelInfo) int {
		k := levelKey{info.Label, info.PLevel}
		idx, ok := levelIndex[k]
		if !ok {
			idx = len(levels)
			levelIndex[k] = idx
			levels = append(levels, TopologyLevel{Name: info.Label, PLevel: info.PLevel})
		}
		return idx
	}

	// Add networks to levels.
	for _, ref := range def.Networks {
		info := levelMap[ref.Ref]
		idx := getOrAddLevel(info)
		var netAtom *NetworkAtom
		if lib != nil {
			netAtom = lib.Networks[ref.Ref]
		}
		levels[idx].Networks = append(levels[idx].Networks, toTopologyNetwork(ref.Ref, netAtom))
	}

	// Add devices to levels based on their primary network.
	for _, p := range def.Placements {
		info, ok := levelMap[p.Network]
		if !ok {
			info = inferPurdueLevel(p.Network, archetype, nil)
		}
		idx := getOrAddLevel(info)
		dev := toTopologyDevice(p, lib)
		levels[idx].Devices = append(levels[idx].Devices, dev)
	}

	return levels
}

// toTopologyNetwork converts a NetworkAtom (or nil) into a TopologyNetwork.
func toTopologyNetwork(netID string, net *NetworkAtom) TopologyNetwork {
	tn := TopologyNetwork{ID: netID}
	if net != nil {
		tn.Name = net.Network.Name
		tn.Type = net.Network.Type
		tn.Subnet = net.Properties.Subnet
		tn.Managed = net.Properties.ManagedSwitch
		tn.SPAN = net.Properties.SPANCapable
	}
	return tn
}

// toTopologyDevice converts a Placement into a TopologyDevice.
func toTopologyDevice(p Placement, lib *DesignLibrary) TopologyDevice {
	dev := TopologyDevice{
		ID:         p.ID,
		Role:       p.Role,
		IP:         p.IP,
		Port:       p.ModbusPort,
		Serial:     p.ModbusPort == 0 && p.SerialAddress > 0,
		SerialAddr: p.SerialAddress,
		GatewayID:  p.Gateway,
		Installed:  p.Installed,
		EraClass:   EraClass(p.Installed),
	}
	if lib != nil {
		if atom, ok := lib.Devices[p.Device]; ok {
			dev.DeviceType = atom.Device.Model
		}
	}
	if dev.DeviceType == "" {
		dev.DeviceType = p.Device
	}
	return dev
}

// buildBoundaries converts BoundaryDef slice into TopologyBoundary slice.
// The levelMap is used to resolve network IDs to level labels.
func buildBoundaries(defs []BoundaryDef, levelMap map[string]networkLevelInfo) []TopologyBoundary {
	var result []TopologyBoundary
	for _, b := range defs {
		if len(b.Between) < 2 {
			continue
		}
		fromInfo := levelMap[b.Between[0]]
		toInfo := levelMap[b.Between[1]]
		tb := TopologyBoundary{
			FromLevel:      fromInfo.Label,
			ToLevel:        toInfo.Label,
			State:          b.State,
			Infrastructure: b.Infrastructure,
			Installed:      b.Installed,
			CSSClass:       boundaryCSSClass(b.State),
		}
		result = append(result, tb)
	}
	return result
}

// boundaryCSSClass maps a boundary state to its CSS class.
func boundaryCSSClass(state string) string {
	switch state {
	case "enforced":
		return "boundary-enforced"
	case "intended":
		return "boundary-intended"
	case "absent":
		return "boundary-absent"
	default:
		return "boundary-intended"
	}
}

// buildBridges extracts TopologyBridge entries from placements with bridge definitions.
func buildBridges(placements []Placement) []TopologyBridge {
	var bridges []TopologyBridge
	for _, p := range placements {
		for _, b := range p.Bridges {
			bridges = append(bridges, TopologyBridge{
				DeviceID:    p.ID,
				FromNetwork: b.FromNetwork,
				ToNetwork:   b.ToNetwork,
			})
		}
	}
	return bridges
}

// EraClass maps an installation year to a decade CSS class.
// Returns empty string if installed is nil. Exported for testing.
//
// PROTOTYPE-DEBT: [td-047] Decade-based neutral blue hue scale. May not scale
// to environments spanning more than four decades.
func EraClass(installed *int) string {
	if installed == nil {
		return ""
	}
	decade := *installed / 10 * 10
	switch {
	case decade <= 1999:
		return "era-1990s"
	case decade <= 2009:
		return "era-2000s"
	case decade <= 2019:
		return "era-2010s"
	default:
		return "era-2020s"
	}
}

// sortLevels orders levels from WAN (top) through Purdue L3, L2, L1, flat, and
// serial backbone at the bottom. Unclassified appears after flat.
func sortLevels(levels []TopologyLevel) {
	sort.SliceStable(levels, func(i, j int) bool {
		return levelSortKey(levels[i].PLevel) < levelSortKey(levels[j].PLevel)
	})
}

// levelSortKey maps a PLevel integer to a sort order (ascending = top of diagram).
func levelSortKey(plevel int) int {
	switch {
	case plevel == 99: // WAN
		return 0
	case plevel == 3: // Level 3
		return 1
	case plevel == 2: // Level 2
		return 2
	case plevel == 1: // Level 1
		return 3
	case plevel == 0: // Unclassified
		return 4
	case plevel == -1: // Flat
		return 5
	case plevel == -2: // Serial backbone
		return 6
	case plevel == -3: // Link
		return 7
	default:
		return 8
	}
}

// buildEnvSummaries returns an alphabetically-sorted slice of EnvSummary
// for use in the environment selector on the topology page.
func buildEnvSummaries(lib *DesignLibrary) []EnvSummary {
	if lib == nil {
		return nil
	}
	ids := sortedKeys(lib.Environments)
	summaries := make([]EnvSummary, 0, len(ids))
	for _, id := range ids {
		env := lib.Environments[id]
		summaries = append(summaries, EnvSummary{
			ID:        env.Env.ID,
			Name:      env.Env.Name,
			Archetype: env.Env.Archetype,
		})
	}
	return summaries
}

// focalDeviceID returns the ID of the layout focal point for a legacy-flat or hybrid
// environment: the placement with the most network connections (dual-homed or gateway).
// If none qualify, returns the first placement ID.
func focalDeviceID(placements []Placement) string {
	best := ""
	bestScore := -1
	for _, p := range placements {
		score := len(p.AdditionalNetworks) + len(p.Bridges)
		if score > bestScore {
			bestScore = score
			best = p.ID
		}
	}
	return best
}

// archetypeLabel returns the human-readable display label for an archetype string.
func archetypeLabel(archetype string) string {
	switch strings.ToLower(archetype) {
	case "modern-segmented":
		return "Modern Segmented"
	case "legacy-flat":
		return "Legacy Flat"
	case "hybrid":
		return "Hybrid"
	default:
		return archetype
	}
}
