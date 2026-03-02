// Package discovery implements active device discovery and register enumeration.
// Discovery probes configured Modbus TCP endpoints to determine device register
// counts, addressing modes, and response times -- the same information a real
// OT security tool would gather during an initial network survey.
//
// [OT-REVIEW] Discovery uses a 100ms inter-read delay between enumeration reads
// to avoid competing with the plant's simulated SCADA polling for connection slots.
// Devices like the ROC800 support only 4 concurrent TCP connections, and aggressive
// scanning during active SCADA polling is a known cause of production incidents.
package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

const (
	// maxRegisters is the Modbus maximum register address (16-bit field).
	// [OT-REVIEW] Cap at 65535: Modbus register address is a 16-bit field (max 65535).
	maxRegisters = 65535

	// modbusMaxReadQuantity is the Modbus spec maximum registers per read request.
	modbusMaxReadQuantity = 125

	// connectTimeout is the per-endpoint TCP connect timeout.
	connectTimeout = 2 * time.Second

	// responseSamples is the number of RTT samples used to compute mean and jitter.
	responseSamples = 3
)

// discoveryReadDelay is the inter-read pause during enumeration.
// [OT-REVIEW] 100ms avoids competing with SCADA polling for connection slots.
// Devices like the ROC800 support only 4 concurrent TCP connections; aggressive
// scanning during active SCADA polling is a known cause of production incidents.
// This is a package-level variable so tests can set it to zero for speed.
var discoveryReadDelay = 100 * time.Millisecond

// ClientInterface abstracts the modbus.ModbusClient for testing.
// Only the methods used by discovery are included per interface minimality.
type ClientInterface interface {
	Open() error
	Close() error
	SetUnitId(id uint8) error
	ReadRegisters(addr uint16, quantity uint16, regType modbus.RegType) ([]uint16, error)
	ReadCoils(addr uint16, quantity uint16) ([]bool, error)
}

// clientFactory creates a Modbus TCP client for the given URL.
// Replaced in tests with a mock factory.
type clientFactory func(url string) (ClientInterface, error)

// defaultClientFactory creates a real simonvetter/modbus client.
func defaultClientFactory(url string) (ClientInterface, error) {
	return modbus.NewClient(&modbus.ClientConfiguration{
		URL:     url,
		Timeout: connectTimeout,
	})
}

// Discoverer probes configured Modbus endpoints to build the asset inventory.
type Discoverer struct {
	cfg          *config.Config
	inv          *inventory.Inventory
	newClient    clientFactory
	gatewayDelay time.Duration
}

// New creates a Discoverer using the production Modbus client factory.
func New(cfg *config.Config, inv *inventory.Inventory) *Discoverer {
	return &Discoverer{
		cfg:          cfg,
		inv:          inv,
		newClient:    defaultClientFactory,
		gatewayDelay: time.Duration(cfg.GatewayRequestDelayMs) * time.Millisecond,
	}
}

// NewWithFactory creates a Discoverer with a custom client factory.
// This constructor is exported for use in unit tests; production code uses New().
func NewWithFactory(cfg *config.Config, inv *inventory.Inventory, f func(url string) (ClientInterface, error)) *Discoverer {
	return &Discoverer{
		cfg:          cfg,
		inv:          inv,
		newClient:    f,
		gatewayDelay: time.Duration(cfg.GatewayRequestDelayMs) * time.Millisecond,
	}
}

// DiscoverAll probes every configured endpoint and populates the inventory.
// Endpoints that are offline are recorded with status "offline"; they do not
// cause an error return. Only configuration or context errors are returned.
//
// PROTOTYPE-DEBT: [td-discovery-026] Register enumeration runs once at startup.
// If the plant binary changes register configuration, the cached count becomes stale.
// Future: periodic re-enumeration or trigger on ErrIllegalDataAddress during polling.
func (d *Discoverer) DiscoverAll(ctx context.Context) error {
	for _, env := range d.cfg.Environments {
		for _, ep := range env.Endpoints {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if ep.Gateway {
				if err := d.discoverGateway(ctx, env, ep); err != nil {
					slog.Error("gateway discovery failed",
						"endpoint", endpointAddr(env.Address, ep.Port),
						"error", err)
				}
			} else {
				if err := d.discoverEndpoint(ctx, env, ep); err != nil {
					slog.Error("endpoint discovery failed",
						"endpoint", endpointAddr(env.Address, ep.Port),
						"error", err)
				}
			}
		}
	}
	return nil
}

