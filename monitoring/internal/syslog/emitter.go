package syslog

import (
	"fmt"
	"net"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// tcpWriteTimeout is the maximum time allowed for a single TCP write operation.
// Syslog emission must not block the polling cycle (NFR-1: max 10ms per batch),
// so TCP writes are bounded by this deadline.
//
// PROTOTYPE-DEBT: [td-syslog-090] No reconnection logic for TCP. Connection
// failure stops forwarding until the monitor restarts. Beta 0.7 should add
// TCP reconnection with exponential backoff.
const tcpWriteTimeout = 10 * time.Millisecond

// Emitter manages a syslog forwarding connection and sends Modbus transaction
// events as CEF-formatted syslog messages over UDP or TCP.
//
// PROTOTYPE-DEBT: [td-syslog-091] No buffering or retry. If Send fails, events
// are lost from syslog forwarding (but retained in the event store, which is
// the primary persistence layer). A ring buffer for retry is a future enhancement.
type Emitter struct {
	conn        net.Conn
	facilityNum int
	protocol    string
}

// New creates an Emitter from the provided SyslogConfig. It opens a UDP or TCP
// connection to cfg.Target and returns an Emitter ready to send events.
// Returns an error if the connection cannot be established.
// Callers should call Close() when the emitter is no longer needed.
func New(cfg config.SyslogConfig) (*Emitter, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("syslog is not enabled in config")
	}

	conn, err := net.Dial(cfg.Protocol, cfg.Target)
	if err != nil {
		return nil, fmt.Errorf("dial %s %s: %w", cfg.Protocol, cfg.Target, err)
	}

	return &Emitter{
		conn:        conn,
		facilityNum: FacilityCode(cfg.Facility),
		protocol:    cfg.Protocol,
	}, nil
}

// Send formats each event as a CEF syslog message and writes it to the
// underlying connection. For UDP, each message is one datagram. For TCP,
// messages are newline-terminated per RFC 6587 non-transparent framing.
//
// Errors from individual message sends are collected and returned as a combined
// error. The caller logs at Warn level; syslog emission errors must not stop
// the polling cycle or event store persistence.
func (e *Emitter) Send(events []*eventstore.TransactionEvent) error {
	var firstErr error
	for _, ev := range events {
		msg := FormatCEF(ev, e.facilityNum)
		if err := e.writeMessage(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Close closes the underlying network connection.
func (e *Emitter) Close() error {
	return e.conn.Close()
}

// writeMessage sends a single CEF message to the syslog target.
// UDP sends are fire-and-forget (no framing delimiter needed per RFC).
// TCP sends append a newline terminator per RFC 6587 non-transparent framing,
// and apply a write deadline to prevent blocking the polling cycle.
func (e *Emitter) writeMessage(msg string) error {
	if e.protocol == "tcp" {
		deadline := time.Now().Add(tcpWriteTimeout)
		if err := e.conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("set write deadline: %w", err)
		}
		_, err := fmt.Fprintf(e.conn, "%s\n", msg)
		return err
	}
	// UDP: send as a single datagram; no framing delimiter required.
	_, err := fmt.Fprint(e.conn, msg)
	return err
}
