package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		errMessage string
	}{
		{
			name: "ok",
			config: Config{
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
			},
		},

		{
			name: "missing output file",
			config: Config{
				OutputFile:     "",
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
			},
			errMessage: `'OutputFile' failed on the 'required' tag`,
		},

		{
			name: "missing start date",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "",
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
			},
			errMessage: `'StartDate' failed on the 'required' tag`,
		},

		{
			name: "malformed start date",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "01-02-2024", // Not the correct date format.
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
			},
			errMessage: `'StartDate' failed on the 'datetime' tag`,
		},

		{
			name: "missing JQL",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'JQL' failed on the 'required' tag`,
		},

		{
			name: "missing moving average weeks",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 0,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'MovingAvgWeeks' failed on the 'required' tag`,
		},

		{
			name: "missing Jira URL",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'JiraURL' failed on the 'required' tag`,
		},

		{
			name: "malformed Jira URL",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "noturl",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'JiraURL' failed on the 'url' tag`,
		},

		{
			name: "missing Jira username",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'Username' failed on the 'required' tag`,
		},

		{
			name: "missing API token",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "UserName",
					APIToken:             "",
					SizeField:            "SizeField",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'APIToken' failed on the 'required' tag`,
		},

		{
			name: "missing size field",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "",
					PercentCompleteField: "PercentCompleteField",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'SizeField' failed on the 'required' tag`,
		},

		{
			name: "missing percent complete field",
			config: Config{
				OutputFile:     "OutputFile",
				StartDate:      "2024-01-01",
				JQL:            "Jql",
				MovingAvgWeeks: 1,
				Jira: JiraConfig{
					JiraURL:              "https://example.atlassian.net",
					Username:             "UserName",
					APIToken:             "ApiToken",
					SizeField:            "SizeField",
					PercentCompleteField: "",
					DoneStatuses:         []string{"Done"},
				},
			},
			errMessage: `'PercentCompleteField' failed on the 'required' tag`,
		},

		{
			name: "missing done statuses (nil)",
			config: Config{
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
					DoneStatuses:         nil,
				},
			},
			errMessage: `'DoneStatuses' failed on the 'required' tag`,
		},

		{
			name: "missing done statuses (empty)",
			config: Config{
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
					DoneStatuses:         []string{},
				},
			},
			errMessage: `'DoneStatuses' failed on the 'min' tag`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.errMessage == "" {
				assert.NoError(t, err, `expected no errors`)
			} else {
				assert.ErrorContains(t, err, tt.errMessage, `expected error`)
			}
		})
	}
}
