package eventstore

import "fmt"

// FuncCodeInfo describes a Modbus function code for display and classification.
type FuncCodeInfo struct {
	Code     uint8
	Name     string // e.g., "Read Holding Registers"
	Category string // "read", "write", "diagnostic", or "unknown"
	IsWrite  bool
}

// ExceptionCodeInfo describes a Modbus exception code for display.
// Exception codes appear in the response when a device cannot execute a request.
type ExceptionCodeInfo struct {
	Code        uint8
	Name        string // canonical Modbus name
	Description string // educational description for dashboard display
}

// funcCodeTable maps Modbus function code numbers to their canonical information.
// Covers the function codes used in the OT simulator's monitored environments.
//
// FC 43 (MEI Transport -- Device Identification, sub-function 14) is classified
// as diagnostic and marked IsWrite=false for typical usage. However, trainees
// should be aware that FC 43 was used as a reconnaissance vector in the
// TRITON/TRISIS attack (2017, Schneider Electric Triconex safety controllers):
// attackers used FC 43 to enumerate the device identification of safety PLCs
// before deploying the TRITON payload. FC 43 observed from an unknown source
// address during normal operations is a high-fidelity indicator of compromise.
// Multiple sub-functions exist under FC 43; sub-function 0x0D (CANopen) can
// carry write payloads, making FC 43 versatile for both reconnaissance and
// potential exploitation.
var funcCodeTable = map[uint8]FuncCodeInfo{
	1:  {Code: 1, Name: "Read Coils", Category: "read", IsWrite: false},
	2:  {Code: 2, Name: "Read Discrete Inputs", Category: "read", IsWrite: false},
	3:  {Code: 3, Name: "Read Holding Registers", Category: "read", IsWrite: false},
	4:  {Code: 4, Name: "Read Input Registers", Category: "read", IsWrite: false},
	5:  {Code: 5, Name: "Write Single Coil", Category: "write", IsWrite: true},
	6:  {Code: 6, Name: "Write Single Register", Category: "write", IsWrite: true},
	15: {Code: 15, Name: "Write Multiple Coils", Category: "write", IsWrite: true},
	16: {Code: 16, Name: "Write Multiple Registers", Category: "write", IsWrite: true},
	43: {Code: 43, Name: "MEI Transport (Device Identification)", Category: "diagnostic", IsWrite: false},
}

// exceptionCodeTable maps standard Modbus exception codes 1-8 to their
// canonical names and educational descriptions.
//
// [OT-REVIEW] Exception codes have specific operational meanings that trainees
// must recognize in the dashboard rather than raw numbers:
//   - Code 02 (Illegal Data Address) is the most common in OT environments,
//     often indicating a misconfigured SCADA poll or an attacker probing the
//     device's register map beyond declared boundaries.
//   - Code 04 (Slave Device Failure) indicates a hardware fault, which is
//     critical for uptime monitoring and should trigger immediate investigation.
var exceptionCodeTable = map[uint8]ExceptionCodeInfo{
	1: {
		Code:        1,
		Name:        "Illegal Function",
		Description: "The device does not support this function code. Common when sending FC 5/6/15/16 to a read-only device or a device that does not implement write operations.",
	},
	2: {
		Code:        2,
		Name:        "Illegal Data Address",
		Description: "The requested address is outside the device's register map. Most common exception in OT -- often indicates a misconfigured SCADA poll targeting non-existent registers, or an attacker probing beyond declared register boundaries.",
	},
	3: {
		Code:        3,
		Name:        "Illegal Data Value",
		Description: "The value in the request is outside the valid range for this register. Indicates a write with an out-of-range setpoint or an invalid coil state encoding.",
	},
	4: {
		Code:        4,
		Name:        "Slave Device Failure",
		Description: "The device encountered an unrecoverable error while attempting to execute the request. Indicates a hardware fault or internal device error; critical for uptime monitoring.",
	},
	5: {
		Code:        5,
		Name:        "Acknowledge",
		Description: "The device accepted the request but requires time to process it. The master should poll for completion. Rare in modern devices; more common in legacy hardware with slow flash write operations.",
	},
	6: {
		Code:        6,
		Name:        "Slave Device Busy",
		Description: "The device is currently processing a long-duration command and cannot accept a new request. The master should retry after a delay.",
	},
	8: {
		Code:        8,
		Name:        "Memory Parity Error",
		Description: "The device detected a parity error in the extended memory area during a read/write operation. Indicates potential hardware degradation or storage fault.",
	},
	11: {
		Code:        11,
		Name:        "Gateway Path Unavailable",
		Description: "The Modbus gateway cannot route the request to the target device. The gateway is configured but the downstream path (RS-485 bus or serial link) is not available.",
	},
	12: {
		Code:        12,
		Name:        "Gateway Target Device Failed to Respond",
		Description: "The gateway received the request but the target device on the downstream bus did not respond within the timeout. Common when a serial slave device is powered off or its address is wrong.",
	},
}

// LookupFuncCode returns canonical information about a Modbus function code.
// Unknown codes return a generic entry with category "unknown" and IsWrite false.
func LookupFuncCode(fc uint8) FuncCodeInfo {
	if info, ok := funcCodeTable[fc]; ok {
		return info
	}
	return FuncCodeInfo{
		Code:     fc,
		Name:     fmt.Sprintf("Unknown (FC %d)", fc),
		Category: "unknown",
		IsWrite:  false,
	}
}

// LookupExceptionCode returns canonical information about a Modbus exception code.
// Unknown codes return a generic entry with name "Unknown Exception".
func LookupExceptionCode(code uint8) ExceptionCodeInfo {
	if info, ok := exceptionCodeTable[code]; ok {
		return info
	}
	return ExceptionCodeInfo{
		Code:        code,
		Name:        "Unknown Exception",
		Description: fmt.Sprintf("Exception code %d is not defined in the Modbus specification.", code),
	}
}
