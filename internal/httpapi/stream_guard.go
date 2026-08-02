package httpapi

import (
	"sync"
	"time"
)

type streamGuardEntry struct {
	active      int
	requests    int
	windowReset time.Time
	lastSeen    time.Time
}

type streamGuard struct {
	mu                sync.Mutex
	entries           map[int64]streamGuardEntry
	now               func() time.Time
	maxActive         int
	requestsPerMinute int
}

func newStreamGuard(now func() time.Time, maxActive, requestsPerMinute int) *streamGuard {
	return &streamGuard{entries: make(map[int64]streamGuardEntry), now: now, maxActive: maxActive, requestsPerMinute: requestsPerMinute}
}

func (g *streamGuard) begin(userID int64, active bool) (func(), bool, time.Duration) {
	now := g.now()
	g.mu.Lock()
	entry := g.entries[userID]
	if !entry.windowReset.After(now) {
		entry.requests = 0
		entry.windowReset = now.Add(time.Minute)
	}
	if entry.requests >= g.requestsPerMinute {
		retry := entry.windowReset.Sub(now)
		g.mu.Unlock()
		return func() {}, false, retry
	}
	if active && entry.active >= g.maxActive {
		g.mu.Unlock()
		return func() {}, false, time.Second
	}
	entry.requests++
	if active {
		entry.active++
	}
	entry.lastSeen = now
	g.entries[userID] = entry
	g.cleanupLocked(now)
	g.mu.Unlock()
	return func() {
		if !active {
			return
		}
		g.mu.Lock()
		current := g.entries[userID]
		if current.active > 0 {
			current.active--
		}
		current.lastSeen = g.now()
		g.entries[userID] = current
		g.mu.Unlock()
	}, true, 0
}

func (g *streamGuard) cleanupLocked(now time.Time) {
	for userID, entry := range g.entries {
		if entry.active == 0 && now.Sub(entry.lastSeen) > 15*time.Minute {
			delete(g.entries, userID)
		}
	}
}
