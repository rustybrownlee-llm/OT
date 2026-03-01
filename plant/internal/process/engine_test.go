package process

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

// countingModel is a test ProcessModel that records how many times Tick was called.
type countingModel struct {
	name  string
	ticks atomic.Int64
}

func (m *countingModel) Name() string { return m.name }
func (m *countingModel) Tick()        { m.ticks.Add(1) }

// TestEngine_TicksModels verifies that the engine calls Tick on each registered model.
// Uses a very short ticker interval (10ms) to avoid slow test execution.
func TestEngine_TicksModels(t *testing.T) {
	logger := logging.NewTestLogger()
	engine := NewSimulationEngine(logger)

	m1 := &countingModel{name: "model-1"}
	m2 := &countingModel{name: "model-2"}
	engine.AddModel(m1)
	engine.AddModel(m2)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Run in background. The test ticker in the engine is 1s, which is too long for unit tests.
	// We test tick() directly to avoid slow tests.
	engine.tick()
	engine.tick()
	engine.tick()

	if m1.ticks.Load() != 3 {
		t.Errorf("model-1 ticks: got %d, want 3", m1.ticks.Load())
	}
	if m2.ticks.Load() != 3 {
		t.Errorf("model-2 ticks: got %d, want 3", m2.ticks.Load())
	}
	_ = ctx // suppress unused warning
}

// TestEngine_CleanShutdown verifies that Run returns promptly after context cancellation
// with no goroutine leak (NFR-2).
func TestEngine_CleanShutdown(t *testing.T) {
	logger := logging.NewTestLogger()
	engine := NewSimulationEngine(logger)

	m := &countingModel{name: "test-model"}
	engine.AddModel(m)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx)
	}()

	// Give the goroutine time to start, then cancel.
	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Engine returned cleanly.
	case <-time.After(2 * time.Second):
		t.Error("engine did not stop within 2 seconds of context cancellation (goroutine leak?)")
	}
}

// TestEngine_AddModel_NoPanic verifies that AddModel with no prior Run does not panic.
func TestEngine_AddModel_NoPanic(t *testing.T) {
	logger := logging.NewTestLogger()
	engine := NewSimulationEngine(logger)
	engine.AddModel(&countingModel{name: "early-model"})

	if len(engine.models) != 1 {
		t.Errorf("expected 1 model, got %d", len(engine.models))
	}
}
