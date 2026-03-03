package eventstore_test

import (
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

func TestLookupFuncCode_KnownCodes(t *testing.T) {
	tests := []struct {
		fc       uint8
		name     string
		category string
		isWrite  bool
	}{
		{1, "Read Coils", "read", false},
		{2, "Read Discrete Inputs", "read", false},
		{3, "Read Holding Registers", "read", false},
		{4, "Read Input Registers", "read", false},
		{5, "Write Single Coil", "write", true},
		{6, "Write Single Register", "write", true},
		{15, "Write Multiple Coils", "write", true},
		{16, "Write Multiple Registers", "write", true},
		{43, "MEI Transport (Device Identification)", "diagnostic", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := eventstore.LookupFuncCode(tt.fc)
			if info.Code != tt.fc {
				t.Errorf("Code: got %d, want %d", info.Code, tt.fc)
			}
			if info.Name != tt.name {
				t.Errorf("Name: got %q, want %q", info.Name, tt.name)
			}
			if info.Category != tt.category {
				t.Errorf("Category: got %q, want %q", info.Category, tt.category)
			}
			if info.IsWrite != tt.isWrite {
				t.Errorf("IsWrite: got %v, want %v", info.IsWrite, tt.isWrite)
			}
		})
	}
}

func TestLookupFuncCode_UnknownCode(t *testing.T) {
	info := eventstore.LookupFuncCode(99)

	if info.Code != 99 {
		t.Errorf("Code: got %d, want 99", info.Code)
	}
	if info.Category != "unknown" {
		t.Errorf("Category: got %q, want %q", info.Category, "unknown")
	}
	if info.IsWrite {
		t.Error("IsWrite: unknown codes must not be flagged as write")
	}
	if !strings.Contains(info.Name, "99") {
		t.Errorf("Name should contain the FC number; got %q", info.Name)
	}
}

func TestLookupFuncCode_WriteFCsAreIsWrite(t *testing.T) {
	writeFCs := []uint8{5, 6, 15, 16}
	for _, fc := range writeFCs {
		info := eventstore.LookupFuncCode(fc)
		if !info.IsWrite {
			t.Errorf("FC %d should have IsWrite=true", fc)
		}
		if info.Category != "write" {
			t.Errorf("FC %d category: got %q, want %q", fc, info.Category, "write")
		}
	}
}

func TestLookupFuncCode_ReadFCsAreNotIsWrite(t *testing.T) {
	readFCs := []uint8{1, 2, 3, 4}
	for _, fc := range readFCs {
		info := eventstore.LookupFuncCode(fc)
		if info.IsWrite {
			t.Errorf("FC %d should have IsWrite=false", fc)
		}
		if info.Category != "read" {
			t.Errorf("FC %d category: got %q, want %q", fc, info.Category, "read")
		}
	}
}

func TestLookupFuncCode_FC43IsDiagnostic(t *testing.T) {
	info := eventstore.LookupFuncCode(43)
	if info.Category != "diagnostic" {
		t.Errorf("FC 43 category: got %q, want %q", info.Category, "diagnostic")
	}
	if info.IsWrite {
		t.Error("FC 43 IsWrite: should be false for standard Device Identification sub-function")
	}
	// Verify the name matches the OT-REVIEW corrected naming.
	if info.Name != "MEI Transport (Device Identification)" {
		t.Errorf("FC 43 Name: got %q, want %q",
			info.Name, "MEI Transport (Device Identification)")
	}
}

func TestLookupExceptionCode_KnownCodes(t *testing.T) {
	tests := []struct {
		code    uint8
		name    string
		hasDesc bool
	}{
		{1, "Illegal Function", true},
		{2, "Illegal Data Address", true},
		{3, "Illegal Data Value", true},
		{4, "Slave Device Failure", true},
		{5, "Acknowledge", true},
		{6, "Slave Device Busy", true},
		{8, "Memory Parity Error", true},
		{11, "Gateway Path Unavailable", true},
		{12, "Gateway Target Device Failed to Respond", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := eventstore.LookupExceptionCode(tt.code)
			if info.Code != tt.code {
				t.Errorf("Code: got %d, want %d", info.Code, tt.code)
			}
			if info.Name != tt.name {
				t.Errorf("Name: got %q, want %q", info.Name, tt.name)
			}
			if tt.hasDesc && info.Description == "" {
				t.Error("Description should not be empty for a known exception code")
			}
		})
	}
}

func TestLookupExceptionCode_UnknownCode(t *testing.T) {
	info := eventstore.LookupExceptionCode(99)
	if info.Code != 99 {
		t.Errorf("Code: got %d, want 99", info.Code)
	}
	if info.Name != "Unknown Exception" {
		t.Errorf("Name: got %q, want %q", info.Name, "Unknown Exception")
	}
	if info.Description == "" {
		t.Error("Description should not be empty even for unknown codes")
	}
}

// TestLookupExceptionCode_02IsIllegalDataAddress confirms exception 02 is named
// correctly, since it is the most common exception seen in OT environments and
// the dashboard uses the canonical name for trainee education.
func TestLookupExceptionCode_02IsIllegalDataAddress(t *testing.T) {
	info := eventstore.LookupExceptionCode(2)
	if info.Name != "Illegal Data Address" {
		t.Errorf("Exception 02 Name: got %q, want %q", info.Name, "Illegal Data Address")
	}
}

// TestLookupExceptionCode_04IsSlaveDeviceFailure confirms exception 04 is named
// correctly; Slave Device Failure is critical for uptime monitoring and alert
// rules trigger on this code.
func TestLookupExceptionCode_04IsSlaveDeviceFailure(t *testing.T) {
	info := eventstore.LookupExceptionCode(4)
	if info.Name != "Slave Device Failure" {
		t.Errorf("Exception 04 Name: got %q, want %q", info.Name, "Slave Device Failure")
	}
}
