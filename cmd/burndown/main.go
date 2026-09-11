// Package main provides the command-line interface for the Jira burndown report generator.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go-burndown/config"
	"go-burndown/excel"
	"go-burndown/jira"
	"go-burndown/override"

	"github.com/pkg/errors"
)

const (
	// exampleLookbackWeeks is how far before the last Tuesday the demo project starts.
	// Progress is seeded on each weekly boundary from that start through last Tuesday.
	exampleLookbackWeeks = 6
	defaultBurndownFile  = "burndown.xlsx"
	defaultBurnupFile    = "burnup.xlsx"
	doneStatus           = "Done"
)

func main() {
	configFile := flag.String("config", "", "Path to configuration file")
	jql := flag.String("jql", "", "JQL query")
	outputFile := flag.String("output", "", "Output Excel file")
	startDate := flag.String("start-date", "", "Project start date (YYYY-MM-DD)")
	overridesDir := flag.String("overrides-dir", "", "Directory of hand-maintained issue overlay JSON files ({KEY}.json or {KEY}-*.json)")
	reportType := flag.String("report-type", "", "Report type: burndown (default) or burnup")
	example := flag.Bool("example", false, "Create example spreadsheet with mock data (no config file or Jira required)")
	fromOverrides := flag.Bool("from-overrides", false, "Load issues only from overrides-dir (no Jira; file names supply issue keys)")
	flag.Parse()

	if *example && *fromOverrides {
		log.Fatal("use only one of --example or --from-overrides")
	}
	if *reportType != "" {
		if _, err := config.NormalizeReportType(*reportType); err != nil {
			log.Fatalf("Configuration error: %+v", err)
		}
	}

	var cfg config.Config
	var err error
	var issues []jira.Issue

	switch {
	case *example:
		// Demo mode: no config file; built-in mock tickets; optional overlays on top.
		cfg = exampleBaseConfig(time.Now(), *reportType)
		applyCommonFlags(&cfg, outputFile, startDate, overridesDir, reportType)
		issues, err = createExampleIssues(&cfg)
		if err != nil {
			log.Fatalf("Failed to create example issues: %+v", err)
		}
		if err := override.Apply(cfg.OverridesDir, issues, &cfg); err != nil {
			log.Fatalf("Local overrides error: %+v", err)
		}

	case *fromOverrides:
		// Overrides-only mode: config required; issues load from overrides_dir; no Jira API.
		cfg, err = loadConfigFile(*configFile)
		if err != nil {
			log.Fatalf("Config loading error: %+v", err)
		}
		applyCommonFlags(&cfg, outputFile, startDate, overridesDir, reportType)
		if err := cfg.ValidateFromOverrides(); err != nil {
			log.Fatalf("Configuration error: %+v", err)
		}
		issues, err = override.LoadAll(cfg.OverridesDir, &cfg)
		if err != nil {
			log.Fatalf("Load overrides error: %+v", err)
		}

	default:
		// Jira mode: config required; query Jira, then optional local overlays.
		cfg, err = loadConfigFile(*configFile)
		if err != nil {
			log.Fatalf("Config loading error: %+v", err)
		}
		if *jql != "" {
			cfg.JQL = *jql
		}
		applyCommonFlags(&cfg, outputFile, startDate, overridesDir, reportType)
		if err := cfg.Validate(); err != nil {
			log.Fatalf("Configuration error: %+v", err)
		}
		issues, err = jira.QueryJira(context.Background(), &cfg)
		if err != nil {
			wrappedErr := errors.Wrap(err, "failed to query Jira")
			log.Fatalf("Jira query error: %+v", wrappedErr)
		}
		if err := override.Apply(cfg.OverridesDir, issues, &cfg); err != nil {
			log.Fatalf("Local overrides error: %+v", err)
		}
	}

	if err := excel.GenerateExcelReport(&cfg, issues); err != nil {
		wrappedErr := errors.Wrap(err, "failed to generate Excel report")
		log.Fatalf("Excel generation error: %+v", wrappedErr)
	}

	kind := "Burndown"
	if cfg.IsBurnup() {
		kind = "Burnup"
	}
	fmt.Printf("%s report generated: %s\n", kind, cfg.OutputFile)
}

