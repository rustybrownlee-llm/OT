// Event-collection methods for the Modbus poller (SOW-028.0).
// Implements per-chunk TransactionEvent creation for FC 3 (Read Holding Registers)
// and FC 1 (Read Coils), along with the helper functions that support event
// field population: exception code extraction and monitor address resolution.
package poller

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

// pollAsset reads all holding registers and coils for a single online asset.
// Reads are chunked at 125 registers per request (Modbus spec maximum).
// Used when no EventHook is registered. See pollAssetWithEvents for the
// event-collecting code path.
func (p *Poller) pollAsset(ctx context.Context, a *inventory.Asset) error {
	client, err := p.pool.get(a.Endpoint)
	if err != nil {
		return fmt.Errorf("get client: %w", err)
	}

	if err := client.SetUnitId(a.UnitID); err != nil {
		return fmt.Errorf("set unit ID %d: %w", a.UnitID, err)
	}

	baseAddr := uint16(0)
	if a.Addressing == "one-based" {
		baseAddr = 1
	}

	holding, err := readHoldingChunked(client, baseAddr, a.HoldingRegCount)
	if err != nil {
		return fmt.Errorf("read holding registers: %w", err)
	}

	coils, err := readCoilsChunked(client, baseAddr, a.CoilCount)
	if err != nil {
		return fmt.Errorf("read coils: %w", err)
	}

	p.inv.UpdateRegisters(a.ID, holding, coils, time.Now())
	p.inv.SetStatus(a.ID, inventory.StatusOnline, time.Now())
	return nil
}

// readHoldingChunked reads count holding registers starting at baseAddr,
// splitting into 125-register chunks as required by the Modbus specification.
func readHoldingChunked(client *modbus.ModbusClient, baseAddr uint16, count int) ([]uint16, error) {
	if count == 0 {
		return nil, nil
	}

	result := make([]uint16, 0, count)
	addr := baseAddr

	for remaining := count; remaining > 0; {
		qty := remaining
		if qty > modbusMaxRead {
			qty = modbusMaxRead
		}
		vals, err := client.ReadRegisters(addr, uint16(qty), modbus.HOLDING_REGISTER)
		if err != nil {
			return nil, fmt.Errorf("read at addr %d qty %d: %w", addr, qty, err)
		}
		result = append(result, vals...)
		addr += uint16(qty)
		remaining -= qty
	}

	return result, nil
}

// readCoilsChunked reads count coils starting at baseAddr in 125-coil chunks.
func readCoilsChunked(client *modbus.ModbusClient, baseAddr uint16, count int) ([]bool, error) {
	if count == 0 {
		return nil, nil
	}

	result := make([]bool, 0, count)
	addr := baseAddr

	for remaining := count; remaining > 0; {
		qty := remaining
		if qty > modbusMaxRead {
			qty = modbusMaxRead
		}
		vals, err := client.ReadCoils(addr, uint16(qty))
		if err != nil {
			return nil, fmt.Errorf("read coils at addr %d qty %d: %w", addr, qty, err)
		}
		result = append(result, vals...)
		addr += uint16(qty)
		remaining -= qty
	}

	return result, nil
}

