package dashboard

import (
	"fmt"
	"net/http"
	"sort"
)

// overviewData is the template data for the overview page.
type overviewData struct {
	Title                string
	ActivePage           string
	DevicesOnline        int
	DevicesOffline       int
	AlertsTotal          int
	AlertsCritical       int
	BaselinesEstablished int
	BaselinesLearning    int
	DiscoveryInProgress  bool
}

func (d *Dashboard) buildOverviewData() overviewData {
	data := overviewData{Title: "Overview", ActivePage: "overview"}
	health, err := d.api.GetHealth()
	if err == nil {
		data.DevicesOnline = health.DevicesOnline
		data.DevicesOffline = health.DevicesOffline
		data.DiscoveryInProgress = (health.DevicesOnline+health.DevicesOffline == 0)
	} else {
		data.DiscoveryInProgress = true
	}
	alerts, err := d.api.GetAlerts(AlertFilters{})
	if err == nil {
		data.AlertsTotal = len(alerts)
		for _, a := range alerts {
			if a.Severity == "critical" {
				data.AlertsCritical++
			}
		}
	}
	baselines, err := d.api.GetBaselines()
	if err == nil {
		for _, b := range baselines {
			switch b.Status {
			case "established":
				data.BaselinesEstablished++
			case "learning":
				data.BaselinesLearning++
			}
		}
	}
	return data
}

// assetRow is an asset enriched with display-layer fields.
type assetRow struct {
	Asset
	ConnectionType string
	DeviceAtomID   string
}

// assetGroup groups asset rows by environment name.
type assetGroup struct {
	Name   string
	Assets []assetRow
}

// assetsPageData is the template data for the asset inventory page.
type assetsPageData struct {
	Title               string
	ActivePage          string
	Groups              []assetGroup
	DiscoveryInProgress bool
}

func (d *Dashboard) buildAssetsData() assetsPageData {
	data := assetsPageData{Title: "Assets", ActivePage: "assets"}
	assets, err := d.api.GetAssets()
	if err != nil || len(assets) == 0 {
		data.DiscoveryInProgress = true
		return data
	}
	groupMap := make(map[string][]assetRow)
	for _, a := range assets {
		row := assetRow{
			Asset:          a,
			ConnectionType: connectionType(a),
			DeviceAtomID:   d.lib.ResolveDeviceAtomID(a.ConfigDesc),
		}
		groupMap[a.EnvironmentName] = append(groupMap[a.EnvironmentName], row)
	}
	envNames := sortedKeys(groupMap)
	for _, name := range envNames {
		rows := groupMap[name]
		sort.Slice(rows, func(i, j int) bool { return rows[i].Endpoint < rows[j].Endpoint })
		data.Groups = append(data.Groups, assetGroup{Name: name, Assets: rows})
	}
	return data
}

// connectionType derives the connection type label for an asset.
func connectionType(a Asset) string {
	if a.ViaGateway != "" {
		return "Serial via Gateway"
	}
	return "Direct Ethernet"
}

// RegisterInfo holds design-layer enrichment for one register address.
type RegisterInfo struct {
	Name        string
	Unit        string
	ScaledValue string
	Writable    bool
	Description string
}

// assetDetailData is the template data for the device detail page.
type assetDetailData struct {
	Title        string
	ActivePage   string
	Asset        Asset
	DeviceAtomID string
	Registers    *RegisterResponse
	Enriched     bool
	RegisterInfo []*RegisterInfo
	CoilInfo     []*RegisterInfo
}

func (d *Dashboard) buildAssetDetailData(id string) *assetDetailData {
	assets, err := d.api.GetAssets()
	if err != nil {
		return nil
	}
	var found *Asset
	for i := range assets {
		if assets[i].ID == id {
			found = &assets[i]
			break
		}
	}
	if found == nil {
		return nil
	}
	data := &assetDetailData{
		Title:        found.ConfigDesc,
		ActivePage:   "assets",
		Asset:        *found,
		DeviceAtomID: d.lib.ResolveDeviceAtomID(found.ConfigDesc),
	}
	data.Registers, _ = d.api.GetAssetRegisters(id)
	d.enrichRegisters(data)
	return data
}

