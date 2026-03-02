// Package timeseries provides fixed-size circular buffers for timestamped
// register and coil samples. Buffers allocate all memory at construction;
// subsequent pushes are O(1) with no heap allocation.
package timeseries

import "time"

// RegisterSample is a timestamped holding register value.
type RegisterSample struct {
	Timestamp time.Time
	Value     uint16
}

// CoilSample is a timestamped coil state.
type CoilSample struct {
	Timestamp time.Time
	Value     bool
}

// RingBuffer is a fixed-capacity circular buffer of RegisterSamples.
// When the buffer is full, the oldest entry is overwritten.
// All methods are NOT goroutine-safe; the caller is responsible for locking.
type RingBuffer struct {
	data  []RegisterSample
	size  int // capacity
	head  int // next write position (index of oldest entry when full)
	count int // number of valid entries [0, size]
}

// NewRingBuffer allocates a RingBuffer with the given capacity.
// Panics if size is less than 1.
func NewRingBuffer(size int) *RingBuffer {
	if size < 1 {
		panic("ringbuffer: size must be >= 1")
	}
	return &RingBuffer{
		data: make([]RegisterSample, size),
		size: size,
	}
}

// Push adds a sample to the buffer. If the buffer is full, the oldest
// entry is overwritten. O(1).
func (rb *RingBuffer) Push(s RegisterSample) {
	rb.data[rb.head] = s
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Len returns the number of valid entries in the buffer.
func (rb *RingBuffer) Len() int {
	return rb.count
}

// Latest returns the most recently pushed sample and true.
// Returns the zero value and false if the buffer is empty.
func (rb *RingBuffer) Latest() (RegisterSample, bool) {
	if rb.count == 0 {
		return RegisterSample{}, false
	}
	idx := (rb.head - 1 + rb.size) % rb.size
	return rb.data[idx], true
}

// Iterate calls fn on each valid entry from oldest to newest. O(n).
func (rb *RingBuffer) Iterate(fn func(RegisterSample)) {
	if rb.count == 0 {
		return
	}
	// When the buffer is full, head points to the oldest entry.
	// When not full, oldest is at index 0 and head == count.
	start := 0
	if rb.count == rb.size {
		start = rb.head
	}
	for i := 0; i < rb.count; i++ {
		fn(rb.data[(start+i)%rb.size])
	}
}

// CoilRingBuffer is a fixed-capacity circular buffer of CoilSamples.
// When the buffer is full, the oldest entry is overwritten.
// All methods are NOT goroutine-safe; the caller is responsible for locking.
type CoilRingBuffer struct {
	data  []CoilSample
	size  int
	head  int
	count int
}

// NewCoilRingBuffer allocates a CoilRingBuffer with the given capacity.
// Panics if size is less than 1.
func NewCoilRingBuffer(size int) *CoilRingBuffer {
	if size < 1 {
		panic("coilringbuffer: size must be >= 1")
	}
	return &CoilRingBuffer{
		data: make([]CoilSample, size),
		size: size,
	}
}

// Push adds a sample to the buffer. If the buffer is full, the oldest
// entry is overwritten. O(1).
func (rb *CoilRingBuffer) Push(s CoilSample) {
	rb.data[rb.head] = s
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Len returns the number of valid entries in the buffer.
func (rb *CoilRingBuffer) Len() int {
	return rb.count
}

// Latest returns the most recently pushed sample and true.
// Returns the zero value and false if the buffer is empty.
func (rb *CoilRingBuffer) Latest() (CoilSample, bool) {
	if rb.count == 0 {
		return CoilSample{}, false
	}
	idx := (rb.head - 1 + rb.size) % rb.size
	return rb.data[idx], true
}