// pollAssetWithEvents polls a single asset, creating one TransactionEvent per
// chunked Modbus read request. Returns all collected events and any error.
// On error, the failed transaction event is included in the returned events slice.
//
// [OT-REVIEW] FR-6: An offline device produces no transactions. Events are only
// created for actual Modbus wire transactions; transport failures terminate collection.
func (p *Poller) pollAssetWithEvents(a *inventory.Asset) ([]*eventstore.TransactionEvent, error) {
	client, err := p.pool.get(a.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if err := client.SetUnitId(a.UnitID); err != nil {
		return nil, fmt.Errorf("set unit ID %d: %w", a.UnitID, err)
	}

	baseAddr := uint16(0)
	if a.Addressing == "one-based" {
		baseAddr = 1
	}

	var events []*eventstore.TransactionEvent

	holdingEvents, holding, err := p.readHoldingWithEvents(client, baseAddr, a.HoldingRegCount, a)
	events = append(events, holdingEvents...)
	if err != nil {
		return events, fmt.Errorf("read holding registers: %w", err)
	}

	coilEvents, coils, err := p.readCoilsWithEvents(client, baseAddr, a.CoilCount, a)
	events = append(events, coilEvents...)
	if err != nil {
		return events, fmt.Errorf("read coils: %w", err)
	}

	p.inv.UpdateRegisters(a.ID, holding, coils, time.Now())
	p.inv.SetStatus(a.ID, inventory.StatusOnline, time.Now())
	return events, nil
}

// readHoldingWithEvents reads holding registers in 125-register chunks, creating
// one TransactionEvent per chunk request (FC 3: Read Holding Registers).
// On read failure, the failed event is appended and the error is returned.
func (p *Poller) readHoldingWithEvents(
	client *modbus.ModbusClient,
	baseAddr uint16,
	count int,
	a *inventory.Asset,
) ([]*eventstore.TransactionEvent, []uint16, error) {
	if count == 0 {
		return nil, nil, nil
	}

	var events []*eventstore.TransactionEvent
	result := make([]uint16, 0, count)
	addr := baseAddr

	for remaining := count; remaining > 0; {
		qty := remaining
		if qty > modbusMaxRead {
			qty = modbusMaxRead
		}

		txStart := time.Now()
		vals, err := client.ReadRegisters(addr, uint16(qty), modbus.HOLDING_REGISTER)
		txEnd := time.Now()

		evt := eventstore.NewEvent(
			txEnd,
			p.srcAddr,
			a.Endpoint,
			a.UnitID,
			3, // FC 3: Read Holding Registers
			addr,
			uint16(qty),
			false,
			err == nil,
			extractExceptionCode(err),
			txEnd.Sub(txStart).Microseconds(),
			a.ID,
			a.ConfigDesc,
			a.EnvironmentName,
			nil,
		)
		events = append(events, evt)

		if err != nil {
			return events, nil, fmt.Errorf("read at addr %d qty %d: %w", addr, qty, err)
		}

		result = append(result, vals...)
		addr += uint16(qty)
		remaining -= qty
	}

	return events, result, nil
}

// readCoilsWithEvents reads coils in 2000-coil chunks, creating one
// TransactionEvent per chunk request (FC 1: Read Coils).
//
// [OT-REVIEW] FC 1/2 coil reads allow up to 2000 coils per request (250 bytes =
// 2000 bits), not 125. Using modbusMaxCoilRead teaches the correct protocol limit.
// On read failure, the failed event is appended and the error is returned.
func (p *Poller) readCoilsWithEvents(
	client *modbus.ModbusClient,
	baseAddr uint16,
	count int,
	a *inventory.Asset,
) ([]*eventstore.TransactionEvent, []bool, error) {
	if count == 0 {
		return nil, nil, nil
	}

	var events []*eventstore.TransactionEvent
	result := make([]bool, 0, count)
	addr := baseAddr

	for remaining := count; remaining > 0; {
		qty := remaining
		if qty > modbusMaxCoilRead {
			qty = modbusMaxCoilRead
		}

		txStart := time.Now()
		vals, err := client.ReadCoils(addr, uint16(qty))
		txEnd := time.Now()

		evt := eventstore.NewEvent(
			txEnd,
			p.srcAddr,
			a.Endpoint,
			a.UnitID,
			1, // FC 1: Read Coils
			addr,
			uint16(qty),
			false,
			err == nil,
			extractExceptionCode(err),
			txEnd.Sub(txStart).Microseconds(),
			a.ID,
			a.ConfigDesc,
			a.EnvironmentName,
			nil,
		)
		events = append(events, evt)

		if err != nil {
			return events, nil, fmt.Errorf("read coils at addr %d qty %d: %w", addr, qty, err)
		}

		result = append(result, vals...)
		addr += uint16(qty)
		remaining -= qty
	}

	return events, result, nil
}

// extractExceptionCode attempts to extract a Modbus exception code from an
// error returned by the simonvetter/modbus library. Returns 0 for nil errors
// and for transport-level errors (TCP timeout, connection refused) that are
// not Modbus protocol exceptions.
//
// [OT-REVIEW] Exception codes reveal device health and potential probing activity.
// Exception 01 (Illegal Function) and 02 (Illegal Data Address) distinguish
// misconfigured SCADA polls from attacker probing. Exception 04 (Server Device
// Failure) signals device health problems. Recording ExceptionCode=0 for all
// failures teaches trainees that exception codes are meaningless, which is incorrect.
func extractExceptionCode(err error) uint8 {
	if err == nil {
		return 0
	}

	switch err {
	case modbus.ErrIllegalFunction:
		return 0x01
	case modbus.ErrIllegalDataAddress:
		return 0x02
	case modbus.ErrIllegalDataValue:
		return 0x03
	case modbus.ErrServerDeviceFailure:
		return 0x04
	case modbus.ErrAcknowledge:
		return 0x05
	case modbus.ErrServerDeviceBusy:
		return 0x06
	case modbus.ErrMemoryParityError:
		return 0x08
	case modbus.ErrGWPathUnavailable:
		return 0x0a
	case modbus.ErrGWTargetFailedToRespond:
		return 0x0b
	default:
		// Transport-level error (TCP timeout, connection refused, bad CRC, etc.)
		// These are not Modbus protocol exceptions; return 0.
		return 0
	}
}

// resolveMonitorAddr extracts the monitor's IP from the first configured
// environment's address field and appends port "0" as a placeholder.
//
// [OT-REVIEW] Using the environment address (e.g., "10.10.30.100") rather than
// cfg.APIAddr avoids placing the HTTP port (8091) in Modbus transaction records,
// which would confuse trainees comparing event logs with packet captures.
// Port 0 is honest: the simonvetter/modbus client does not expose the ephemeral
// TCP source port, so we use 0 to indicate the value is unknown.
func resolveMonitorAddr(cfg *config.Config) string {
	if len(cfg.Environments) == 0 {
		return "0.0.0.0:0"
	}
	host := cfg.Environments[0].Address
	if host == "" {
		return "0.0.0.0:0"
	}
	// Strip any existing port if the address was provided with one.
	if h, _, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, "0")
	}
	return net.JoinHostPort(host, "0")
}
