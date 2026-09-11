package excel

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"go-burndown/config"
	"go-burndown/jira"
)

func TestGenerateExcelReportProjections(t *testing.T) {
	t.Parallel()

	start := time.Now().AddDate(0, 0, -28).Format("2006-01-02")
	tests := []struct {
		name          string
		reportType    string
		wantHeaders   []string
		unwantHeaders []string
		diffFormula   string
	}{
		{
			name:       "burndown keeps remaining and forecasts",
			reportType: config.ReportTypeBurndown,
			wantHeaders: []string{
				"Date", "Completed", "Remaining", "Velocity", "Avg (3w)",
				"StdDev (3w)", "Fast (p90)", "Mean", "Slow (p90)",
				"V. Fast (p90)", "V. Slow (p90)",
			},
			unwantHeaders: []string{"Diff (3w)"},
		},
		{
			name:       "burnup gamifies velocity instead of projecting dates",
			reportType: config.ReportTypeBurnup,
			wantHeaders: []string{
				"Date", "Completed", "Velocity", "Avg (3w)", "Diff (3w)",
			},
			unwantHeaders: []string{
				"Remaining", "StdDev (3w)", "Fast (p90)", "Mean",
				"Slow (p90)", "V. Fast (p90)", "V. Slow (p90)",
			},
			diffFormula: "=C4-D4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "report.xlsx")
			cfg := testReportConfig(out, start, tc.reportType)
			require.NoError(t, GenerateExcelReport(cfg, []jira.Issue{testIssue(cfg)}))

			f, err := excelize.OpenFile(out)
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			headers := projectionHeaders(t, f)
			for _, want := range tc.wantHeaders {
				assert.Contains(t, headers, want)
			}
			for _, unwant := range tc.unwantHeaders {
				assert.NotContains(t, headers, unwant)
			}

			if tc.reportType == config.ReportTypeBurnup {
				formula, err := f.GetCellFormula("Projections", "E4")
				require.NoError(t, err)
				assert.Equal(t, tc.diffFormula, formula)
			}
		})
	}
}

func testReportConfig(outputFile, startDate, reportType string) *config.Config {
	return &config.Config{
		OutputFile:     outputFile,
		StartDate:      startDate,
		JQL:            "test",
		MovingAvgWeeks: 3,
		ReportType:     reportType,
		Jira: config.JiraConfig{
			JiraURL:              "https://example.atlassian.net",
			Username:             "user",
			APIToken:             "token",
			SizeField:            "customfield_10028",
			PercentCompleteField: "Percentage Complete",
			DoneStatuses:         []string{"Done"},
		},
	}
}

func testIssue(cfg *config.Config) jira.Issue {
	start, _ := time.Parse("2006-01-02", cfg.StartDate)
	issue := jira.Issue{
		Key: "TICKET-1",
		Fields: jira.Fields{
			Summary:      "Example",
			CustomFields: map[string]interface{}{cfg.Jira.SizeField: 5.0},
		},
	}
	issue.Fields.Status.Name = "In Progress"
	issue.Fields.Issuetype.Name = "Task"
	issue.Changelog.Histories = []jira.History{
		{
			Created:     start.Format(jira.JIRARFC3339TimeLayout),
			CreatedTime: start,
			Items: []struct {
				Field      string `json:"field"`
				Fieldtype  string `json:"fieldtype"`
				FromString string `json:"fromString"`
				ToString   string `json:"toString"`
			}{
				{Field: cfg.Jira.PercentCompleteField, Fieldtype: "custom", ToString: "0.4"},
			},
		},
	}
	return issue
}

func projectionHeaders(t *testing.T, f *excelize.File) []string {
	t.Helper()
	cols, err := f.GetCols("Projections")
	require.NoError(t, err)
	var headers []string
	for _, col := range cols {
		if len(col) > 0 && col[0] != "" {
			headers = append(headers, col[0])
		}
	}
	return headers
}
