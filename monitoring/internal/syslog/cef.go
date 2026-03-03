package syslog

import (
	"fmt"
	"strings"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// CEF severity levels used in the CEF header field (0-10 scale).
// The mapping is based on the operational risk level of each Modbus operation
// in an OT environment, per SOW-031.0 FR-4.
const (
	// cefSeverityWrite is assigned to all write operations (FC 5, 6, 15, 16)
	// regardless of success or failure. In OT environments, the act of writing
	// to a process control device has physical consequences (motor start, valve
	// actuate, setpoint change). A failed write attempt (exception code response)
	// still warrants High severity because it indicates someone tried to modify
	// a process control register, which may signal an intrusion attempt or a
	// misconfigured client that could cause safety incidents.
	cefSeverityWrite = 7 // High

	// cefSeverityReadFailure is assigned to read operations that return a
	// Modbus exception. Read failures may indicate device probing (attacker
	// scanning beyond declared register boundaries) or device misconfiguration.
	// Medium severity ensures these appear in standard SOC dashboards.
	cefSeverityReadFailure = 5 // Medium

	// cefSeverityDiagnostic is assigned to diagnostic function codes (FC 43 MEI
	// Transport / Device Identification). In the TRITON/TRISIS attack (2017,
	// Schneider Electric Triconex), FC 43 was used for device enumeration before
	// deploying the TRITON payload. FC 43 from an unknown source during normal
	// operations is a high-fidelity indicator of compromise. Severity 3 (above
	// Informational) ensures trainees building SIEM alert rules will see these
	// events in threat-hunting dashboards rather than filtering them as noise.
	cefSeverityDiagnostic = 3 // Medium-Low

	// cefSeverityReadSuccess is the baseline severity for successful read
	// operations. Reads are the expected steady-state activity of SCADA polling
	// and are informational by nature.
	cefSeverityReadSuccess = 1 // Low/Informational
)

// Syslog severity codes (RFC 5424). These are distinct from CEF severity levels.
// The CEF-to-syslog severity mapping is intentional for OT monitoring:
//
//   - Writes -> syslog Critical (2): Writes to process control devices warrant
//     pager-level alerting in OT environments. Physical consequences (motor start,
//     setpoint change) make writes categorically more dangerous than reads.
//   - Read failures -> syslog Warning (4): Potential device probing or misconfiguration.
//   - Diagnostics -> syslog Notice (5): Device enumeration is a reconnaissance
//     indicator; above Informational so it surfaces in threat-hunting queries.
//   - Read successes -> syslog Informational (6): Expected steady-state polling activity.
const (
	syslogSeverityCritical      = 2 // used for write operations
	syslogSeverityWarning       = 4 // used for read failures
	syslogSeverityNotice        = 5 // used for diagnostic operations
	syslogSeverityInformational = 6 // used for read successes
)

// cefSeverityToSyslog maps a CEF severity value to its corresponding RFC 5424
// syslog severity code. See the constant block above for the mapping rationale.
func cefSeverityToSyslog(cefSev int) int {
	switch cefSev {
	case cefSeverityWrite:
		return syslogSeverityCritical
	case cefSeverityReadFailure:
		return syslogSeverityWarning
	case cefSeverityDiagnostic:
		return syslogSeverityNotice
	default:
		return syslogSeverityInformational
	}
}

// cefSeverity returns the CEF severity (1, 3, 5, or 7) for a TransactionEvent.
// Priority order: write > diagnostic > read failure > read success.
// IsWrite is checked first because a failed write is still a write -- the act
// of attempting to modify a process control register is High severity regardless
// of whether the device accepted the request.
func cefSeverity(e *eventstore.TransactionEvent) int {
	if e.IsWrite {
		return cefSeverityWrite
	}
	info := eventstore.LookupFuncCode(e.FunctionCode)
	if info.Category == "diagnostic" {
		return cefSeverityDiagnostic
	}
	if !e.Success {
		return cefSeverityReadFailure
	}
	return cefSeverityReadSuccess
}

// escapeCEFHeader escapes pipe characters and backslashes in CEF header fields.
// CEF header fields are separated by "|" so literal pipes must be escaped as
// "\|". Backslashes must be escaped as "\\" per the ArcSight CEF v25 spec.
// This applies to func_name values that may contain parentheses but not pipes;
// the escaping is applied defensively to all header string fields.
func escapeCEFHeader(s string) string {
	// Escape backslashes first to avoid double-escaping.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `|`, `\|`)
	return s
}

// FormatCEF formats a single TransactionEvent as a complete syslog line
// containing a CEF-formatted message. The syslog priority is prepended in
// RFC 3164 format: <priority>CEF:0|...
//
// The rt (receipt time) extension field is set to milliseconds since the Unix
// epoch, as required by the ArcSight CEF v25 specification. Formatted time
// strings are NOT used; rt must be a numeric value for SIEM ingest pipelines.
//
// CEF field names use AddressStart/AddressCount (cn1/cn2 labels) to match the
// event model's protocol-neutral naming across all four Modbus address spaces
// (coils 0x, discrete inputs 1x, input registers 3x, holding registers 4x).
func FormatCEF(e *eventstore.TransactionEvent, facilityNum int) string {
	cefSev := cefSeverity(e)
	syslogSev := cefSeverityToSyslog(cefSev)
	priority := facilityNum*8 + syslogSev

	outcome := "success"
	if !e.Success {
		outcome = "failure"
	}

	header := buildCEFHeader(e, cefSev)
	extensions := buildCEFExtensions(e, outcome)

	return fmt.Sprintf("<%d>%s|%s", priority, header, extensions)
}

// buildCEFHeader constructs the pipe-delimited CEF header fields.
// Format: CEF:version|vendor|product|version|signatureId|name|severity
func buildCEFHeader(e *eventstore.TransactionEvent, cefSev int) string {
	return fmt.Sprintf("CEF:0|OTSimulator|Monitor|0.6|%d|%s|%d",
		e.FunctionCode,
		escapeCEFHeader(e.FunctionName),
		cefSev,
	)
}

// buildCEFExtensions constructs the space-separated key=value extension pairs.
// Field order follows Splunk CEF add-on conventions: source/destination first,
// then classifier fields, then numeric fields, then timestamp and outcome.
func buildCEFExtensions(e *eventstore.TransactionEvent, outcome string) string {
	return fmt.Sprintf(
		"src=%s dst=%s cs1=%s cs1Label=FunctionCode cn1=%d cn1Label=AddressStart cn2=%d cn2Label=AddressCount cs2=%s cs2Label=DeviceID cs3=%s cs3Label=Environment rt=%d outcome=%s",
		e.SrcAddr,
		e.DstAddr,
		e.FunctionName,
		e.AddressStart,
		e.AddressCount,
		e.DeviceID,
		e.EnvID,
		e.Timestamp.UnixMilli(),
		outcome,
	)
}
