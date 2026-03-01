package modbus

import (
	"fmt"
	"strings"
	"time"

	mblib "github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

// Serial frame time constants derived from 9600 baud 8N1 framing.
// Each character is 10 bits (1 start + 8 data + 1 stop) = 1.042ms.
// Request frame: 8 chars + 3.5 char inter-frame gap = ~12ms.
// Response frame: 19 chars + 3.5 char inter-frame gap = ~24ms.
// These model real serial-to-Ethernet gateway latency components.
// See SOW-002.0 Section 5, Additive Delay Formula.
const (
	serialRequestFrameMS  = 12 * time.Millisecond
	serialResponseFrameMS = 24 * time.Millisecond
)

// PlacementStore pairs a RegisterStore with its placement metadata for engine initialization.
// The simulation engine uses this to create the correct ProcessModel for each placement.
type PlacementStore struct {
	PlacementID        string
	RegisterMapVariant string
	UnitID             uint8 // 0 for direct Ethernet PLCs; gateway unit ID for serial devices
	Store              *RegisterStore
}

// ServerManager creates and manages Modbus TCP listeners for all Ethernet placements.
//
// PROTOTYPE-DEBT: [td-server-009] Servers bind to 0.0.0.0, not placement-specific IPs.
// Multi-homed devices (wt-plc-03 on two networks) are reachable on any interface.
// TODO-FUTURE: Bind to specific IPs when network topology enforcement is implemented.
//
// PROTOTYPE-DEBT: [td-server-010] Gateway listener does not enforce the NPort 5150's
// 4-connection limit or serial bus serialization. All connections are served concurrently.
// TODO-FUTURE: Enhance gateway simulation fidelity with connection limiting.
type ServerManager struct {
	servers        []*mblib.ModbusServer
	placementStores []PlacementStore
	logger          logging.Logger
}

// NewServerManager builds Modbus TCP servers from the resolved environment.
// It iterates placements, groups serial devices under their gateway, builds
// DeviceProfiles, RegisterStores, and Handlers, and creates one ModbusServer
// per Ethernet placement with a modbus_port defined.
//
// Returns an error if any placement references a variant not found in its device atom,
// which causes fail-loud startup per FR-2.
func NewServerManager(resolved *config.ResolvedEnvironment, logger logging.Logger) (*ServerManager, error) {
	sm := &ServerManager{logger: logger}

	// Build a map of gateway placement ID -> list of serial placements.
	serialByGateway := groupSerialPlacements(resolved.Env.Placements)

	for i := range resolved.Env.Placements {
		p := resolved.Env.Placements[i]
		if p.ModbusPort == 0 {
			// Serial placement -- handled when its gateway is processed.
			continue
		}

		dev := resolved.Devices[p.Device]
		srv, err := sm.buildServer(p, dev, serialByGateway, resolved)
		if err != nil {
			return nil, fmt.Errorf("placement %q: %w", p.ID, err)
		}
		sm.servers = append(sm.servers, srv)
	}

	return sm, nil
}

// Start starts all Modbus TCP listeners in order.
// Logs unit ID bindings for gateway placements.
// Returns an error if any listener fails to bind.
func (sm *ServerManager) Start() error {
	for _, srv := range sm.servers {
		if err := srv.Start(); err != nil {
			return fmt.Errorf("modbus server start failed: %w", err)
		}
	}
	return nil
}

// Stop gracefully shuts down all Modbus TCP listeners.
func (sm *ServerManager) Stop() {
	for _, srv := range sm.servers {
		srv.Stop()
	}
	sm.logger.Info("modbus servers stopped")
}

// Stores returns the slice of PlacementStore entries accumulated during server construction.
// Each entry contains the placement ID, register map variant, unit ID, and RegisterStore
// pointer. The simulation engine uses this to create a ProcessModel for each placement.
func (sm *ServerManager) Stores() []PlacementStore {
	return sm.placementStores
}

// buildServer creates a ModbusServer for a single Ethernet placement.
// For gateway placements, it also processes all attached serial placements.
func (sm *ServerManager) buildServer(
	p config.Placement,
	dev *config.Device,
	serialByGateway map[string][]config.Placement,
	resolved *config.ResolvedEnvironment,
) (*mblib.ModbusServer, error) {
	isGateway := dev.Device.Category == "gateway"

	var h *Handler
	var err error

	if isGateway {
		h, err = sm.buildGatewayHandler(p, dev, serialByGateway, resolved)
	} else {
		h, err = sm.buildDirectHandler(p, dev)
	}
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("tcp://0.0.0.0:%d", p.ModbusPort)
	srv, err := mblib.NewServer(&mblib.ServerConfiguration{
		URL:        url,
		MaxClients: uint(dev.Connectivity.ConcurrentConns),
	}, h)
	if err != nil {
		return nil, fmt.Errorf("create modbus server on %s: %w", url, err)
	}

	sm.logger.Info("modbus server created",
		"placement", p.ID,
		"port", p.ModbusPort,
		"device", dev.Device.ID,
		"role", p.Role,
		"gateway", isGateway,
	)

	return srv, nil
}

// buildDirectHandler creates a Handler for a non-gateway Ethernet PLC.
// The store is registered at sentinel key 0; the handler responds to any unit ID (FR-4).
// Records the store in sm.placementStores for engine initialization.
func (sm *ServerManager) buildDirectHandler(p config.Placement, dev *config.Device) (*Handler, error) {
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		return nil, err
	}

	store := NewRegisterStore(profile)

	sm.placementStores = append(sm.placementStores, PlacementStore{
		PlacementID:        p.ID,
		RegisterMapVariant: p.RegisterMapVariant,
		UnitID:             0,
		Store:              store,
	})

	stores := map[uint8]*RegisterStore{0: store}
	delays := map[uint8]time.Duration{0: profile.ResponseDelay}
	jitters := map[uint8]time.Duration{0: profile.ResponseJitter}

	sm.logger.Info("modbus direct handler configured",
		"placement", p.ID,
		"port", p.ModbusPort,
		"variant", p.RegisterMapVariant,
		"addressing", profile.Addressing,
		"delay_ms", profile.ResponseDelay.Milliseconds(),
	)

	return NewHandler(stores, delays, jitters, false, sm.logger), nil
}

