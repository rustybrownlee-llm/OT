package syslog_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	internalsyslog "github.com/rustybrownlee/ot-simulator/monitoring/internal/syslog"
)

// makeEvent is a test helper that builds a TransactionEvent with common fields
// pre-populated. Callers override specific fields to exercise different paths.
func makeEvent(fc uint8, isWrite, success bool) *eventstore.TransactionEvent {
	ts := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	return &eventstore.TransactionEvent{
		ID:           "test-uuid",
		Timestamp:    ts,
		SrcAddr:      "192.168.10.5:52100",
		DstAddr:      "192.168.1.20:5020",
		UnitID:       1,
		FunctionCode: fc,
		FunctionName: eventstore.LookupFuncCode(fc).Name,
		AddressStart: 100,
		AddressCount: 10,
		IsWrite:      isWrite,
		Success:      success,
		DeviceID:     "192.168.1.20:5020:1",
		EnvID:        "water-treatment",
	}
}

// TestFormatCEF_ReadSuccess verifies the CEF output for a successful read (FC 3).
// Severity must be 1 (Low) in the CEF header and 6 (Informational) in the
// syslog priority.
func TestFormatCEF_ReadSuccess(t *testing.T) {
	e := makeEvent(3, false, true)
	facilityNum := 16 // local0

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(16) * 8 + syslogSeverity(6) = 134
	assertContains(t, msg, "<134>", "syslog priority for read success")
	assertContains(t, msg, "CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|",
		"CEF header with severity 1")
	assertContains(t, msg, "src=192.168.10.5:52100", "src address")
	assertContains(t, msg, "dst=192.168.1.20:5020", "dst address")
	assertContains(t, msg, "cn1Label=AddressStart", "AddressStart label")
	assertContains(t, msg, "cn2Label=AddressCount", "AddressCount label")
	assertContains(t, msg, "outcome=success", "outcome field")
}

// TestFormatCEF_ReadFailure verifies the CEF output for a failed read (FC 3,
// exception returned). Severity must be 5 (Medium) in CEF, 4 (Warning) in syslog.
func TestFormatCEF_ReadFailure(t *testing.T) {
	e := makeEvent(3, false, false)
	e.ExceptionCode = 2
	facilityNum := 16 // local0

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(16) * 8 + syslogSeverity(4) = 132
	assertContains(t, msg, "<132>", "syslog priority for read failure")
	assertContains(t, msg, "|Read Holding Registers|5|", "CEF severity 5 for read failure")
	assertContains(t, msg, "outcome=failure", "outcome field")
}

// TestFormatCEF_Write verifies the CEF output for a write operation (FC 6).
// Severity must be 7 (High) in CEF, 2 (Critical) in syslog -- regardless of
// success. Writes to process control devices warrant pager-level alerting.
func TestFormatCEF_Write(t *testing.T) {
	e := makeEvent(6, true, true)
	facilityNum := 16 // local0

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(16) * 8 + syslogSeverity(2) = 130
	assertContains(t, msg, "<130>", "syslog priority for write")
	assertContains(t, msg, "|Write Single Register|7|", "CEF severity 7 for write")
	assertContains(t, msg, "outcome=success", "outcome field")
}

// TestFormatCEF_WriteFailure verifies that a FAILED write still produces
// severity 7 (High). The attempt to modify a process control register is
// High severity regardless of whether the device accepted the request.
func TestFormatCEF_WriteFailure(t *testing.T) {
	e := makeEvent(6, true, false)
	facilityNum := 16 // local0

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(16) * 8 + syslogSeverity(2) = 130
	assertContains(t, msg, "<130>", "syslog priority for failed write")
	assertContains(t, msg, "|7|", "CEF severity 7 for failed write")
	assertContains(t, msg, "outcome=failure", "outcome field")
}

