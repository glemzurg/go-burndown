package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validConfig() Config {
	return Config{
		OutputFile:     "OutputFile",
		StartDate:      "2024-01-01",
		JQL:            "Jql",
		MovingAvgWeeks: 1,
		Jira: JiraConfig{
			JiraURL:              "https://example.atlassian.net",
			Username:             "UserName",
			APIToken:             "ApiToken",
			SizeField:            "SizeField",
			PercentCompleteField: "PercentCompleteField",
			DoneStatuses:         []string{"Done"},
		},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Config)
		errMessage string
	}{
		{name: "ok"},
		{
			name:       "missing output file",
			mutate:     func(c *Config) { c.OutputFile = "" },
			errMessage: `'OutputFile' failed on the 'required' tag`,
		},
		{
			name:       "missing start date",
			mutate:     func(c *Config) { c.StartDate = "" },
			errMessage: `'StartDate' failed on the 'required' tag`,
		},
		{
			name:       "malformed start date",
			mutate:     func(c *Config) { c.StartDate = "01-02-2024" },
			errMessage: `'StartDate' failed on the 'datetime' tag`,
		},
		{
			name:       "missing JQL",
			mutate:     func(c *Config) { c.JQL = "" },
			errMessage: `'JQL' failed on the 'required' tag`,
		},
		{
			name:       "missing moving average weeks",
			mutate:     func(c *Config) { c.MovingAvgWeeks = 0 },
			errMessage: `'MovingAvgWeeks' failed on the 'required' tag`,
		},
		{
			name:       "missing Jira URL",
			mutate:     func(c *Config) { c.Jira.JiraURL = "" },
			errMessage: `'JiraURL' failed on the 'required' tag`,
		},
		{
			name:       "malformed Jira URL",
			mutate:     func(c *Config) { c.Jira.JiraURL = "noturl" },
			errMessage: `'JiraURL' failed on the 'url' tag`,
		},
		{
			name:       "missing Jira username",
			mutate:     func(c *Config) { c.Jira.Username = "" },
			errMessage: `'Username' failed on the 'required' tag`,
		},
		{
			name:       "missing API token",
			mutate:     func(c *Config) { c.Jira.APIToken = "" },
			errMessage: `'APIToken' failed on the 'required' tag`,
		},
		{
			name:       "missing size field",
			mutate:     func(c *Config) { c.Jira.SizeField = "" },
			errMessage: `'SizeField' failed on the 'required' tag`,
		},
		{
			name:       "missing percent complete field",
			mutate:     func(c *Config) { c.Jira.PercentCompleteField = "" },
			errMessage: `'PercentCompleteField' failed on the 'required' tag`,
		},
		{
			name:       "missing done statuses (nil)",
			mutate:     func(c *Config) { c.Jira.DoneStatuses = nil },
			errMessage: `'DoneStatuses' failed on the 'required' tag`,
		},
		{
			name:   "report type burnup",
			mutate: func(c *Config) { c.ReportType = ReportTypeBurnup },
		},
		{
			name:       "invalid report type",
			mutate:     func(c *Config) { c.ReportType = "sideways" },
			errMessage: `report_type must be "burndown" or "burnup"`,
		},
		{
			name:       "missing done statuses (empty)",
			mutate:     func(c *Config) { c.Jira.DoneStatuses = []string{} },
			errMessage: `'DoneStatuses' failed on the 'min' tag`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}
			err := cfg.Validate()
			if tt.errMessage == "" {
				assert.NoError(t, err, `expected no errors`)
			} else {
				assert.ErrorContains(t, err, tt.errMessage, `expected error`)
			}
		})
	}
}

func TestValidateFromOverrides(t *testing.T) {
	ok := Config{
		OutputFile:     "out.xlsx",
		StartDate:      "2026-01-01",
		MovingAvgWeeks: 3,
		OverridesDir:   "overrides",
		Jira: JiraConfig{
			SizeField:            "customfield_1",
			PercentCompleteField: "Percentage Complete",
			DoneStatuses:         []string{"Done"},
		},
	}
	assert.NoError(t, ok.ValidateFromOverrides())

	// JQL / credentials not required for from-overrides validation.
	assert.NoError(t, ok.ValidateFromOverrides())

	missingDir := ok
	missingDir.OverridesDir = ""
	assert.ErrorContains(t, missingDir.ValidateFromOverrides(), "overrides_dir")

	missingSize := ok
	missingSize.Jira.SizeField = ""
	assert.ErrorContains(t, missingSize.ValidateFromOverrides(), "size_field")

	badType := ok
	badType.ReportType = "nope"
	assert.ErrorContains(t, badType.ValidateFromOverrides(), "report_type")
}

func TestNormalizeReportType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty is burndown", input: "", want: ReportTypeBurndown},
		{name: "whitespace is burndown", input: "  ", want: ReportTypeBurndown},
		{name: "burndown", input: "burndown", want: ReportTypeBurndown},
		{name: "burnup", input: "burnup", want: ReportTypeBurnup},
		{name: "burnup mixed case", input: "BurnUp", want: ReportTypeBurnup},
		{name: "invalid", input: "forecast", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeReportType(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	burnup := Config{ReportType: "BURNUP"}
	assert.True(t, burnup.IsBurnup())
	assert.Equal(t, ReportTypeBurnup, burnup.EffectiveReportType())

	def := Config{}
	assert.False(t, def.IsBurnup())
	assert.Equal(t, ReportTypeBurndown, def.EffectiveReportType())
}