// discoverEndpoint connects to a single non-gateway endpoint, enumerates registers,
// measures response times, and adds the device to the inventory.
func (d *Discoverer) discoverEndpoint(ctx context.Context, env config.Environment, ep config.Endpoint) error {
	addr := endpointAddr(env.Address, ep.Port)
	client, err := d.newClient("tcp://" + addr)
	if err != nil {
		d.recordOffline(addr, ep.UnitID, env, ep)
		return nil
	}

	if err := client.Open(); err != nil {
		d.recordOffline(addr, ep.UnitID, env, ep)
		return nil
	}
	defer client.Close()

	if err := client.SetUnitId(ep.UnitID); err != nil {
		d.recordOffline(addr, ep.UnitID, env, ep)
		return nil
	}

	return d.probeAndRecord(ctx, client, addr, ep.UnitID, "", env, ep)
}

// discoverGateway scans the configured unit IDs on a gateway port. Each responding
// unit ID becomes a separate asset in the inventory, linked to a gateway asset.
//
// [OT-REVIEW] Unit ID 247 is a simulator convention for gateway diagnostics.
// Real Moxa NPort 5150 devices are managed via HTTP (port 80) and SNMP (port 161),
// not Modbus. This is an educational abstraction.
func (d *Discoverer) discoverGateway(ctx context.Context, env config.Environment, ep config.Endpoint) error {
	addr := endpointAddr(env.Address, ep.Port)

	client, err := d.newClient("tcp://" + addr)
	if err != nil {
		slog.Warn("gateway unreachable", "addr", addr, "error", err)
		return nil
	}
	if err := client.Open(); err != nil {
		slog.Warn("gateway open failed", "addr", addr, "error", err)
		return nil
	}
	defer client.Close()

	var gatewayAssetID string

	for _, unitID := range ep.ScanUnitIDs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		gatewayAssetID = d.scanGatewayUnit(ctx, client, addr, unitID, gatewayAssetID, env, ep)
	}

	return nil
}

// scanGatewayUnit attempts to probe a single unit ID on a gateway port.
// Returns the updated gatewayAssetID (set when unit 247 responds).
func (d *Discoverer) scanGatewayUnit(
	ctx context.Context,
	client ClientInterface,
	addr string,
	unitID uint8,
	gatewayAssetID string,
	env config.Environment,
	ep config.Endpoint,
) string {
	if err := client.SetUnitId(unitID); err != nil {
		slog.Debug("gateway: SetUnitId failed", "addr", addr, "unit", unitID, "error", err)
		return gatewayAssetID
	}

	// [OT-REVIEW] Gateway request delay: RS-485 inter-frame gap.
	// Min 4ms at 9600 baud; 10ms provides margin for jitter.
	time.Sleep(d.gatewayDelay)

	// Probe register 0 to test responsiveness.
	_, err := client.ReadRegisters(0, 1, modbus.HOLDING_REGISTER)
	if err != nil && !isDataAddressError(err) {
		slog.Debug("gateway: unit not responding", "addr", addr, "unit", unitID, "error", err)
		return gatewayAssetID
	}

	if probeErr := d.probeAndRecord(ctx, client, addr, unitID, gatewayAssetID, env, ep); probeErr != nil {
		slog.Warn("gateway: probe failed", "addr", addr, "unit", unitID, "error", probeErr)
		return gatewayAssetID
	}

	if gatewayAssetID == "" && unitID == 247 {
		return assetID(addr, 247)
	}
	return gatewayAssetID
}