// buildGatewayHandler creates a Handler for a gateway placement that routes
// by unit ID to multiple downstream serial devices.
//
// Unit ID 247 maps to the gateway's own registers.
// Each serial device's serial_address maps to that device's registers with additive delay.
//
// [OT-REVIEW] Unit ID 247 (0xF7) is the highest valid Modbus slave address and is a
// recognized convention for gateway management addresses on devices like Digi TransPort,
// Lantronix, and ProSoft modules. Unit ID 0 is the Modbus broadcast address and must
// never be used as a device address (Modbus Application Protocol V1.1b3, Section 4.1).
func (sm *ServerManager) buildGatewayHandler(
	p config.Placement,
	dev *config.Device,
	serialByGateway map[string][]config.Placement,
	resolved *config.ResolvedEnvironment,
) (*Handler, error) {
	const gatewayUnitID uint8 = 247

	stores := make(map[uint8]*RegisterStore)
	delays := make(map[uint8]time.Duration)
	jitters := make(map[uint8]time.Duration)

	// Gateway own registers at unit ID 247.
	gwProfile, err := device.BuildProfile(p, dev)
	if err != nil {
		return nil, err
	}
	gwStore := NewRegisterStore(gwProfile)
	stores[gatewayUnitID] = gwStore
	delays[gatewayUnitID] = gwProfile.ResponseDelay
	jitters[gatewayUnitID] = gwProfile.ResponseJitter

	sm.placementStores = append(sm.placementStores, PlacementStore{
		PlacementID:        p.ID,
		RegisterMapVariant: p.RegisterMapVariant,
		UnitID:             gatewayUnitID,
		Store:              gwStore,
	})

	// Process serial placements attached to this gateway.
	serials := serialByGateway[p.ID]
	bindingParts := []string{fmt.Sprintf("unit 247 = gateway status (simulated)")}

	for _, sp := range serials {
		unitID := uint8(sp.SerialAddress)
		serialDev := resolved.Devices[sp.Device]

		serialProfile, buildErr := device.BuildProfile(sp, serialDev)
		if buildErr != nil {
			return nil, fmt.Errorf("serial placement %q: %w", sp.ID, buildErr)
		}

		serialStore := NewRegisterStore(serialProfile)
		stores[unitID] = serialStore

		sm.placementStores = append(sm.placementStores, PlacementStore{
			PlacementID:        sp.ID,
			RegisterMapVariant: sp.RegisterMapVariant,
			UnitID:             unitID,
			Store:              serialStore,
		})

		// Additive delay: gateway processing + request frame TX + slave turnaround + response frame TX.
		totalDelay := gwProfile.ResponseDelay +
			serialRequestFrameMS +
			serialProfile.ResponseDelay +
			serialResponseFrameMS
		delays[unitID] = totalDelay

		// Combined jitter: sum of gateway and serial device jitter ranges.
		jitters[unitID] = gwProfile.ResponseJitter + serialProfile.ResponseJitter

		bindingParts = append(bindingParts,
			fmt.Sprintf("unit %d = %s (%s)",
				unitID, serialDev.Device.Model, sp.Role),
		)

		sm.logger.Info("gateway serial device registered",
			"gateway_placement", p.ID,
			"serial_placement", sp.ID,
			"unit_id", unitID,
			"device", serialDev.Device.ID,
			"total_delay_ms", totalDelay.Milliseconds(),
		)
	}

	bindingSummary := strings.Join(bindingParts, ", ")
	sm.logger.Info("modbus gateway handler configured",
		"placement", p.ID,
		"port", p.ModbusPort,
		"unit_bindings", bindingSummary,
	)

	return NewHandler(stores, delays, jitters, true, sm.logger), nil
}

// groupSerialPlacements builds a map from gateway placement ID to all serial placements
// that reference it. Serial placements are identified by having a non-empty Gateway field.
func groupSerialPlacements(placements []config.Placement) map[string][]config.Placement {
	result := make(map[string][]config.Placement)
	for _, p := range placements {
		if p.Gateway != "" {
			result[p.Gateway] = append(result[p.Gateway], p)
		}
	}
	return result
}
