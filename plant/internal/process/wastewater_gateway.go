package process

import (
	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// NewWastewaterGatewayModel creates a GatewayModel for the brownfield wastewater serial gateway.
// Moxa NPort 5150 with ww-serial-gateway variant.
//
// TX/RX: 2 per tick (two SLC-500 PLCs behind the gateway, unit IDs 1 and 2).
// Error rate: 0.3%/tick -- slightly elevated over mfg-gateway (0.2%) because the RS-485
// bus serves 1997-vintage ProSoft MVI46-MCM modules and aging field wiring from 2008.
// Serial mode is RS-485-2wire (enum value 2) -- half-duplex multidrop required for shared bus.
//
// [OT-REVIEW] 2 tx/tick models realistic polling of two downstream SLC-500 PLCs.
// [OT-REVIEW] Elevated error rate vs mfg-gateway reflects aging wiring and ProSoft RTT overhead.
//
// Reuses the existing GatewayModel implementation with wastewater-specific parameters.
// No changes to GatewayModel internals -- demonstrates model reuse per FR-012.
//
// PROTOTYPE-DEBT: [td-gateway-020] Counter rates are hardcoded.
// Wastewater gateway shares the same future resolution path as mfg-gateway.
// TODO-FUTURE: Derive from environment placement topology (count serial devices behind each gateway).
//
// Implemented in SOW-018.0.
func NewWastewaterGatewayModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *GatewayModel {
	// [OT-REVIEW] RS-485-2wire (serial_mode enum 2) is fixed for this variant.
	// TX/RX of 2 models two SLC-500 PLCs at unit IDs 1 and 2 sharing the serial bus.
	// Error rate 0.003 (0.3%) is slightly higher than mfg-gateway: aging RS-485 wiring
	// from 2008 and ProSoft MVI46-MCM modules create more framing errors than a
	// newer installation.
	return newGatewayModelParams(store, profile, "ww-gateway", 2, 2, 0.003)
}
