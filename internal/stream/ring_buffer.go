package stream

import (
	"sync"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
)

// RingBuffer is a thread-safe circular buffer for storing recent ticks and transactions
type RingBuffer struct {
	mu sync.RWMutex

	// Circular buffers
	ticks        []*domain.Tick
	transactions []*domain.Transaction

	// Buffer configuration
	maxTicks        int
	maxTransactions int

	// Indices for circular buffer (points to next write position)
	tickIndex int
	txIndex   int

	// Actual count (may be less than max during initial fill)
	tickCount int
	txCount   int

	// Broadcast channels for notifying listeners of updates
	updateChan chan struct{}
	listeners  []chan struct{}
}

// NewRingBuffer creates a new ring buffer with specified capacity
func NewRingBuffer(maxTicks, maxTransactions int) *RingBuffer {
	return &RingBuffer{
		ticks:           make([]*domain.Tick, maxTicks),
		transactions:    make([]*domain.Transaction, maxTransactions),
		maxTicks:        maxTicks,
		maxTransactions: maxTransactions,
		updateChan:      make(chan struct{}, 100),
		listeners:       make([]chan struct{}, 0),
	}
}

// AddTick adds a tick to the ring buffer and extracts its transactions
func (rb *RingBuffer) AddTick(tick *domain.Tick) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Add tick to circular buffer
	rb.ticks[rb.tickIndex] = tick
	rb.tickIndex = (rb.tickIndex + 1) % rb.maxTicks
	if rb.tickCount < rb.maxTicks {
		rb.tickCount++
	}

	// Extract and add transactions from this tick
	for i := range tick.Transactions {
		rb.transactions[rb.txIndex] = &tick.Transactions[i]
		rb.txIndex = (rb.txIndex + 1) % rb.maxTransactions
		if rb.txCount < rb.maxTransactions {
			rb.txCount++
		}
	}

	// Notify listeners (non-blocking)
	rb.notifyListeners()
}

// GetSnapshot returns the current snapshot of recent ticks and transactions
func (rb *RingBuffer) GetSnapshot() (ticks []*domain.Tick, transactions []*domain.Transaction) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	// Copy ticks in chronological order
	ticks = make([]*domain.Tick, 0, rb.tickCount)
	if rb.tickCount < rb.maxTicks {
		// Buffer not full yet, copy from start
		for i := 0; i < rb.tickCount; i++ {
			if rb.ticks[i] != nil {
				ticks = append(ticks, rb.ticks[i])
			}
		}
	} else {
		// Buffer is full, copy from current position onwards (oldest to newest)
		for i := 0; i < rb.maxTicks; i++ {
			idx := (rb.tickIndex + i) % rb.maxTicks
			if rb.ticks[idx] != nil {
				ticks = append(ticks, rb.ticks[idx])
			}
		}
	}

	// Copy transactions in chronological order
	transactions = make([]*domain.Transaction, 0, rb.txCount)
	if rb.txCount < rb.maxTransactions {
		// Buffer not full yet, copy from start
		for i := 0; i < rb.txCount; i++ {
			if rb.transactions[i] != nil {
				transactions = append(transactions, rb.transactions[i])
			}
		}
	} else {
		// Buffer is full, copy from current position onwards (oldest to newest)
		for i := 0; i < rb.maxTransactions; i++ {
			idx := (rb.txIndex + i) % rb.maxTransactions
			if rb.transactions[idx] != nil {
				transactions = append(transactions, rb.transactions[idx])
			}
		}
	}

	return ticks, transactions
}

// Subscribe returns a channel that receives notifications when the buffer is updated
func (rb *RingBuffer) Subscribe() <-chan struct{} {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	ch := make(chan struct{}, 10)
	rb.listeners = append(rb.listeners, ch)
	return ch
}

// Unsubscribe removes a listener channel
func (rb *RingBuffer) Unsubscribe(ch <-chan struct{}) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i, listener := range rb.listeners {
		if listener == ch {
			// Close the channel and remove from slice
			close(listener)
			rb.listeners = append(rb.listeners[:i], rb.listeners[i+1:]...)
			break
		}
	}
}

// notifyListeners notifies all subscribers (must be called with lock held)
func (rb *RingBuffer) notifyListeners() {
	// Non-blocking send to all listeners
	for _, listener := range rb.listeners {
		select {
		case listener <- struct{}{}:
		default:
			// Skip if channel is full (slow consumer)
		}
	}
}

// Stats returns statistics about the ring buffer
func (rb *RingBuffer) Stats() (tickCount, txCount int) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.tickCount, rb.txCount
}
