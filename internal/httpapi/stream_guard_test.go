package httpapi

import (
	"testing"
	"time"
)

func TestStreamGuardLimitsOnlyRequestRate(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	guard := newStreamGuard(func() time.Time { return now }, 4)
	for request := 1; request <= 4; request++ {
		if ok, _ := guard.allow(1); !ok {
			t.Fatalf("request %d rejected", request)
		}
	}
	if ok, retry := guard.allow(1); ok || retry <= 0 {
		t.Fatalf("request rate limit not applied, ok=%v retry=%s", ok, retry)
	}
	now = now.Add(time.Minute)
	if ok, _ := guard.allow(1); !ok {
		t.Fatal("request rate limit did not reset")
	}
}
