package process

import (
	"context"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

// SimulationEngine runs all process models on a 1-second tick.
//
// The 1-second tick rate is a simulator throughput constraint, not a model of PLC scan time.
// Real PLCs scan far faster: CompactLogix 5-20ms, SLC-500 10-50ms, Modicon 984 20-100ms.
// This tick rate controls how often the simulator writes updated register values. It does
// NOT represent the actual scan cycle of the physical devices being modeled.
// Trainees should understand that Modbus polls at human-observable rates (hundreds of ms to
// seconds) and that the simulator produces plausible values at each poll interval.
//
// PROTOTYPE-DEBT: [td-engine-014] Tick rate hardcoded to 1 second.
// TODO-FUTURE: Make configurable for performance profiling (SOW amendment required).
type SimulationEngine struct {
	models []ProcessModel
	logger logging.Logger
}

// NewSimulationEngine creates an engine with no models registered.
// Models are added via AddModel before calling Run.
func NewSimulationEngine(logger logging.Logger) *SimulationEngine {
	return &SimulationEngine{logger: logger}
}

// AddModel registers a ProcessModel with the engine.
// Models are ticked in registration order on each interval.
func (e *SimulationEngine) AddModel(m ProcessModel) {
	e.models = append(e.models, m)
	e.logger.Debug("simulation model registered", "model", m.Name())
}

// Run starts the 1-second ticker and dispatches Tick() to all registered models.
// It blocks until ctx is cancelled, then returns immediately with no goroutine leak.
// Each tick completes all model updates before sleeping; updates are synchronous within
// a tick to avoid cross-model ordering issues.
func (e *SimulationEngine) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	e.logger.Info("simulation engine started", "models", len(e.models))

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("simulation engine stopped")
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

// tick executes one simulation step across all registered models.
// NFR-1: must complete in under 10ms.
func (e *SimulationEngine) tick() {
	for _, m := range e.models {
		m.Tick()
	}
}
