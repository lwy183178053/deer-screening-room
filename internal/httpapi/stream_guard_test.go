package httpapi

import (
	"testing"
	"time"
)

func TestStreamGuardLimitsRequestRate(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	guard := newStreamGuard(func() time.Time { return now }, 4)
	for i := 0; i < 4; i++ {
		if ok, _ := guard.begin(1); !ok {
			t.Fatal("request rejected before rate limit")
		}
	}
	if ok, retry := guard.begin(1); ok || retry <= 0 {
		t.Fatalf("request rate limit not applied, ok=%v retry=%s", ok, retry)
	}
	now = now.Add(time.Minute)
	if ok, _ := guard.begin(1); !ok {
		t.Fatal("request rate limit did not reset")
	}
}
