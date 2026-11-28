package dualshock4

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	inputTransitionCapacity   = 64
	maximumInputTransitionAge = 50 * time.Millisecond
)

type inputTransition struct {
	state InputState
	at    time.Time
}

// The HID IN pipe consumes each button and D-pad boundary in order. Motion
// and axes can use the newest sample once those boundaries have been read.
type dualShock4InputQueue struct {
	mu      sync.Mutex
	ready   chan struct{}
	entries [inputTransitionCapacity]inputTransition
	head    int
	count   int
	latest  InputState
	pending bool
	owner   *dualShock4SerialReservation
}

func (q *dualShock4InputQueue) publish(state InputState, now time.Time, owner *dualShock4SerialReservation) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if owner != nil && owner != q.owner {
		return
	}
	if state.Buttons != q.latest.Buttons || state.DPad != q.latest.DPad {
		if q.count == len(q.entries) {
			q.head, q.count = 0, 0
		}
		q.entries[(q.head+q.count)%len(q.entries)] = inputTransition{state, now}
		q.count++
		q.pending = false
	} else {
		q.pending = true
	}
	q.latest = state
	q.signal()
}

func (q *dualShock4InputQueue) snapshot() InputState {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.latest
}

// Publication and reset use the same lock so an old stream cannot insert
// a pre-recovery press into the new stream's input queue.
func (q *dualShock4InputQueue) beginStream(owner *dualShock4SerialReservation) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.owner = owner
	q.resetLocked()
}

func (q *dualShock4InputQueue) endStream(owner *dualShock4SerialReservation) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.owner == owner {
		q.owner = nil
		q.resetLocked()
	}
}

func (q *dualShock4InputQueue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.resetLocked()
}

func (q *dualShock4InputQueue) resetLocked() {
	q.head, q.count = 0, 0
	q.latest = *NewInputState()
	q.pending = true
	q.signal()
}

func (q *dualShock4InputQueue) signal() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *dualShock4InputQueue) take(now time.Time, repeat bool) (InputState, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count != 0 {
		next := q.entries[q.head]
		if now.Sub(next.at) < maximumInputTransitionAge {
			q.head = (q.head + 1) % len(q.entries)
			q.count--
			if q.count != 0 || q.pending {
				q.signal()
			}
			return next.state, true
		}

		// After a paused host, resume with the current state without
		// replaying expired presses or separating a press from its release.
		q.head, q.count = 0, 0
		q.pending = true
	}
	if q.pending || repeat {
		q.pending = false
		return q.latest, true
	}
	return InputState{}, false
}

func (q *dualShock4InputQueue) read(ctx context.Context) (InputState, bool) {
	for {
		err := ctx.Err()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return InputState{}, false
		}
		if state, ok := q.take(time.Now(), err != nil); ok {
			return state, true
		}
		select {
		case <-ctx.Done():
		case <-q.ready:
		}
	}
}