func loadConfigFile(configPath string) (config.Config, error) {
	if configPath == "" {
		configPath = "config.json"
	}
	return config.LoadConfig(configPath)
}

func applyCommonFlags(cfg *config.Config, outputFile, startDate, overridesDir, reportType *string) {
	if *outputFile != "" {
		cfg.OutputFile = *outputFile
	}
	if *startDate != "" {
		cfg.StartDate = *startDate
	}
	if *overridesDir != "" {
		cfg.OverridesDir = *overridesDir
	}
	if *reportType != "" {
		cfg.ReportType = *reportType
	}
}

// exampleBaseConfig is used only for --example (no config file or Jira).
func exampleBaseConfig(now time.Time, reportType string) config.Config {
	normalized, _ := config.NormalizeReportType(reportType)
	outputFile := defaultBurndownFile
	if normalized == config.ReportTypeBurnup {
		outputFile = defaultBurnupFile
	}
	return config.Config{
		OutputFile:     outputFile,
		StartDate:      exampleStartDate(now).Format("2006-01-02"),
		JQL:            "example",
		MovingAvgWeeks: 3,
		ReportType:     normalized,
		Jira: config.JiraConfig{
			JiraURL:              "https://example.atlassian.net",
			Username:             "example@example.com",
			APIToken:             "example",
			SizeField:            "customfield_10028",
			PercentCompleteField: "Percentage Complete",
			DoneStatuses:         []string{doneStatus, "Closed", "Resolved", "Complete", "Completed"},
		},
	}
}

// lastTuesdayOnOrBefore returns the most recent Tuesday on or before now (date only).
func lastTuesdayOnOrBefore(now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := (int(day.Weekday()) - int(time.Tuesday) + 7) % 7
	return day.AddDate(0, 0, -offset)
}

// exampleStartDate is six weeks before the last Tuesday on or before now.
func exampleStartDate(now time.Time) time.Time {
	return lastTuesdayOnOrBefore(now).AddDate(0, 0, -7*exampleLookbackWeeks)
}

