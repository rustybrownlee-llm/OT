// Package eventstore provides persistent SQLite storage for Modbus transaction
// events captured by the monitoring poller. It implements ADR-011 Layer 1
// (Protocol Telemetry) -- the foundational event type and store that all Beta
// 0.6 telemetry work builds upon.
package eventstore

import (
	"crypto/rand"
	"fmt"
	"time"
)

// TransactionEvent represents a single Modbus transaction captured by the
// monitoring poller. This is the foundational event type for ADR-011 Layer 1
// (Protocol Telemetry). Each polling read or write produces one event.
//
// [OT-REVIEW] SrcAddr in Beta 0.6 is always the monitor's ephemeral client
// address because the poller is the only active Modbus client. This field
// becomes meaningful in Beta 0.7 (passive capture) when third-party clients
// (including attackers) are observed.
type TransactionEvent struct {
	ID             string       // UUID v4, generated on creation
	Timestamp      time.Time    // when the transaction completed
	SrcAddr        string       // source IP:port (monitor's address)
	DstAddr        string       // destination IP:port (device endpoint)
	UnitID         uint8        // Modbus unit/slave ID
	FunctionCode   uint8        // Modbus function code (1-127)
	FunctionName   string       // human-readable ("Read Holding Registers")
	AddressStart   uint16       // starting address (coil address for FC 1/2/5/15; register address for FC 3/4/6/16)
	AddressCount   uint16       // number of addresses in request (coils or registers depending on FC)
	IsWrite        bool         // true for FC 5, 6, 15, 16
	Success        bool         // completed without Modbus exception
	ExceptionCode  uint8        // Modbus exception code if !Success
	ResponseTimeUs int64        // round-trip time in microseconds
	DeviceID       string       // resolved asset ID (ip:port:unit_id)
	DeviceName     string       // human-readable name from design library (populated by SOW-028.0; empty until then)
	EnvID          string       // environment ID from config
	WriteDetail    *WriteDetail // non-nil for write operations only
}

// WriteDetail holds the values written in a Modbus write operation.
// Stored as JSON in the SQLite write_values column. Nil for read operations.
type WriteDetail struct {
	Values     []uint16 `json:"values,omitempty"`      // FC 6, 16
	CoilValues []bool   `json:"coil_values,omitempty"` // FC 5, 15
}

// FilterOptions specifies query filters for event retrieval.
// All non-nil fields are combined with AND logic.
// A zero-value FilterOptions returns all events.
//
// [OT-REVIEW] AddressStart/AddressCount use protocol-neutral naming because
// Modbus has four separate address spaces (coils 0x, discrete inputs 1x,
// input registers 3x, holding registers 4x). Using "register" terminology
// for coil operations would teach incorrect address space semantics.
type FilterOptions struct {
	After    *time.Time // events at or after this timestamp (inclusive)
	Before   *time.Time // events before this timestamp (exclusive)
	DeviceID *string    // exact device ID match
	FuncCode *uint8     // exact function code match
	IsWrite  *bool      // filter by write operations
	Success  *bool      // filter by success/failure
	Limit    int        // max results (0 = no limit; capped at 10000)
	Offset   int        // pagination offset
}

// PROTOTYPE-DEBT: [td-events-067] UUID generation uses crypto/rand directly
// instead of a UUID library. Compliant with RFC 4122 v4 but does not validate
// parsed UUIDs. No resolution needed unless UUID parsing is required in a
// future SOW.

// newUUID generates a random UUID v4 string using crypto/rand.
// No external library -- 10 lines of standard library.
func newUUID() string {
	var u [16]byte
	_, _ = rand.Read(u[:])
	u[6] = (u[6] & 0x0f) | 0x40 // version 4
	u[8] = (u[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// NewEvent constructs a TransactionEvent with a generated UUID and the
// provided field values. Callers supply all fields; the ID is assigned here.
func NewEvent(
	timestamp time.Time,
	srcAddr, dstAddr string,
	unitID uint8,
	fc uint8,
	addressStart, addressCount uint16,
	isWrite, success bool,
	exceptionCode uint8,
	responseTimeUs int64,
	deviceID, deviceName, envID string,
	detail *WriteDetail,
) *TransactionEvent {
	info := LookupFuncCode(fc)
	return &TransactionEvent{
		ID:             newUUID(),
		Timestamp:      timestamp,
		SrcAddr:        srcAddr,
		DstAddr:        dstAddr,
		UnitID:         unitID,
		FunctionCode:   fc,
		FunctionName:   info.Name,
		AddressStart:   addressStart,
		AddressCount:   addressCount,
		IsWrite:        isWrite,
		Success:        success,
		ExceptionCode:  exceptionCode,
		ResponseTimeUs: responseTimeUs,
		DeviceID:       deviceID,
		DeviceName:     deviceName,
		EnvID:          envID,
		WriteDetail:    detail,
	}
}
