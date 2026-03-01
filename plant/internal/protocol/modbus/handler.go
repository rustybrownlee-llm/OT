package modbus

import (
	"math/rand/v2"
	"time"

	mblib "github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

// Handler implements modbus.RequestHandler for a single device placement.
// For direct Ethernet PLCs, one Handler serves one listener.
// For the gateway, one Handler routes to multiple RegisterStores by unit ID.
//
// PROTOTYPE-DEBT: [td-handler-007] Non-gateway handlers respond to any unit ID per
// CompactLogix behavior. Should optionally restrict to a configured ID for stricter
// simulation of other vendor devices.
// TODO-FUTURE: Add per-handler unit ID restriction when communication patterns are implemented.
type Handler struct {
	stores    map[uint8]*RegisterStore // unitID -> store (gateway: multiple; non-gateway: typically one)
	delays    map[uint8]time.Duration  // unitID -> total response delay
	jitters   map[uint8]time.Duration  // unitID -> jitter range
	isGateway bool
	logger    logging.Logger
}

// NewHandler creates a Handler with the given stores, delays, jitters, and gateway flag.
// For non-gateway handlers, a single store is registered under unit ID 0 as a sentinel --
// the handler responds to any unit ID using that store.
func NewHandler(
	stores map[uint8]*RegisterStore,
	delays map[uint8]time.Duration,
	jitters map[uint8]time.Duration,
	isGateway bool,
	logger logging.Logger,
) *Handler {
	return &Handler{
		stores:    stores,
		delays:    delays,
		jitters:   jitters,
		isGateway: isGateway,
		logger:    logger,
	}
}

// HandleHoldingRegisters dispatches FC03, FC06, FC10 requests.
// Applies per-device response delay with jitter, then delegates to the register store.
func (h *Handler) HandleHoldingRegisters(req *mblib.HoldingRegistersRequest) ([]uint16, error) {
	store, err := h.resolveStore(req.UnitId)
	if err != nil {
		h.logger.Warn("holding register request rejected",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"error", err,
		)
		return nil, err
	}

	h.applyDelay(req.UnitId)

	if req.IsWrite {
		if err := store.WriteHolding(req.Addr, req.Args); err != nil {
			h.logger.Warn("holding register write rejected",
				"unit_id", req.UnitId,
				"addr", req.Addr,
				"error", err,
			)
			return nil, err
		}
		return nil, nil
	}

	vals, err := store.ReadHolding(req.Addr, req.Quantity)
	if err != nil {
		h.logger.Warn("holding register read failed",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"qty", req.Quantity,
			"error", err,
		)
	}
	return vals, err
}

// HandleCoils dispatches FC01, FC05, FC0F requests.
func (h *Handler) HandleCoils(req *mblib.CoilsRequest) ([]bool, error) {
	store, err := h.resolveStore(req.UnitId)
	if err != nil {
		h.logger.Warn("coil request rejected",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"error", err,
		)
		return nil, err
	}

	h.applyDelay(req.UnitId)

	if req.IsWrite {
		if err := store.WriteCoils(req.Addr, req.Args); err != nil {
			h.logger.Warn("coil write rejected",
				"unit_id", req.UnitId,
				"addr", req.Addr,
				"error", err,
			)
			return nil, err
		}
		return nil, nil
	}

	vals, err := store.ReadCoils(req.Addr, req.Quantity)
	if err != nil {
		h.logger.Warn("coil read failed",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"qty", req.Quantity,
			"error", err,
		)
	}
	return vals, err
}

// HandleInputRegisters dispatches FC04 (read-only).
func (h *Handler) HandleInputRegisters(req *mblib.InputRegistersRequest) ([]uint16, error) {
	store, err := h.resolveStore(req.UnitId)
	if err != nil {
		h.logger.Warn("input register request rejected",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"error", err,
		)
		return nil, err
	}

	h.applyDelay(req.UnitId)

	vals, err := store.ReadInputRegisters(req.Addr, req.Quantity)
	if err != nil {
		h.logger.Warn("input register read failed",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"qty", req.Quantity,
			"error", err,
		)
	}
	return vals, err
}

// HandleDiscreteInputs dispatches FC02 (read-only).
func (h *Handler) HandleDiscreteInputs(req *mblib.DiscreteInputsRequest) ([]bool, error) {
	store, err := h.resolveStore(req.UnitId)
	if err != nil {
		h.logger.Warn("discrete input request rejected",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"error", err,
		)
		return nil, err
	}

	h.applyDelay(req.UnitId)

	vals, err := store.ReadDiscreteInputs(req.Addr, req.Quantity)
	if err != nil {
		h.logger.Warn("discrete input read failed",
			"unit_id", req.UnitId,
			"addr", req.Addr,
			"qty", req.Quantity,
			"error", err,
		)
	}
	return vals, err
}

// resolveStore returns the RegisterStore for the given unit ID.
// Gateway: requires an exact unit ID match; returns ErrGWPathUnavailable for unknown IDs.
// Non-gateway: returns the sentinel store (key 0), which responds to any unit ID.
func (h *Handler) resolveStore(unitID uint8) (*RegisterStore, error) {
	if h.isGateway {
		store, ok := h.stores[unitID]
		if !ok {
			return nil, mblib.ErrGWPathUnavailable
		}
		return store, nil
	}

	// Non-gateway: stored under sentinel key 0, responds to any unit ID.
	store, ok := h.stores[0]
	if !ok {
		return nil, mblib.ErrGWPathUnavailable
	}
	return store, nil
}

// applyDelay sleeps for the configured delay plus a random jitter in [-jitter, +jitter].
// The library spawns a goroutine per connection, so sleeping does not block other clients.
// For non-gateway handlers, the delay is stored at sentinel key 0 regardless of unit ID.
func (h *Handler) applyDelay(unitID uint8) {
	lookupKey := unitID
	if !h.isGateway {
		lookupKey = 0
	}

	delay, ok := h.delays[lookupKey]
	if !ok || delay <= 0 {
		return
	}

	jitter, jok := h.jitters[lookupKey]
	if jok && jitter > 0 {
		// Apply jitter in [-jitter, +jitter] range.
		jitterNs := int64(jitter)
		offset := rand.Int64N(2*jitterNs) - jitterNs //nolint:gosec // jitter is not security-sensitive
		delay += time.Duration(offset)
		if delay < 0 {
			delay = 0
		}
	}

	time.Sleep(delay)
}
