package camera

import "time"

// frameTicker keeps (approx) a fixed frame rate without accumulating drift badly.
// Call tk.Wait() once per loop iteration.
type frameTicker struct {
	dur  time.Duration
	next time.Time
}

// newTicker creates a frameTicker for desired FPS. If fps <= 0, it returns a ticker that never waits.
func newTicker(fps int) *frameTicker {
	if fps <= 0 {
		return &frameTicker{dur: 0}
	}
	return &frameTicker{dur: time.Second / time.Duration(fps)}
}

// Wait sleeps until the scheduled next frame time.
// If we're behind schedule, it resets the schedule to now (drops delay rather than piling up).
func (t *frameTicker) Wait() {
	if t.dur <= 0 {
		return
	}
	now := time.Now()
	if t.next.IsZero() {
		t.next = now.Add(t.dur)
		return
	}
	if sleep := t.next.Sub(now); sleep > 0 {
		time.Sleep(sleep)
	}
	t.next = t.next.Add(t.dur)
	// If fell behind more than one interval, reset to now + dur
	if lag := time.Since(t.next); lag > t.dur {
		t.next = time.Now().Add(t.dur)
	}
}