// TestFormatCEF_Diagnostic verifies the CEF output for FC 43 (MEI Transport /
// Device Identification). Severity must be 3 (Medium-Low) in CEF, 5 (Notice)
// in syslog. FC 43 is a known reconnaissance indicator (TRITON/TRISIS context).
func TestFormatCEF_Diagnostic(t *testing.T) {
	e := makeEvent(43, false, true)
	facilityNum := 16 // local0

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(16) * 8 + syslogSeverity(5) = 133
	assertContains(t, msg, "<133>", "syslog priority for diagnostic")
	assertContains(t, msg, "|MEI Transport (Device Identification)|3|",
		"CEF severity 3 for diagnostic FC 43")
}

// TestFormatCEF_RTNumericMilliseconds verifies that the rt (receipt time)
// extension field is a numeric epoch milliseconds value, not a formatted time
// string. The ArcSight CEF v25 specification defines rt as milliseconds since
// the Unix epoch. Formatted strings cause parse errors in SIEM ingest pipelines.
func TestFormatCEF_RTNumericMilliseconds(t *testing.T) {
	ts := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	e := makeEvent(3, false, true)
	e.Timestamp = ts

	msg := internalsyslog.FormatCEF(e, 16)

	// Find "rt=" and extract the value following it.
	const prefix = "rt="
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		t.Fatalf("rt field not found in CEF output: %s", msg)
	}

	start := idx + len(prefix)
	end := start
	for end < len(msg) && msg[end] != ' ' {
		end++
	}
	rtValue := msg[start:end]

	// rt must be parseable as a base-10 integer.
	rtMs, err := strconv.ParseInt(rtValue, 10, 64)
	if err != nil {
		t.Fatalf("rt field %q is not a numeric integer: %v", rtValue, err)
	}

	// Verify it matches the expected epoch milliseconds.
	want := ts.UnixMilli()
	if rtMs != want {
		t.Errorf("rt field: got %d ms, want %d ms (difference: %d ms)",
			rtMs, want, rtMs-want)
	}
}

// TestFormatCEF_AddressFieldNames verifies that CEF extension labels use
// AddressStart and AddressCount (not RegisterStart/RegisterCount). Modbus has
// four address spaces; "register" implies the holding register space only.
func TestFormatCEF_AddressFieldNames(t *testing.T) {
	e := makeEvent(1, false, true) // FC 1 = Read Coils (coil address space)
	e.AddressStart = 50
	e.AddressCount = 8

	msg := internalsyslog.FormatCEF(e, 16)

	assertContains(t, msg, "cn1=50", "AddressStart value")
	assertContains(t, msg, "cn1Label=AddressStart", "AddressStart label (not RegisterStart)")
	assertContains(t, msg, "cn2=8", "AddressCount value")
	assertContains(t, msg, "cn2Label=AddressCount", "AddressCount label (not RegisterCount)")
}

// TestFormatCEF_FacilityLocal2 verifies that the syslog priority is calculated
// correctly for a non-default facility (local2 = code 18).
func TestFormatCEF_FacilityLocal2(t *testing.T) {
	e := makeEvent(3, false, true)
	facilityNum := 18 // local2

	msg := internalsyslog.FormatCEF(e, facilityNum)

	// Syslog priority: facility(18) * 8 + syslogSeverity(6) = 150
	assertContains(t, msg, "<150>", "syslog priority for local2 + read success")
}

// TestFormatCEF_PipeEscapingInHeader verifies that pipe characters in function
// names are escaped in the CEF header. CEF headers are pipe-delimited; literal
// pipes break SIEM parser field extraction.
func TestFormatCEF_PipeEscapingInHeader(t *testing.T) {
	e := makeEvent(43, false, true)
	// Override the function name to contain a pipe character to test escaping.
	e.FunctionName = "MEI|Transport"

	msg := internalsyslog.FormatCEF(e, 16)

	// The pipe in the function name must be escaped as \| in the CEF header.
	// The unescaped pipe should not appear as a field separator.
	if strings.Contains(msg, "CEF:0|OTSimulator|Monitor|0.6|43|MEI|Transport|") {
		t.Error("unescaped pipe in CEF header: parser will split incorrectly")
	}
	assertContains(t, msg, `MEI\|Transport`, "escaped pipe in function name")
}

// assertContains is a test helper that fails if substr is not found in s.
func assertContains(t *testing.T, s, substr, description string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("%s: expected %q in output\ngot: %s", description, substr, s)
	}
}
