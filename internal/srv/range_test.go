package srv

import (
	"testing"
	"time"
)

func TestCalendarIntervals(t *testing.T) {
	cases := []struct{ at, interval, want string }{
		{"2024-02-01T00:00:00Z", "1M", "2024-03-01T00:00:00Z"},
		{"2024-03-04T00:00:00Z", "1w", "2024-03-11T00:00:00Z"},
		{"2024-01-01T00:00:00Z", "4h", "2024-01-01T04:00:00Z"},
	}
	for _, tc := range cases {
		at, _ := time.Parse(time.RFC3339, tc.at)
		want, _ := time.Parse(time.RFC3339, tc.want)
		got, err := NextOpen(at, tc.interval)
		if err != nil || !got.Equal(want) {
			t.Fatalf("%s: %v %v", tc.interval, got, err)
		}
	}
	at := time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC)
	if _, err := NextOpen(at, "1m"); err == nil {
		t.Fatal("unaligned time accepted")
	}
	if _, err := NextOpen(at, "unsupported"); err == nil {
		t.Fatal("unknown interval accepted")
	}
}
