// rules.go defines the five anomaly detection rules evaluated after the
// baseline learning period completes.
//
// Rules operate on raw uint16 register values and boolean coil states.
// The monitor has no knowledge of register names, engineering units, or
// device semantics -- it observes only what is visible on the wire.
// This is architecturally correct per ADR-005 D4.
package baseline

import (
	"fmt"
	"math"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
)

// Rule represents one anomaly detection rule.
type Rule struct {
	ID       string
	Severity string
}

// Rules is the ordered list of anomaly detection rules. All five rules are
// evaluated on every established device after each polling cycle.
var Rules = []Rule{
	{ID: "value_out_of_range", Severity: "warning"},
	{ID: "unexpected_write", Severity: "high"},
	{ID: "new_device", Severity: "critical"},
	{ID: "device_offline", Severity: "high"},
	{ID: "response_time_anomaly", Severity: "warning"},
}

// evaluateValueOutOfRange checks each holding register against its baseline.
//
// - Constant registers: any deviation from the baseline mean triggers a Warning.
// - Analog registers: values outside mean ± 3*stddev trigger a Warning.
//   If the deviation exceeds 6*stddev OR the value falls outside the learned
//   min/max, severity escalates to High.
// - Counter registers: excluded from magnitude checks. A decreasing value
//   between consecutive polls triggers a High-severity counter_direction_change alert.
func evaluateValueOutOfRange(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	for i, v := range snap.Holding {
		if i >= len(db.RegisterStats) {
			break
		}
		rs := &db.RegisterStats[i]
		alertID := alert.AlertID("value_out_of_range", snap.DeviceID, i)

		switch rs.Type {
		case RegisterTypeConstant:
			if float64(v) != rs.Mean {
				store.GetOrCreate(alertID, func() *alert.Alert {
					return buildRegAlert(
						alertID, "value_out_of_range", "warning",
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Holding register %d value %d changed from constant baseline %g", i, v, rs.Mean),
						fmt.Sprintf("%g (constant)", rs.Mean),
						fmt.Sprintf("%d", v),
					)
				})
			} else {
				store.Clear(alertID, snap.Timestamp)
			}

		case RegisterTypeAnalog:
			deviation := math.Abs(float64(v) - rs.Mean)
			threshold3 := 3 * rs.StdDev
			threshold6 := 6 * rs.StdDev
			if deviation > threshold3 || rs.StdDev == 0 {
				severity := "warning"
				if deviation > threshold6 || v > rs.Max || v < rs.Min {
					severity = "high"
				}
				store.GetOrCreate(alertID, func() *alert.Alert {
					return buildRegAlert(
						alertID, "value_out_of_range", severity,
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Holding register %d value %d is outside baseline range [%d, %d] (mean=%.1f, stddev=%.1f)",
							i, v, rs.Min, rs.Max, rs.Mean, rs.StdDev),
						fmt.Sprintf("%.1f +/- %.1f (3-sigma)", rs.Mean, threshold3),
						fmt.Sprintf("%d", v),
					)
				})
			} else {
				store.Clear(alertID, snap.Timestamp)
			}

		case RegisterTypeCounter:
			// Counter direction change: value decreased between consecutive polls.
			// Sub-rule fires only if we have a previous value to compare.
			dirAlertID := alert.AlertID("value_out_of_range_counter", snap.DeviceID, i)
			if len(snap.PrevHolding) > i && int(v) < int(snap.PrevHolding[i]) {
				store.GetOrCreate(dirAlertID, func() *alert.Alert {
					return buildRegAlert(
						dirAlertID, "value_out_of_range", "high",
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Counter register %d decreased from %d to %d (possible register tampering or device reset)",
							i, snap.PrevHolding[i], v),
						fmt.Sprintf(">= %d (non-decreasing)", snap.PrevHolding[i]),
						fmt.Sprintf("%d", v),
					)
				})
			} else {
				store.Clear(dirAlertID, snap.Timestamp)
			}
		}
	}
}

// evaluateUnexpectedWrite checks for unexpected register and coil changes.
//
// - Constant registers: any change triggers High.
// - Analog registers: step change > 6*stddev triggers High.
// - Counter registers: exempt.
// - Locked coils (never toggled during learning): any state change triggers High.
// - Operational coils (toggled during learning): excessive toggle rate triggers Warning.
func evaluateUnexpectedWrite(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	evaluateUnexpectedWriteRegisters(snap, db, store)
	evaluateUnexpectedWriteCoils(snap, db, store)
}

