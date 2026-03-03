package syslog_test

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	internalsyslog "github.com/rustybrownlee/ot-simulator/monitoring/internal/syslog"
)

// makeSyslogEvent returns a minimal TransactionEvent for emitter tests.
// Uses FC 3 (Read Holding Registers) as a representative read operation.
func makeSyslogEvent() *eventstore.TransactionEvent {
	return &eventstore.TransactionEvent{
		ID:           "emitter-test-uuid",
		Timestamp:    time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		SrcAddr:      "127.0.0.1:50000",
		DstAddr:      "127.0.0.1:5020",
		UnitID:       1,
		FunctionCode: 3,
		FunctionName: "Read Holding Registers",
		AddressStart: 0,
		AddressCount: 10,
		IsWrite:      false,
		Success:      true,
		DeviceID:     "127.0.0.1:5020:1",
		EnvID:        "test",
	}
}

// TestEmitter_UDP verifies that an event sent via UDP reaches a local listener.
// The listener is a standard UDP socket bound on a random OS-assigned port.
func TestEmitter_UDP(t *testing.T) {
	// Bind a UDP listener on a random port.
	lconn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer lconn.Close()

	addr := lconn.LocalAddr().String()

	cfg := config.SyslogConfig{
		Enabled:  true,
		Target:   addr,
		Protocol: "udp",
		Facility: "local0",
		Format:   "cef",
	}

	emitter, err := internalsyslog.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emitter.Close()

	ev := makeSyslogEvent()

	// Send in a separate goroutine to avoid blocking the test if the
	// UDP buffer is full. This is safe because UDP sends are non-blocking.
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- emitter.Send([]*eventstore.TransactionEvent{ev})
	}()

	// Read the datagram from the listener.
	buf := make([]byte, 4096)
	lconn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := lconn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read UDP datagram: %v", err)
	}

	received := string(buf[:n])
	if !strings.Contains(received, "CEF:0|OTSimulator") {
		t.Errorf("UDP message does not contain expected CEF header, got: %s", received)
	}
	if !strings.Contains(received, "Read Holding Registers") {
		t.Errorf("UDP message does not contain function name, got: %s", received)
	}

	if err := <-sendDone; err != nil {
		t.Errorf("Send returned error: %v", err)
	}
}

// TestEmitter_TCP verifies that an event sent via TCP reaches a local listener.
// Messages are newline-terminated per RFC 6587 non-transparent framing.
func TestEmitter_TCP(t *testing.T) {
	// Bind a TCP listener on a random port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen TCP: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Accept one connection in a goroutine, read the message, signal done.
	received := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			received <- ""
			return
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			received <- scanner.Text()
		} else {
			received <- ""
		}
	}()

	cfg := config.SyslogConfig{
		Enabled:  true,
		Target:   addr,
		Protocol: "tcp",
		Facility: "local0",
		Format:   "cef",
	}

	emitter, err := internalsyslog.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emitter.Close()

	ev := makeSyslogEvent()
	if err := emitter.Send([]*eventstore.TransactionEvent{ev}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msg := <-received
	if msg == "" {
		t.Fatal("TCP listener received empty message")
	}
	if !strings.Contains(msg, "CEF:0|OTSimulator") {
		t.Errorf("TCP message does not contain expected CEF header, got: %s", msg)
	}
	if !strings.Contains(msg, "Read Holding Registers") {
		t.Errorf("TCP message does not contain function name, got: %s", msg)
	}
}

// TestEmitter_New_Disabled verifies that calling New with Enabled=false returns
// an error and does not open a network connection.
func TestEmitter_New_Disabled(t *testing.T) {
	cfg := config.SyslogConfig{
		Enabled:  false,
		Target:   "localhost:514",
		Protocol: "udp",
		Facility: "local0",
		Format:   "cef",
	}

	emitter, err := internalsyslog.New(cfg)
	if err == nil {
		if emitter != nil {
			emitter.Close()
		}
		t.Fatal("expected error when Enabled=false, got nil")
	}
}

// TestEmitter_MultipleEvents verifies that Send correctly delivers multiple
// events in a single call. Uses UDP for simplicity.
func TestEmitter_MultipleEvents(t *testing.T) {
	lconn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer lconn.Close()

	addr := lconn.LocalAddr().String()

	cfg := config.SyslogConfig{
		Enabled:  true,
		Target:   addr,
		Protocol: "udp",
		Facility: "local0",
		Format:   "cef",
	}

	emitter, err := internalsyslog.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emitter.Close()

	// Build two events: a read and a write.
	ev1 := makeSyslogEvent()
	ev2 := makeSyslogEvent()
	ev2.FunctionCode = 6
	ev2.FunctionName = "Write Single Register"
	ev2.IsWrite = true

	received := make([]string, 0, 2)
	buf := make([]byte, 4096)
	lconn.SetReadDeadline(time.Now().Add(2 * time.Second))

	go func() {
		emitter.Send([]*eventstore.TransactionEvent{ev1, ev2})
	}()

	for i := 0; i < 2; i++ {
		n, _, err := lconn.ReadFrom(buf)
		if err != nil {
			break
		}
		received = append(received, string(buf[:n]))
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 UDP datagrams, got %d", len(received))
	}
}
