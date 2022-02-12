package redisconn

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDurationReconnect(t *testing.T) {
	dur := time.Duration(42) * time.Millisecond
	r := NewDurationReconnect(dur)

	assert.Equal(t, dur, r.GetBackoff(nil, time.Now()))
}

func TestExpBackoffReconnectStepsMax(t *testing.T) {
	base := time.Duration(1) * time.Millisecond
	max := time.Duration(200) * time.Millisecond
	reset := 5 * time.Minute

	randIntFunc := func(max int64) int64 { return max }

	r := NewExpBackoffReconnect(randIntFunc, base, max, reset)
	c := &Connection{}
	now, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:04:05 2022")

	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 9*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 27*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 81*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 175*time.Millisecond, r.GetBackoff(c, now)) // Max
	assert.Equal(t, 175*time.Millisecond, r.GetBackoff(c, now)) // Max

	later, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:09:15 2022")
	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c, later))
}

func TestExpBackoffReconnectStepsMin(t *testing.T) {
	base := time.Duration(1) * time.Millisecond
	max := time.Duration(200) * time.Millisecond
	reset := 5 * time.Minute

	randIntFunc := func(max int64) int64 { return 0 }

	r := NewExpBackoffReconnect(randIntFunc, base, max, reset)
	c := &Connection{}
	now, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:04:05 2022")

	assert.Equal(t, 1*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 1*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 1*time.Millisecond, r.GetBackoff(c, now))

	later, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:09:15 2022")
	assert.Equal(t, 1*time.Millisecond, r.GetBackoff(c, later))
}

func TestExpBackoffReconnectMultipleConns(t *testing.T) {
	base := time.Duration(1) * time.Millisecond
	max := time.Duration(200) * time.Millisecond
	reset := 5 * time.Minute

	randIntFunc := func(max int64) int64 { return max }

	r := NewExpBackoffReconnect(randIntFunc, base, max, reset)
	c1 := &Connection{}
	c2 := &Connection{}

	now, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:04:05 2022")

	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c1, now))
	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c2, now))

	assert.Equal(t, 9*time.Millisecond, r.GetBackoff(c1, now))
	assert.Equal(t, 9*time.Millisecond, r.GetBackoff(c2, now))

	assert.Equal(t, 27*time.Millisecond, r.GetBackoff(c1, now))
	assert.Equal(t, 27*time.Millisecond, r.GetBackoff(c2, now))
}

func TestExpBackoffReconnectConnClosed(t *testing.T) {
	base := time.Duration(1) * time.Millisecond
	max := time.Duration(200) * time.Millisecond
	reset := 5 * time.Minute

	randIntFunc := func(max int64) int64 { return max }

	r := NewExpBackoffReconnect(randIntFunc, base, max, reset)
	c := &Connection{}

	now, _ := time.Parse(time.ANSIC, "Mon Jan 2 15:04:05 2022")

	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c, now))
	assert.Equal(t, 9*time.Millisecond, r.GetBackoff(c, now))
	r.ConnClosed(c)

	assert.Equal(t, 3*time.Millisecond, r.GetBackoff(c, now))
}
