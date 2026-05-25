package triage

import "time"

type TicketTimer struct {
	ticketID int64
	elapsed  time.Duration
	started  time.Time
	running  bool
}

func (t *TicketTimer) Focus(ticketID int64, now time.Time) {
	if t.running {
		t.elapsed += now.Sub(t.started)
	}
	if t.ticketID != ticketID {
		t.ticketID = ticketID
		t.elapsed = 0
	}
	t.started = now
	t.running = ticketID != 0
}

func (t *TicketTimer) Pause(now time.Time) {
	if !t.running {
		return
	}
	t.elapsed += now.Sub(t.started)
	t.running = false
}

func (t *TicketTimer) Resume(now time.Time) {
	if t.ticketID == 0 || t.running {
		return
	}
	t.started = now
	t.running = true
}

func (t *TicketTimer) Reset(now time.Time) {
	t.elapsed = 0
	if t.running {
		t.started = now
	}
}

func (t TicketTimer) Elapsed(now time.Time) time.Duration {
	if t.running {
		return t.elapsed + now.Sub(t.started)
	}
	return t.elapsed
}

func (t TicketTimer) ElapsedSeconds(now time.Time) int {
	seconds := int(t.Elapsed(now).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func (t TicketTimer) TicketID() int64 {
	return t.ticketID
}

func (t TicketTimer) Running() bool {
	return t.running
}
