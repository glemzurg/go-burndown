package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastTuesdayOnOrBefore(t *testing.T) {
	loc := time.UTC
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "already tuesday",
			now:  time.Date(2026, 8, 4, 15, 30, 0, 0, loc), // Tuesday
			want: time.Date(2026, 8, 4, 0, 0, 0, 0, loc),
		},
		{
			name: "wednesday uses prior tuesday",
			now:  time.Date(2026, 8, 5, 9, 0, 0, 0, loc), // Wednesday
			want: time.Date(2026, 8, 4, 0, 0, 0, 0, loc),
		},
		{
			name: "monday uses prior tuesday",
			now:  time.Date(2026, 8, 3, 12, 0, 0, 0, loc), // Monday
			want: time.Date(2026, 7, 28, 0, 0, 0, 0, loc),
		},
		{
			name: "sunday uses prior tuesday",
			now:  time.Date(2026, 8, 9, 0, 0, 0, 0, loc), // Sunday
			want: time.Date(2026, 8, 4, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lastTuesdayOnOrBefore(tc.now)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExampleStartDate(t *testing.T) {
	// Last Tuesday 2026-08-04 → start is six weeks earlier: 2026-06-23.
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) // Saturday
	got := exampleStartDate(now)
	require.Equal(t, time.Tuesday, got.Weekday())
	assert.Equal(t, time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC), got)
	assert.Equal(t, 6*7, int(lastTuesdayOnOrBefore(now).Sub(got).Hours()/24))
}