func evaluateUnexpectedWriteRegisters(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	if len(snap.PrevHolding) == 0 {
		return // no previous values to compare
	}
	for i, v := range snap.Holding {
		if i >= len(db.RegisterStats) || i >= len(snap.PrevHolding) {
			break
		}
		rs := &db.RegisterStats[i]
		prev := snap.PrevHolding[i]
		alertID := alert.AlertID("unexpected_write", snap.DeviceID, i)

		switch rs.Type {
		case RegisterTypeConstant:
			if v != prev {
				store.GetOrCreate(alertID, func() *alert.Alert {
					return buildRegAlert(
						alertID, "unexpected_write", "high",
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Holding register %d (constant) changed from %d to %d", i, prev, v),
						fmt.Sprintf("%d (constant)", prev),
						fmt.Sprintf("%d", v),
					)
				})
			} else {
				store.Clear(alertID, snap.Timestamp)
			}

		case RegisterTypeAnalog:
			if rs.StdDev > 0 {
				stepChange := math.Abs(float64(v) - float64(prev))
				if stepChange > 6*rs.StdDev {
					store.GetOrCreate(alertID, func() *alert.Alert {
						return buildRegAlert(
							alertID, "unexpected_write", "high",
							snap.DeviceID, i, snap.Timestamp,
							fmt.Sprintf("Holding register %d step change %.0f exceeds 6-sigma threshold (%.1f)",
								i, stepChange, 6*rs.StdDev),
							fmt.Sprintf("step < %.1f (6-sigma)", 6*rs.StdDev),
							fmt.Sprintf("%d (prev=%d)", v, prev),
						)
					})
				} else {
					store.Clear(alertID, snap.Timestamp)
				}
			}

		case RegisterTypeCounter:
			// Counters are exempt from unexpected_write (they change by design).
			store.Clear(alertID, snap.Timestamp)
		}
	}
}

func evaluateUnexpectedWriteCoils(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	if len(snap.PrevCoils) == 0 {
		return
	}
	for i, v := range snap.Coils {
		if i >= len(db.CoilStats) || i >= len(snap.PrevCoils) {
			break
		}
		cs := &db.CoilStats[i]
		alertID := alert.AlertID("unexpected_write_coil", snap.DeviceID, i)

		isLocked := cs.ToggleCount == 0

		if isLocked {
			// Locked coil: any state change is High severity.
			if v != snap.PrevCoils[i] {
				store.GetOrCreate(alertID, func() *alert.Alert {
					return buildCoilAlert(
						alertID, "unexpected_write", "high",
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Coil %d (locked during baseline) changed state to %v", i, v),
						fmt.Sprintf("%v (locked coil, never toggled during learning)", snap.PrevCoils[i]),
						fmt.Sprintf("%v", v),
					)
				})
			} else {
				store.Clear(alertID, snap.Timestamp)
			}
		} else {
			// Operational coil: alert if toggle rate exceeds 3x the observed maximum.
			// We measure toggle rate as: did the coil change this cycle?
			// Compare to max toggle frequency (toggles per cycle).
			// currentRate is 1.0 if toggled, 0.0 if not.
			currentToggle := 0.0
			if v != snap.PrevCoils[i] {
				currentToggle = 1.0
			}
			threshold := 3 * cs.MaxToggleFrequency
			if threshold <= 0 {
				threshold = 0 // no-op if max was 0 (should not happen for operational coils)
			}
			if currentToggle > threshold {
				store.GetOrCreate(alertID, func() *alert.Alert {
					return buildCoilAlert(
						alertID, "unexpected_write", "warning",
						snap.DeviceID, i, snap.Timestamp,
						fmt.Sprintf("Coil %d toggle rate exceeds 3x baseline maximum (%.2f toggles/cycle)", i, cs.MaxToggleFrequency),
						fmt.Sprintf("<= %.2f toggles/cycle (3x max observed)", threshold),
						fmt.Sprintf("%.1f toggles/cycle this window", currentToggle),
					)
				})
			} else {
				store.Clear(alertID, snap.Timestamp)
			}
		}
	}
}