// probeAndRecord performs register enumeration, response time measurement, and
// inventory upsert for a single device/unit combination.
func (d *Discoverer) probeAndRecord(
	ctx context.Context,
	client ClientInterface,
	addr string,
	unitID uint8,
	viaGateway string,
	env config.Environment,
	ep config.Endpoint,
) error {
	now := time.Now()

	holdCount, addressing, err := enumerateHoldingRegisters(client)
	if err != nil {
		return fmt.Errorf("enumerate holding registers: %w", err)
	}

	if err := sleepCtx(ctx, discoveryReadDelay); err != nil {
		return err
	}

	coilCount, err := enumerateCoils(client)
	if err != nil {
		slog.Debug("coil enumeration failed", "addr", addr, "unit", unitID, "error", err)
		coilCount = 0
	}

	baseAddr := baseAddress(addressing)
	meanMs, jitterMs, err := measureResponseTime(client, baseAddr, holdCount)
	if err != nil {
		slog.Debug("response time measurement failed", "addr", addr, "unit", unitID, "error", err)
		meanMs, jitterMs = 0, 0
	}

	a := buildAsset(addr, unitID, addressing, holdCount, coilCount, meanMs, jitterMs,
		now, viaGateway, env, ep)
	d.inv.Upsert(a)

	slog.Info("device discovered",
		"id", a.ID,
		"holding_regs", holdCount,
		"coils", coilCount,
		"addressing", addressing,
		"response_time_ms", meanMs)
	return nil
}

// buildAsset constructs an Asset struct for a discovered device.
func buildAsset(
	addr string, unitID uint8, addressing string,
	holdCount, coilCount int, meanMs, jitterMs float64,
	now time.Time, viaGateway string,
	env config.Environment, ep config.Endpoint,
) *inventory.Asset {
	return &inventory.Asset{
		ID:               assetID(addr, unitID),
		Endpoint:         addr,
		UnitID:           unitID,
		Addressing:       addressing,
		HoldingRegCount:  holdCount,
		CoilCount:        coilCount,
		ResponseTimeMs:   meanMs,
		ResponseJitterMs: jitterMs,
		Status:           inventory.StatusOnline,
		FirstSeen:        now,
		LastSeen:         now,
		ViaGateway:       viaGateway,
		EnvironmentName:  env.Name,
		ConfigDesc:       ep.Description,
		Protocol:         "modbus-tcp",
		Category:         categoryFromDescription(ep.Description),
		PurdueLevel:      purdueLevel(env.Name),
	}
}

// recordOffline adds an offline asset to the inventory for an unreachable endpoint.
func (d *Discoverer) recordOffline(addr string, unitID uint8, env config.Environment, ep config.Endpoint) {
	now := time.Now()
	id := assetID(addr, unitID)

	if _, exists := d.inv.Get(id); exists {
		d.inv.SetStatus(id, inventory.StatusOffline, now)
		return
	}

	d.inv.Upsert(&inventory.Asset{
		ID:              id,
		Endpoint:        addr,
		UnitID:          unitID,
		Status:          inventory.StatusOffline,
		FirstSeen:       now,
		LastSeen:        now,
		EnvironmentName: env.Name,
		ConfigDesc:      ep.Description,
		Protocol:        "modbus-tcp",
		Category:        categoryFromDescription(ep.Description),
		PurdueLevel:     purdueLevel(env.Name),
	})
	slog.Warn("endpoint offline", "id", id)
}

// endpointAddr formats the host:port string for a configured endpoint.
func endpointAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

// assetID returns the deterministic asset identifier for a device/unit.
func assetID(endpoint string, unitID uint8) string {
	return fmt.Sprintf("%s:%d", endpoint, unitID)
}

// baseAddress returns the first register address for the given addressing mode.
func baseAddress(addressing string) uint16 {
	if addressing == "one-based" {
		return 1
	}
	return 0
}

// isDataAddressError returns true if the error is a Modbus illegal data address exception.
func isDataAddressError(err error) bool {
	return err == modbus.ErrIllegalDataAddress
}

// sleepCtx sleeps for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// categoryFromDescription extracts a device category from the config description string.
func categoryFromDescription(desc string) string {
	keywords := map[string]string{
		"plc":     "plc",
		"rtu":     "rtu",
		"gateway": "gateway",
		"moxa":    "gateway",
		"hmi":     "hmi",
	}
	lowerDesc := asciiToLower(desc)
	for k, cat := range keywords {
		if contains(lowerDesc, k) {
			return cat
		}
	}
	return "unknown"
}

// asciiToLower converts ASCII characters to lowercase.
func asciiToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains reports whether substr is present in s.
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// purdueLevel derives a Purdue model level label from the environment name.
func purdueLevel(envName string) string {
	switch envName {
	case "water-mfg":
		return "level-3"
	case "pipeline":
		return "station-lan"
	default:
		return "unknown"
	}
}
