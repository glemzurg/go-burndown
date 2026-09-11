package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-burndown/config"
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

func TestExampleBaseConfigIsSelfContained(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		reportType string
		outputFile string
		wantType   string
	}{
		{name: "default report", reportType: "", outputFile: defaultBurndownFile, wantType: config.ReportTypeBurndown},
		{name: "explicit date forecast", reportType: config.ReportTypeBurndown, outputFile: defaultBurndownFile, wantType: config.ReportTypeBurndown},
		{name: "velocity scoreboard", reportType: config.ReportTypeBurnup, outputFile: defaultBurnupFile, wantType: config.ReportTypeBurnup},
		{name: "velocity scoreboard mixed case", reportType: "Burnup", outputFile: defaultBurnupFile, wantType: config.ReportTypeBurnup},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exampleBaseConfig(now, tc.reportType)
			require.NoError(t, cfg.Validate())
			assert.Equal(t, tc.outputFile, cfg.OutputFile)
			assert.Equal(t, tc.wantType, cfg.EffectiveReportType())
			assert.Equal(t, exampleStartDate(now).Format("2006-01-02"), cfg.StartDate)
		})
	}

	cfg := exampleBaseConfig(now, "")
	issues, err := createExampleIssues(&cfg)
	require.NoError(t, err)
	require.NotEmpty(t, issues)
}
