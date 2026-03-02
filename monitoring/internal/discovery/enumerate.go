// enumerate.go contains the register and coil enumeration algorithms used during
// device discovery. The binary-search approach minimizes Modbus reads: O(log N)
// reads to determine register count, where N is the maximum possible count.
package discovery

import (
	"fmt"
	"math"
	"time"

	"github.com/simonvetter/modbus"
)

// PROTOTYPE-DEBT: [td-enumerate-025] Input registers and discrete inputs are not
// enumerated or polled. Only holding registers and coils are supported. Low priority
// because the simulator primarily uses holding registers and coils.

// enumerateHoldingRegisters determines register count and addressing mode via
// binary search. The algorithm reads O(log N) registers to bound the count.
//
// Addressing mode detection: a read at address 0 succeeds for zero-based devices;
// a read at address 1 succeeds (and 0 fails) for one-based devices.
//
// Count formula: `low` (the converged binary-search bound) is always the highest
// valid relative offset from baseAddr. Count = low + 1 in both addressing modes:
//   - Zero-based: addresses 0..low, count = low+1.
//   - One-based: addresses 1..(1+low), count = low+1; and 1+low == count by definition.
func enumerateHoldingRegisters(client ClientInterface) (count int, addressing string, err error) {
	baseAddr, addressing, found := detectHoldingBase(client)
	if !found {
		return 0, "unknown", nil
	}

	upper := expandUpper(client, baseAddr)
	low := binarySearchHolding(client, baseAddr, upper/2, upper)

	return int(low) + 1, addressing, nil
}

// detectHoldingBase probes addresses 0 and 1 to determine addressing mode.
// Returns (baseAddr, mode, true) on success or ("", false) if no registers exist.
func detectHoldingBase(client ClientInterface) (baseAddr uint16, addressing string, found bool) {
	_, err0 := client.ReadRegisters(0, 1, modbus.HOLDING_REGISTER)
	time.Sleep(discoveryReadDelay)

	_, err1 := client.ReadRegisters(1, 1, modbus.HOLDING_REGISTER)
	time.Sleep(discoveryReadDelay)

	switch {
	case err0 == nil:
		return 0, "zero-based", true
	case err1 == nil:
		return 1, "one-based", true
	default:
		return 0, "", false
	}
}

// expandUpper doubles probe until a read fails, returning the first failing probe value.
// The highest valid relative offset is between probe/2 and probe.
func expandUpper(client ClientInterface, baseAddr uint16) uint16 {
	probe := uint16(1)
	for probe <= maxRegisters {
		addr := baseAddr + probe
		if addr > maxRegisters {
			break
		}
		_, err := client.ReadRegisters(addr, 1, modbus.HOLDING_REGISTER)
		time.Sleep(discoveryReadDelay)
		if err != nil {
			break
		}
		if probe*2 > maxRegisters {
			probe = maxRegisters
			break
		}
		probe *= 2
	}
	return probe
}

// binarySearchHolding bisects [low, high] to find the highest valid relative offset.
func binarySearchHolding(client ClientInterface, baseAddr, low, high uint16) uint16 {
	for low < high {
		mid := (low + high + 1) / 2
		addr := baseAddr + mid
		if addr > maxRegisters {
			high = mid - 1
			continue
		}
		_, err := client.ReadRegisters(addr, 1, modbus.HOLDING_REGISTER)
		time.Sleep(discoveryReadDelay)
		if err != nil {
			high = mid - 1
		} else {
			low = mid
		}
	}
	return low
}

// enumerateCoils determines coil count using the same binary-search approach
// as enumerateHoldingRegisters.
func enumerateCoils(client ClientInterface) (count int, err error) {
	baseAddr, found := detectCoilBase(client)
	if !found {
		return 0, nil
	}

	upper := expandUpperCoils(client, baseAddr)
	low := binarySearchCoils(client, baseAddr, upper/2, upper)

	return int(low) + 1, nil
}

// detectCoilBase probes coil addresses 0 and 1 to find base address.
func detectCoilBase(client ClientInterface) (baseAddr uint16, found bool) {
	_, err0 := client.ReadCoils(0, 1)
	time.Sleep(discoveryReadDelay)

	_, err1 := client.ReadCoils(1, 1)
	time.Sleep(discoveryReadDelay)

	switch {
	case err0 == nil:
		return 0, true
	case err1 == nil:
		return 1, true
	default:
		return 0, false
	}
}

// expandUpperCoils doubles probe until a coil read fails.
func expandUpperCoils(client ClientInterface, baseAddr uint16) uint16 {
	probe := uint16(1)
	for probe <= maxRegisters {
		addr := baseAddr + probe
		if addr > maxRegisters {
			break
		}
		_, err := client.ReadCoils(addr, 1)
		time.Sleep(discoveryReadDelay)
		if err != nil {
			break
		}
		if probe*2 > maxRegisters {
			probe = maxRegisters
			break
		}
		probe *= 2
	}
	return probe
}

// binarySearchCoils bisects [low, high] to find the highest valid coil offset.
func binarySearchCoils(client ClientInterface, baseAddr, low, high uint16) uint16 {
	for low < high {
		mid := (low + high + 1) / 2
		addr := baseAddr + mid
		if addr > maxRegisters {
			high = mid - 1
			continue
		}
		_, err := client.ReadCoils(addr, 1)
		time.Sleep(discoveryReadDelay)
		if err != nil {
			high = mid - 1
		} else {
			low = mid
		}
	}
	return low
}

// measureResponseTime records RTT for responseSamples reads and returns mean
// and standard deviation (jitter) in milliseconds.
func measureResponseTime(client ClientInterface, baseAddr uint16, regCount int) (meanMs float64, jitterMs float64, err error) {
	if regCount == 0 {
		return 0, 0, nil
	}

	samples := make([]float64, 0, responseSamples)
	for i := 0; i < responseSamples; i++ {
		start := time.Now()
		_, readErr := client.ReadRegisters(baseAddr, 1, modbus.HOLDING_REGISTER)
		elapsed := time.Since(start)
		if readErr != nil {
			return 0, 0, fmt.Errorf("RTT sample %d: %w", i, readErr)
		}
		samples = append(samples, float64(elapsed.Nanoseconds())/1e6)
	}

	mean, jitter := computeMeanStddev(samples)
	return mean, jitter, nil
}

// computeMeanStddev computes mean and population standard deviation of a sample set.
func computeMeanStddev(samples []float64) (mean, stddev float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	for _, s := range samples {
		mean += s
	}
	mean /= float64(len(samples))

	variance := 0.0
	for _, s := range samples {
		d := s - mean
		variance += d * d
	}
	variance /= float64(len(samples))

	return mean, math.Sqrt(variance)
}