// createExampleIssues builds mock issues with changelog history on each project week
// so the Work and Projections sheets show realistic % complete and earned value.
//
// Progress is week-indexed from config.StartDate across the six-week demo window
// (seven weekly points: start through last Tuesday inclusive).
func createExampleIssues(config *config.Config) ([]jira.Issue, error) {
	startDate, err := time.ParseInLocation("2006-01-02", config.StartDate, time.Local)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid start date for example data: %s", config.StartDate)
	}

	// progressByWeek[i] is percent complete (0.0–1.0) on startDate + i weeks.
	// Seven samples cover start .. start+6w (last Tuesday when start is computed by exampleStartDate).
	//
	// Shaped so weekly velocity has one early spike (ticket finished in week 1) then a
	// steadier pace: the first Slow (p90) CI week can show "unknown" (lower bound ≤ 0),
	// while later weeks have positive V. Slow for real pessimistic dates.
	exampleData := []struct {
		key            string
		summary        string
		issueType      string
		assignee       string
		size           float64
		progressByWeek []float64
	}{
		{
			key:            "TICKET-1234",
			summary:        "Work stuff",
			issueType:      "Task",
			assignee:       "Alice",
			size:           1,
			progressByWeek: []float64{0.2, 0.5, 0.65, 0.75, 0.85, 0.95, 1.0},
		},
		{
			key:       "TICKET-1235",
			summary:   "More work stuff",
			issueType: "Bug",
			assignee:  "Bob",
			size:      2,
			// Early finish creates the first-week velocity spike (wide early CI).
			progressByWeek: []float64{0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0},
		},
		{
			key:            "TICKET-1236",
			summary:        "Big work stuff",
			issueType:      "Story",
			assignee:       "Carol",
			size:           3,
			progressByWeek: []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
		},
		{
			key:       "TICKET-1237",
			summary:   "Tall work stuff",
			issueType: "Story",
			assignee:  "Bob",
			size:      5,
			// Steady ramp (no one-week jump) keeps later velocity variance low.
			progressByWeek: []float64{0, 0.1, 0.2, 0.3, 0.42, 0.52, 0.62},
		},
		{
			key:            "TICKET-1238",
			summary:        "Short work stuff",
			issueType:      "Task",
			assignee:       "Alice",
			size:           8,
			progressByWeek: []float64{0, 0.05, 0.1, 0.16, 0.22, 0.28, 0.35},
		},
		{
			key:            "TICKET-1239",
			summary:        "A big block of work stuff",
			issueType:      "Epic",
			assignee:       "Dana",
			size:           13,
			progressByWeek: []float64{0, 0, 0, 0, 0, 0, 0},
		},
	}

	var issues []jira.Issue
	now := time.Now()

	for _, data := range exampleData {
		finalPercent := 0.0
		if len(data.progressByWeek) > 0 {
			finalPercent = data.progressByWeek[len(data.progressByWeek)-1]
		}
		status := statusForProgress(finalPercent)

		issue := jira.Issue{
			Key: data.key,
			Fields: jira.Fields{
				Summary: data.summary,
				Status: struct {
					Name string `json:"name"`
				}{Name: status},
				Issuetype: struct {
					Name string `json:"name"`
				}{Name: data.issueType},
				Assignee: struct {
					DisplayName string `json:"displayName"`
				}{DisplayName: data.assignee},
				Created:      startDate.Format("2006-01-02T15:04:05.000-0700"),
				Updated:      now.Format("2006-01-02T15:04:05.000-0700"),
				CustomFields: make(map[string]interface{}),
			},
			Changelog: struct {
				Histories []jira.History `json:"histories"`
			}{},
		}

		issue.Fields.CustomFields[config.Jira.SizeField] = data.size

		// One changelog entry per week where percent changes, so history-based
		// weekly columns and velocity projections have real cumulative progress.
		var histories []jira.History
		prevPercent := -1.0
		for weekIndex, percent := range data.progressByWeek {
			if percent == prevPercent {
				continue
			}
			// Skip pure zeros: blank % cells read as "not started" until first progress.
			if percent <= 0 && prevPercent < 0 {
				prevPercent = percent
				continue
			}

			weekDate := startDate.AddDate(0, 0, 7*weekIndex)
			histories = append(histories, jira.History{
				Created: weekDate.Format(jira.JIRARFC3339TimeLayout),
				Items: []struct {
					Field      string `json:"field"`
					Fieldtype  string `json:"fieldtype"`
					FromString string `json:"fromString"`
					ToString   string `json:"toString"`
				}{
					{
						Field:     config.Jira.PercentCompleteField,
						Fieldtype: "custom",
						ToString:  fmt.Sprintf("%g", percent),
					},
				},
				CreatedTime: weekDate,
			})
			prevPercent = percent
		}

		// When fully done, record a Done status change so done_statuses also mark 100%.
		if finalPercent >= 1.0 {
			doneWeek := startDate.AddDate(0, 0, 7*(len(data.progressByWeek)-1))
			histories = append(histories, jira.History{
				Created: doneWeek.Format(jira.JIRARFC3339TimeLayout),
				Items: []struct {
					Field      string `json:"field"`
					Fieldtype  string `json:"fieldtype"`
					FromString string `json:"fromString"`
					ToString   string `json:"toString"`
				}{
					{
						Field:     "status",
						Fieldtype: "jira",
						ToString:  doneStatus,
					},
				},
				CreatedTime: doneWeek,
			})
		}

		issue.Changelog.Histories = histories
		issues = append(issues, issue)
	}

	return issues, nil
}

func statusForProgress(percent float64) string {
	switch {
	case percent >= 1.0:
		return doneStatus
	case percent > 0:
		return "In Progress"
	default:
		return "To Do"
	}
}