func (d *Dashboard) buildRegisterData(id string) *assetDetailData {
	assets, err := d.api.GetAssets()
	if err != nil {
		return &assetDetailData{}
	}
	var found *Asset
	for i := range assets {
		if assets[i].ID == id {
			found = &assets[i]
			break
		}
	}
	if found == nil {
		return &assetDetailData{}
	}
	data := &assetDetailData{
		Asset:        *found,
		DeviceAtomID: d.lib.ResolveDeviceAtomID(found.ConfigDesc),
	}
	data.Registers, _ = d.api.GetAssetRegisters(id)
	d.enrichRegisters(data)
	return data
}

func (d *Dashboard) enrichRegisters(data *assetDetailData) {
	if data.Registers == nil || data.DeviceAtomID == "" {
		return
	}
	atom, ok := d.lib.Devices[data.DeviceAtomID]
	if !ok {
		return
	}
	variantName := d.lib.ResolveVariantForPlacement(data.Asset.ConfigDesc)
	variant, ok := atom.Variants[variantName]
	if !ok {
		return
	}
	data.Enriched = true

	holdingByAddr := make(map[int]*RegisterEntry, len(variant.Holding))
	for i := range variant.Holding {
		holdingByAddr[variant.Holding[i].Address] = &variant.Holding[i]
	}
	data.RegisterInfo = make([]*RegisterInfo, len(data.Registers.HoldingRegisters))
	for i, raw := range data.Registers.HoldingRegisters {
		re := holdingByAddr[i]
		if re == nil {
			continue
		}
		data.RegisterInfo[i] = &RegisterInfo{
			Name:        re.Name,
			Unit:        re.Unit,
			ScaledValue: FormatScaled(raw, re.ScaleMin, re.ScaleMax, re.Unit),
			Writable:    re.Writable,
			Description: re.Description,
		}
	}

	coilByAddr := make(map[int]*CoilEntry, len(variant.Coils))
	for i := range variant.Coils {
		coilByAddr[variant.Coils[i].Address] = &variant.Coils[i]
	}
	data.CoilInfo = make([]*RegisterInfo, len(data.Registers.Coils))
	for i := range data.Registers.Coils {
		ce := coilByAddr[i]
		if ce == nil {
			continue
		}
		data.CoilInfo[i] = &RegisterInfo{
			Name:        ce.Name,
			Writable:    ce.Writable,
			Description: ce.Description,
		}
	}
}

// alertsPageData is the template data for the alerts page.
type alertsPageData struct {
	Title             string
	ActivePage        string
	Alerts            []Alert
	AlertsUnavailable bool
	FilterSeverity    string
	FilterDevice      string
	FilterRule        string
	DeviceOptions     []string
	RuleOptions       []string
}

func (d *Dashboard) buildAlertsData(r *http.Request) alertsPageData {
	q := r.URL.Query()
	data := alertsPageData{
		Title:          "Alerts",
		ActivePage:     "alerts",
		FilterSeverity: q.Get("severity"),
		FilterDevice:   q.Get("device"),
		FilterRule:     q.Get("rule"),
	}
	alerts, err := d.api.GetAlerts(AlertFilters{
		Severity: data.FilterSeverity,
		Device:   data.FilterDevice,
		Rule:     data.FilterRule,
	})
	if err != nil {
		data.AlertsUnavailable = true
		return data
	}
	data.Alerts = alerts
	ruleSet := make(map[string]bool)
	for _, a := range alerts {
		ruleSet[a.Rule] = true
	}
	assets, _ := d.api.GetAssets()
	deviceSet := make(map[string]bool)
	for _, a := range assets {
		deviceSet[a.ID] = true
	}
	data.RuleOptions = sortedKeys(ruleSet)
	data.DeviceOptions = sortedKeys(deviceSet)
	return data
}

// deviceLibEntry wraps a DeviceAtom with a pre-computed variant count.
type deviceLibEntry struct {
	*DeviceAtom
	VariantCount int
}

// deviceGroup groups device library entries by category.
type deviceGroup struct {
	Category string
	Devices  []*deviceLibEntry
}

// designDevicesData is the template data for the device library page.
type designDevicesData struct {
	Title              string
	ActivePage         string
	Groups             []deviceGroup
	LibraryUnavailable bool
}