// evaluateNewDevice fires a Critical alert when a device ID is seen that was
// not present at the end of initial discovery. The device is added to the
// known set so the alert fires only once.
func evaluateNewDevice(snap *DeviceSnapshot, knownIDs map[string]bool, store *alert.Store) {
	if knownIDs[snap.DeviceID] {
		return
	}
	alertID := alert.AlertID("new_device", snap.DeviceID, -1)
	store.GetOrCreate(alertID, func() *alert.Alert {
		return &alert.Alert{
			ID:              alertID,
			Timestamp:       snap.Timestamp,
			Severity:        "critical",
			RuleID:          "new_device",
			DeviceID:        snap.DeviceID,
			Description:     fmt.Sprintf("Previously unknown device %q appeared during polling", snap.DeviceID),
			RegisterAddress: -1,
			ExpectedValue:   "device not present at discovery",
			ActualValue:     snap.DeviceID,
			Active:          true,
		}
	})
	// Add to known set to prevent re-alerting on every subsequent cycle.
	knownIDs[snap.DeviceID] = true
}

// evaluateDeviceOffline fires a High alert when a discovered device stops
// responding. The alert clears when the device comes back online.
// This rule runs regardless of whether the baseline is established (FR-16).
func evaluateDeviceOffline(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	alertID := alert.AlertID("device_offline", snap.DeviceID, -1)

	if !snap.Online {
		store.GetOrCreate(alertID, func() *alert.Alert {
			return &alert.Alert{
				ID:              alertID,
				Timestamp:       snap.Timestamp,
				Severity:        "high",
				RuleID:          "device_offline",
				DeviceID:        snap.DeviceID,
				Description:     fmt.Sprintf("Device %q is not responding to Modbus polls", snap.DeviceID),
				RegisterAddress: -1,
				ExpectedValue:   "online",
				ActualValue:     "offline",
				Active:          true,
			}
		})
	} else {
		store.Clear(alertID, snap.Timestamp)
	}

	// Suppress unused parameter lint for db (required by caller signature but not used here).
	_ = db
}

// evaluateResponseTimeAnomaly fires a Warning alert when a device's response
// time exceeds the baseline mean + 3*stddev.
func evaluateResponseTimeAnomaly(snap *DeviceSnapshot, db *DeviceBaseline, store *alert.Store) {
	if db.ResponseTimeMean <= 0 || db.ResponseTimeStdDev <= 0 {
		return
	}

	alertID := alert.AlertID("response_time_anomaly", snap.DeviceID, -1)
	threshold := db.ResponseTimeMean + 3*db.ResponseTimeStdDev

	if snap.ResponseMs > threshold {
		store.GetOrCreate(alertID, func() *alert.Alert {
			return &alert.Alert{
				ID:          alertID,
				Timestamp:   snap.Timestamp,
				Severity:    "warning",
				RuleID:      "response_time_anomaly",
				DeviceID:    snap.DeviceID,
				Description: fmt.Sprintf("Device %q response time %.1fms exceeds baseline threshold %.1fms (mean=%.1f, stddev=%.1f)",
					snap.DeviceID, snap.ResponseMs, threshold, db.ResponseTimeMean, db.ResponseTimeStdDev),
				RegisterAddress: -1,
				ExpectedValue:   fmt.Sprintf("<= %.1fms (mean + 3-sigma)", threshold),
				ActualValue:     fmt.Sprintf("%.1fms", snap.ResponseMs),
				Active:          true,
			}
		})
	} else {
		store.Clear(alertID, snap.Timestamp)
	}
}

// buildRegAlert constructs an Alert for a register-level anomaly.
func buildRegAlert(id, ruleID, severity, deviceID string, addr int, ts time.Time,
	desc, expected, actual string) *alert.Alert {
	return &alert.Alert{
		ID:              id,
		Timestamp:       ts,
		Severity:        severity,
		RuleID:          ruleID,
		DeviceID:        deviceID,
		Description:     desc,
		RegisterAddress: addr,
		ExpectedValue:   expected,
		ActualValue:     actual,
		Active:          true,
	}
}

// buildCoilAlert constructs an Alert for a coil-level anomaly.
func buildCoilAlert(id, ruleID, severity, deviceID string, addr int, ts time.Time,
	desc, expected, actual string) *alert.Alert {
	return &alert.Alert{
		ID:              id,
		Timestamp:       ts,
		Severity:        severity,
		RuleID:          ruleID,
		DeviceID:        deviceID,
		Description:     desc,
		RegisterAddress: addr,
		ExpectedValue:   expected,
		ActualValue:     actual,
		Active:          true,
	}
}
