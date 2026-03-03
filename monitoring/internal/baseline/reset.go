package baseline

import (
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/timeseries"
)

// Reset clears all device baselines, returning every device to the learning state.
// The knownDeviceIDs set is preserved to avoid false "new device" alerts after reset.
// Only baseline statistics, register/coil buffers, and sample accumulators are cleared.
//
// [OT-REVIEW] Full reset is equivalent to disabling the IDS for all devices during
// the re-learning period. The admin CLI enforces a confirmation prompt before calling
// this endpoint (see FR-11). knownDeviceIDs is preserved per FR-11 requirements.
//
// Returns the number of device baselines that were cleared.
func (e *Engine) Reset() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := len(e.baselines)

	e.baselines = make(map[string]*DeviceBaseline)
	e.registerBuffers = make(map[string][]*timeseries.RingBuffer)
	e.coilBuffers = make(map[string][]*timeseries.CoilRingBuffer)
	e.sampleAccum = make(map[string][][]uint16)
	// knownDeviceIDs is NOT cleared -- preserves known device set to prevent
	// false "new device" alerts from firing on the next poll cycle after reset.

	return count
}

// ResetDevice clears the baseline for a single device, returning it to the learning state.
// The device remains in knownDeviceIDs to prevent a false "new device" alert.
// If the device ID is not found, the call is a no-op and returns false.
func (e *Engine) ResetDevice(deviceID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.baselines[deviceID]; !ok {
		return false
	}

	delete(e.baselines, deviceID)
	delete(e.registerBuffers, deviceID)
	delete(e.coilBuffers, deviceID)
	delete(e.sampleAccum, deviceID)
	// knownDeviceIDs entry for this device is preserved.

	return true
}