func (d *Dashboard) buildDesignDevicesData() designDevicesData {
	data := designDevicesData{Title: "Device Library", ActivePage: "design-devices"}
	if d.lib.IsEmpty() {
		data.LibraryUnavailable = true
		return data
	}
	groupMap := make(map[string][]*deviceLibEntry)
	for _, atom := range d.lib.Devices {
		entry := &deviceLibEntry{DeviceAtom: atom, VariantCount: len(atom.Variants)}
		groupMap[atom.Device.Category] = append(groupMap[atom.Device.Category], entry)
	}
	cats := sortedKeys(groupMap)
	categoryOrder := map[string]int{"plc": 0, "rtu": 1, "flow-computer": 2, "gateway": 3}
	sort.Slice(cats, func(i, j int) bool {
		oi, oj := categoryOrder[cats[i]], categoryOrder[cats[j]]
		if oi != oj {
			return oi < oj
		}
		return cats[i] < cats[j]
	})
	for _, cat := range cats {
		entries := groupMap[cat]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Device.ID < entries[j].Device.ID })
		data.Groups = append(data.Groups, deviceGroup{Category: cat, Devices: entries})
	}
	return data
}

// networkLibEntry wraps a NetworkAtom for the template.
type networkLibEntry struct {
	*NetworkAtom
}

// designNetworksData is the template data for the network library page.
type designNetworksData struct {
	Title              string
	ActivePage         string
	Networks           []*networkLibEntry
	LibraryUnavailable bool
}

func (d *Dashboard) buildDesignNetworksData() designNetworksData {
	data := designNetworksData{Title: "Network Library", ActivePage: "design-networks"}
	if d.lib.IsEmpty() {
		data.LibraryUnavailable = true
		return data
	}
	for _, id := range sortedKeys(d.lib.Networks) {
		data.Networks = append(data.Networks, &networkLibEntry{d.lib.Networks[id]})
	}
	return data
}

// envLibEntry wraps an EnvironmentDef with pre-computed counts.
type envLibEntry struct {
	*EnvironmentDef
	Env            EnvironmentMeta
	PlacementCount int
	NetworkCount   int
}

// designEnvsData is the template data for the environment library page.
type designEnvsData struct {
	Title              string
	ActivePage         string
	Environments       []*envLibEntry
	LibraryUnavailable bool
}

func (d *Dashboard) buildDesignEnvsData() designEnvsData {
	data := designEnvsData{Title: "Environment Library", ActivePage: "design-envs"}
	if d.lib.IsEmpty() {
		data.LibraryUnavailable = true
		return data
	}
	for _, id := range sortedKeys(d.lib.Environments) {
		def := d.lib.Environments[id]
		data.Environments = append(data.Environments, &envLibEntry{
			EnvironmentDef: def,
			Env:            def.Env,
			PlacementCount: len(def.Placements),
			NetworkCount:   len(def.Networks),
		})
	}
	return data
}

// portMapRow represents one row in the environment port map summary.
type portMapRow struct {
	PlacementID    string
	HostPort       string
	UnitIDs        string
	ConnectionType string
}

// buildPortMap derives the Modbus TCP port map for an environment.
// Serial-only devices (no modbus_port) appear under their gateway's entry.
func buildPortMap(def *EnvironmentDef) []portMapRow {
	serialByGW := make(map[string][]Placement)
	for _, p := range def.Placements {
		if p.ModbusPort == 0 && p.Gateway != "" {
			serialByGW[p.Gateway] = append(serialByGW[p.Gateway], p)
		}
	}
	var rows []portMapRow
	for _, p := range def.Placements {
		if p.ModbusPort == 0 {
			continue
		}
		unitIDs := "1"
		connType := "Direct"
		if serials, ok := serialByGW[p.ID]; ok {
			ids := make([]string, 0, len(serials))
			for _, s := range serials {
				ids = append(ids, fmt.Sprintf("%d", s.SerialAddress))
			}
			sort.Strings(ids)
			unitIDs = fmt.Sprintf("gateway; serial unit IDs: %s", joinStrings(ids))
			connType = "Gateway (serial)"
		}
		rows = append(rows, portMapRow{
			PlacementID:    p.ID,
			HostPort:       fmt.Sprintf("%s:%d", p.IP, p.ModbusPort),
			UnitIDs:        unitIDs,
			ConnectionType: connType,
		})
	}
	return rows
}

// liveAssetsForDevice returns all live assets whose device atom ID matches deviceID.
func (d *Dashboard) liveAssetsForDevice(deviceID string) []Asset {
	assets, err := d.api.GetAssets()
	if err != nil {
		return nil
	}
	var result []Asset
	for _, a := range assets {
		if d.lib.ResolveDeviceAtomID(a.ConfigDesc) == deviceID {
			result = append(result, a)
		}
	}
	return result
}

// sortedKeys returns the keys of a map[string]T sorted alphabetically.
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinStrings joins a slice of strings with ", ".
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
