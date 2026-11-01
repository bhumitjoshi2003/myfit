package api

import (
	"testing"
	"time"
)

func TestLocalDayFollowsUserZone(t *testing.T) {
	cases := []struct {
		name, tz, want string
		at             time.Time
	}{
		{"9:30 PM IST on Oct 31", "Asia/Kolkata", "2026-10-31", time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)},
		{"11:59 PM IST on Oct 31", "Asia/Kolkata", "2026-10-31", time.Date(2026, 10, 31, 18, 29, 0, 0, time.UTC)},
		{"12:00 AM IST on Nov 1", "Asia/Kolkata", "2026-11-01", time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)},
		{"same instant is still Oct 31 in UTC", "UTC", "2026-10-31", time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)},
		{"zone ahead of IST is already Nov 1", "Asia/Tokyo", "2026-11-01", time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got, e := localDay(c.tz, c.at)
		if e != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, e, c.want)
		}
	}
	if _, e := localDay("Not/AZone", time.Now()); e == nil {
		t.Error("invalid zone accepted")
	}
}
