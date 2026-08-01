package httpapi

import (
	"testing"
	"time"
)

func TestStreamGuardLimitsConcurrencyAndRequestRate(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	guard := newStreamGuard(func() time.Time { return now }, 2, 4)
	releaseFirst, ok, _ := guard.begin(1, true)
	if !ok {
		t.Fatal("first stream rejected")
	}
	releaseSecond, ok, _ := guard.begin(1, true)
	if !ok {
		t.Fatal("second stream rejected")
	}
	if _, ok, _ := guard.begin(1, true); ok {
		t.Fatal("third concurrent stream accepted")
	}
	releaseFirst()
	releaseThird, ok, _ := guard.begin(1, true)
	if !ok {
		t.Fatal("stream rejected after release")
	}
	releaseSecond()
	releaseThird()
	if _, ok, _ := guard.begin(1, false); !ok {
		t.Fatal("fourth request rejected")
	}
	if _, ok, retry := guard.begin(1, false); ok || retry <= 0 {
		t.Fatalf("request rate limit not applied, ok=%v retry=%s", ok, retry)
	}
	now = now.Add(time.Minute)
	if _, ok, _ := guard.begin(1, false); !ok {
		t.Fatal("request rate limit did not reset")
	}
}
