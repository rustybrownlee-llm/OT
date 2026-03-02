package timeseries_test

import (
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/timeseries"
)

// makeRegSample creates a RegisterSample with the given value at the given
// offset seconds from the base time. Used for deterministic test ordering.
func makeRegSample(value uint16, offsetSec int) timeseries.RegisterSample {
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	return timeseries.RegisterSample{
		Timestamp: base.Add(time.Duration(offsetSec) * time.Second),
		Value:     value,
	}
}

func makeCoilSample(value bool, offsetSec int) timeseries.CoilSample {
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	return timeseries.CoilSample{
		Timestamp: base.Add(time.Duration(offsetSec) * time.Second),
		Value:     value,
	}
}

func TestRingBuffer_PushAndIterate_NItems(t *testing.T) {
	rb := timeseries.NewRingBuffer(5)
	for i := 0; i < 5; i++ {
		rb.Push(makeRegSample(uint16(i*10), i))
	}

	if rb.Len() != 5 {
		t.Fatalf("Len: got %d, want 5", rb.Len())
	}

	var collected []uint16
	rb.Iterate(func(s timeseries.RegisterSample) {
		collected = append(collected, s.Value)
	})

	if len(collected) != 5 {
		t.Fatalf("Iterate count: got %d, want 5", len(collected))
	}
	for i, v := range collected {
		if v != uint16(i*10) {
			t.Errorf("collected[%d]: got %d, want %d", i, v, i*10)
		}
	}
}

func TestRingBuffer_Overwrite_OldestEvicted(t *testing.T) {
	rb := timeseries.NewRingBuffer(3)
	rb.Push(makeRegSample(10, 0))
	rb.Push(makeRegSample(20, 1))
	rb.Push(makeRegSample(30, 2))
	rb.Push(makeRegSample(40, 3)) // evicts 10

	if rb.Len() != 3 {
		t.Fatalf("Len after wrap: got %d, want 3", rb.Len())
	}

	var vals []uint16
	rb.Iterate(func(s timeseries.RegisterSample) {
		vals = append(vals, s.Value)
	})

	// Oldest to newest after wrap: 20, 30, 40.
	expected := []uint16{20, 30, 40}
	for i, v := range vals {
		if v != expected[i] {
			t.Errorf("vals[%d]: got %d, want %d", i, v, expected[i])
		}
	}
}

func TestRingBuffer_Latest_ReturnsNewest(t *testing.T) {
	rb := timeseries.NewRingBuffer(5)
	rb.Push(makeRegSample(100, 0))
	rb.Push(makeRegSample(200, 1))
	rb.Push(makeRegSample(300, 2))

	s, ok := rb.Latest()
	if !ok {
		t.Fatal("Latest returned false on non-empty buffer")
	}
	if s.Value != 300 {
		t.Errorf("Latest value: got %d, want 300", s.Value)
	}
}

func TestRingBuffer_Len_BeforeAndAfterWrap(t *testing.T) {
	rb := timeseries.NewRingBuffer(4)

	if rb.Len() != 0 {
		t.Errorf("initial Len: got %d, want 0", rb.Len())
	}

	rb.Push(makeRegSample(1, 0))
	rb.Push(makeRegSample(2, 1))
	if rb.Len() != 2 {
		t.Errorf("Len after 2 pushes: got %d, want 2", rb.Len())
	}

	rb.Push(makeRegSample(3, 2))
	rb.Push(makeRegSample(4, 3))
	if rb.Len() != 4 {
		t.Errorf("Len at capacity: got %d, want 4", rb.Len())
	}

	rb.Push(makeRegSample(5, 4)) // wraps
	if rb.Len() != 4 {
		t.Errorf("Len after wrap: got %d, want 4", rb.Len())
	}
}

func TestRingBuffer_Latest_EmptyReturnsFalse(t *testing.T) {
	rb := timeseries.NewRingBuffer(5)
	_, ok := rb.Latest()
	if ok {
		t.Error("Latest on empty buffer: expected false, got true")
	}
}

func TestCoilRingBuffer_PushAndLatest(t *testing.T) {
	rb := timeseries.NewCoilRingBuffer(4)

	rb.Push(makeCoilSample(false, 0))
	rb.Push(makeCoilSample(true, 1))
	rb.Push(makeCoilSample(false, 2))

	if rb.Len() != 3 {
		t.Fatalf("Len: got %d, want 3", rb.Len())
	}

	s, ok := rb.Latest()
	if !ok {
		t.Fatal("Latest returned false on non-empty buffer")
	}
	if s.Value != false {
		t.Errorf("Latest value: got %v, want false", s.Value)
	}
}

func TestCoilRingBuffer_Overwrite(t *testing.T) {
	rb := timeseries.NewCoilRingBuffer(2)
	rb.Push(makeCoilSample(true, 0))
	rb.Push(makeCoilSample(false, 1))
	rb.Push(makeCoilSample(true, 2)) // evicts first

	if rb.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", rb.Len())
	}

	s, _ := rb.Latest()
	if s.Value != true {
		t.Errorf("Latest after wrap: got %v, want true", s.Value)
	}
}

func TestRingBuffer_Iterate_SingleElement(t *testing.T) {
	rb := timeseries.NewRingBuffer(5)
	rb.Push(makeRegSample(42, 0))

	var count int
	rb.Iterate(func(s timeseries.RegisterSample) {
		count++
		if s.Value != 42 {
			t.Errorf("Iterate value: got %d, want 42", s.Value)
		}
	})
	if count != 1 {
		t.Errorf("Iterate count: got %d, want 1", count)
	}
}
