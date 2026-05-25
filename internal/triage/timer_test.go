package triage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicketTimerAccumulatesPausesAndResets(t *testing.T) {
	start := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	var timer TicketTimer

	timer.Focus(123, start)
	assert.True(t, timer.Running())
	assert.Equal(t, 30, timer.ElapsedSeconds(start.Add(30*time.Second)))

	timer.Pause(start.Add(45 * time.Second))
	assert.False(t, timer.Running())
	assert.Equal(t, 45, timer.ElapsedSeconds(start.Add(10*time.Minute)))

	timer.Resume(start.Add(2 * time.Minute))
	assert.Equal(t, 75, timer.ElapsedSeconds(start.Add(2*time.Minute+30*time.Second)))

	timer.Reset(start.Add(3 * time.Minute))
	assert.Equal(t, 0, timer.ElapsedSeconds(start.Add(3*time.Minute)))
}

func TestTicketTimerSwitchesTicketAndClearsElapsed(t *testing.T) {
	start := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	var timer TicketTimer

	timer.Focus(123, start)
	timer.Focus(456, start.Add(time.Minute))

	assert.Equal(t, int64(456), timer.TicketID())
	assert.Equal(t, 0, timer.ElapsedSeconds(start.Add(time.Minute)))
}
