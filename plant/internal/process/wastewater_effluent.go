package process

import (
	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// EffluentModel simulates the SLC-500 ww-plc-02 secondary clarification and effluent discharge process.
// SLC-500/05 with ww-effluent variant (one-based addressing via ProSoft MVI46-MCM Modbus module).
//
// Simulates secondary clarification, RAS/WAS pumping, UV disinfection, effluent discharge,
// and permit interlock bitmask computation.
//
// ATTACK SURFACES (educational):
//   - was_pump coil (addr 2): uncontrolled wasting destroys activated sludge inventory over 24 hours
//   - effluent_discharge_valve coil (addr 4): requires permit_interlock_status all bits set first
//   - Multi-step attack: disable UV (bit0 clears) then open discharge valve bypasses permit interlock
//
// PROTOTYPE-DEBT: [td-effluent-041] No cross-model coupling from influent flow or aeration DO.
// Effluent flow and ammonia simulate independently. TD-041 per SOW-018.0.
// TODO-FUTURE: Shared process state bus (Beta 0.5+).
//
// Implemented in SOW-018.0.

// effluent holding register addresses (one-based, SLC-500 ww-effluent variant).
const (
	effRegFlowRate     = 1 // GPM, read-only
	effRegClarLevel    = 2 // ft, read-only
	effRegRASFlow      = 3 // GPM, read-only
	effRegWASFlow      = 4 // GPM, read-only
	effRegTSS          = 5 // mg/L, read-only
	effRegAmmonia      = 6 // mg/L, read-only
	effRegUVTrans      = 7 // %, read-only
	effRegPermitStatus = 8 // bitmask, read-only
)

// effluent coil addresses (one-based, SLC-500 ww-effluent variant).
const (
	effCoilRASPump      = 1 // ras_pump, writable
	effCoilWASPump      = 2 // was_pump, writable -- ATTACK SURFACE
	effCoilUVEnable     = 3 // uv_disinfection_enable, writable
	effCoilDischargeValv = 4 // effluent_discharge_valve, writable
)

// permit interlock bitmask values per device atom definition.
const (
	permitBitUVActive     = 1 << 0 // bit0: UV system active
	permitBitFlowOK       = 1 << 1 // bit1: flow within permit limit
	permitBitNPDESActive  = 1 << 2 // bit2: NPDES permit active (static true)
)

// EffluentModel tracks simulation state for the SLC-500 effluent discharge and secondary treatment.
type EffluentModel struct {
	store        *mbstore.RegisterStore
	profile      *device.DeviceProfile
	effFlowRate  float64 // GPM
	clarLevel    float64 // ft
	rasFlow      float64 // GPM
	wasFlow      float64 // GPM
	tss          float64 // mg/L
	ammonia      float64 // mg/L
	uvTrans      float64 // % (UV transmittance at 254nm)
	wasPumpRunTicks int  // consecutive ticks WAS pump has been running (sludge inventory impact)
}

// NewEffluentModel creates an EffluentModel for the given store and profile.
func NewEffluentModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *EffluentModel {
	return &EffluentModel{
		store:       store,
		profile:     profile,
		effFlowRate: 2000.0,
		clarLevel:   2.5,
		rasFlow:     1000.0,
		wasFlow:     15.0,
		tss:         15.0,
		ammonia:     5.0,
		uvTrans:     80.0,
	}
}

// Name returns the model identifier for logging.
func (m *EffluentModel) Name() string { return "ww-effluent" }

// Tick advances the effluent simulation by one second.
func (m *EffluentModel) Tick() {
	coils, err := m.store.ReadCoils(effCoilRASPump, 4)
	if err != nil {
		return
	}
	rasPumpOn := coils[0]
	wasPumpOn := coils[1]
	uvOn := coils[2]

	m.updateEffluentFlow()
	m.updateClarifier()
	m.updateRASFlow(rasPumpOn)
	m.updateWASFlow(wasPumpOn)
	m.updateTSS(wasPumpOn)
	m.updateAmmonia()
	m.updateUVTransmittance(uvOn)
	permit := m.computePermitInterlock(uvOn)
	m.writeRegisters(permit)
}

// updateEffluentFlow advances effluent flow with diurnal-like variation matching influent.
// [td-effluent-041] Independent simulation; no coupling to InfluentModel.
func (m *EffluentModel) updateEffluentFlow() {
	m.effFlowRate = clamp(addNoise(drift(m.effFlowRate, 2000.0, 0.5), 5000.0), 0, 5000)
}

// updateClarifier advances secondary clarifier level with slow variation around setpoint.
func (m *EffluentModel) updateClarifier() {
	m.clarLevel = clamp(addNoise(drift(m.clarLevel, 2.5, 0.001), 5.0), 0, 5)
}

// updateRASFlow advances return activated sludge flow (typically ~50% of effluent flow).
func (m *EffluentModel) updateRASFlow(rasPumpOn bool) {
	if rasPumpOn {
		m.rasFlow = clamp(addNoise(drift(m.rasFlow, m.effFlowRate*0.50, 1.0), 2500.0), 0, 2500)
	} else {
		m.rasFlow = clamp(m.rasFlow-5.0, 0, 2500)
	}
}

// updateWASFlow advances waste activated sludge flow based on pump coil state.
// WAS pump runs at 15-50 GPM in normal operation. Uncontrolled wasting attack education.
func (m *EffluentModel) updateWASFlow(wasPumpOn bool) {
	if wasPumpOn {
		m.wasPumpRunTicks++
		target := 30.0 // nominal WAS flow when pump is on
		m.wasFlow = clamp(addNoise(drift(m.wasFlow, target, 0.5), 100.0), 0, 100)
	} else {
		m.wasPumpRunTicks = 0
		m.wasFlow = clamp(m.wasFlow-1.0, 0, 100)
	}
}

// updateTSS advances effluent total suspended solids.
// Normal ~15 mg/L with noise. Excessive WAS pumping over time degrades solids settlement.
// [OT-REVIEW] TSS rises as activated sludge inventory declines from over-wasting.
func (m *EffluentModel) updateTSS(wasPumpOn bool) {
	target := 15.0
	// Prolonged over-wasting (>3600 ticks = 1 hour continuous) begins degrading treatment.
	if wasPumpOn && m.wasPumpRunTicks > 3600 {
		wasRunHours := float64(m.wasPumpRunTicks) / 3600.0
		target = 15.0 + wasRunHours*2.0 // each hour of over-wasting raises TSS ~2 mg/L
	}
	m.tss = clamp(addNoise(drift(m.tss, target, 0.01), 50.0), 0, 50)
}

// updateAmmonia advances effluent ammonia nitrogen.
// Normal ~5 mg/L; rises slowly if prolonged over-wasting has damaged activated sludge.
// [td-effluent-041] No coupling to aeration DO. Independent estimate.
func (m *EffluentModel) updateAmmonia() {
	target := 5.0
	if m.wasPumpRunTicks > 7200 { // 2+ hours of over-wasting
		wasRunHours := float64(m.wasPumpRunTicks) / 3600.0
		target = 5.0 + wasRunHours*0.5
	}
	m.ammonia = clamp(addNoise(drift(m.ammonia, target, 0.005), 30.0), 0, 30)
}

// updateUVTransmittance advances UV transmittance (inverse correlation with TSS).
// High TSS reduces UV transmittance; when UV system is off, transmittance drifts lower.
func (m *EffluentModel) updateUVTransmittance(uvOn bool) {
	// UV transmittance is an effluent quality indicator: high TSS = low transmittance.
	// Normal range: 60-90% for secondary effluent.
	tssFactor := (50.0 - m.tss) / 50.0 // higher TSS = lower factor
	target := 55.0 + tssFactor*35.0     // maps 0-50 mg/L TSS to 90-55% transmittance
	if !uvOn {
		// UV system off: transmittance drifts down (lamp not warming the probe optically)
		target -= 5.0
	}
	m.uvTrans = clamp(addNoise(drift(m.uvTrans, target, 0.1), 100.0), 0, 100)
}

// computePermitInterlock constructs the discharge permit interlock bitmask.
// bit0: UV active. bit1: flow within permit (< permit limit). bit2: NPDES permit active.
// [OT-REVIEW] All bits must be set before effluent_discharge_valve can legitimately open.
func (m *EffluentModel) computePermitInterlock(uvOn bool) uint16 {
	var permit uint16
	if uvOn {
		permit |= permitBitUVActive
	}
	// Permit flow limit: 5000 GPM (typical for 2-7 MGD plant; within permit if < 5000 GPM)
	if m.effFlowRate < 5000.0 {
		permit |= permitBitFlowOK
	}
	permit |= permitBitNPDESActive // always set -- plant has a valid NPDES permit
	return permit
}

// writeRegisters writes all effluent holding registers to the store (one-based, start at addr 1).
func (m *EffluentModel) writeRegisters(permit uint16) {
	regs := []uint16{
		toRaw(m.effFlowRate, 0, 5000),
		toRaw(m.clarLevel, 0, 5),
		toRaw(m.rasFlow, 0, 2500),
		toRaw(m.wasFlow, 0, 100),
		toRaw(m.tss, 0, 50),
		toRaw(m.ammonia, 0, 30),
		toRaw(m.uvTrans, 0, 100),
		permit,
	}
	_ = m.store.WriteHoldingInternal(effRegFlowRate, regs)
}
