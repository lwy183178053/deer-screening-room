package httpapi

import (
	"sync"
	"time"
)

type streamGuardEntry struct {
	requests    int
	windowReset time.Time
	lastSeen    time.Time
}

type streamGuard struct {
	mu                sync.Mutex
	entries           map[int64]streamGuardEntry
	now               func() time.Time
	requestsPerMinute int
}

func newStreamGuard(now func() time.Time, requestsPerMinute int) *streamGuard {
	return &streamGuard{entries: make(map[int64]streamGuardEntry), now: now, requestsPerMinute: requestsPerMinute}
}

func (g *streamGuard) allow(userID int64) (bool, time.Duration) {
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
		return false, retry
	}
	entry.requests++
	entry.lastSeen = now
	g.entries[userID] = entry
	g.cleanupLocked(now)
	g.mu.Unlock()
	return true, 0
}

func (g *streamGuard) cleanupLocked(now time.Time) {
	for userID, entry := range g.entries {
		if now.Sub(entry.lastSeen) > 15*time.Minute {
			delete(g.entries, userID)
		}
	}
}
