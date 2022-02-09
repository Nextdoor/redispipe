package redisconn

import (
	"sync"
	"time"
)

type ReconnectThrottle interface {
	GetBackoff(conn *Connection, now time.Time) time.Duration
	ConnClosed(conn *Connection)
}

type NoReconnect struct{}

func (n NoReconnect) GetBackoff(_ *Connection, _ time.Time) time.Duration {
	panic("not implemented")
}

func (n NoReconnect) ConnClosed(_ *Connection) {
	panic("not implemented")
}

type DurationReconnect struct {
	dur time.Duration
}

func NewDurationReconnect(dur time.Duration) ReconnectThrottle {
	return &DurationReconnect{
		dur: dur,
	}
}

func (d *DurationReconnect) GetBackoff(_ *Connection, _ time.Time) time.Duration {
	return d.dur
}

func (d *DurationReconnect) ConnClosed(_ *Connection) {}

// ExpBackoffReconnect implements an exponential backoff with a decorrelated jitter
type ExpBackoffReconnect struct {
	randIntFunc func(max int64) int64
	base        time.Duration // minimum time to sleep
	cap         time.Duration // maximum time to sleep
	reset       time.Duration // after how much time to go back tofi base
	mu          sync.Mutex
	trackers    map[*Connection]*timeTracker
}

type timeTracker struct {
	cap        time.Duration // based off of ExpBackoffReconnect.cap but includes jitter
	backoff    time.Duration
	updateTime time.Time // used to reset the backoff after ExpBackoffReconnect.reset window
}

func NewExpBackoffReconnect(randIntFunc func(max int64) int64, base time.Duration, cap time.Duration, reset time.Duration) ReconnectThrottle {
	return &ExpBackoffReconnect{
		randIntFunc: randIntFunc,
		base:        base,
		cap:         cap,
		reset:       reset,
		mu:          sync.Mutex{},
		trackers:    map[*Connection]*timeTracker{},
	}
}

// getNewCap provides a new cap with an element of jitter. The value will be within 1/8 of cap.
func (e *ExpBackoffReconnect) getNewCap() time.Duration {
	window := e.cap / 8

	jitter := e.randIntFunc(window.Nanoseconds())
	return e.cap - (time.Duration(jitter) * time.Nanosecond)
}

func (e *ExpBackoffReconnect) GetBackoff(conn *Connection, now time.Time) time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()

	var tracker = e.trackers[conn]
	if tracker == nil {
		tracker = &timeTracker{
			backoff: e.base,
			cap:     e.getNewCap(),
		}

		e.trackers[conn] = tracker
	} else {
		// Reset the sleep time back to base if enough time has passed
		if tracker.updateTime.Add(e.reset).Before(now) {
			tracker.backoff = e.base
		}
	}

	tracker.updateTime = now

	// Pick new backoff
	maxBackoff := 3 * tracker.backoff

	// Get jitter between the base and new backoff
	newBackoff := maxBackoff - e.base
	if newBackoff > 0 {
		// Add jitter to backoff
		val := e.randIntFunc(newBackoff.Nanoseconds()) + e.base.Nanoseconds()
		newBackoff = time.Duration(val) * time.Nanosecond
	} else {
		newBackoff = e.base
	}

	// Clamp the backoff
	if newBackoff > tracker.cap {
		newBackoff = tracker.cap
	}

	tracker.backoff = newBackoff

	return tracker.backoff
}

func (e *ExpBackoffReconnect) ConnClosed(conn *Connection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.trackers, conn)
}
