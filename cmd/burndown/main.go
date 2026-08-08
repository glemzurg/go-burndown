// Package main provides the command-line interface for the Jira burndown report generator.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"go-burndown/config"
	"go-burndown/excel"
	"go-burndown/jira"

	"github.com/pkg/errors"
)

func main() {
	configFile := flag.String("config", "", "Path to configuration file")
	jql := flag.String("jql", "", "JQL query")
	outputFile := flag.String("output", "", "Output Excel file")
	startDate := flag.String("start-date", "", "Project start date (YYYY-MM-DD)")
	example := flag.Bool("example", false, "Create example spreadsheet with mock data")
	flag.Parse()

	// Set defaults if flags are empty
	configFilePath := *configFile
	if configFilePath == "" {
		configFilePath = "config.json"
	}

	config, err := config.LoadConfig(configFilePath)
	if err != nil {
		log.Fatalf("Config loading error: %+v", err)
	}

	// Override config with command line flags if provided
	if *jql != "" {
		config.JQL = *jql
	}
	if *outputFile != "" {
		config.OutputFile = *outputFile
	}
	if *startDate != "" {
		config.StartDate = *startDate
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Configuration error: %+v", err)
	}

	// Create context for HTTP requests
	ctx := context.Background()

	var issues []jira.Issue

	if *example {
		// Create example data instead of querying Jira
		var err error
		issues, err = createExampleIssues(&config)
		if err != nil {
			log.Fatalf("Failed to create example issues: %+v", err)
		}
	} else {
		// Query Jira
		var err error
		issues, err = jira.QueryJira(ctx, &config)
		if err != nil {
			wrappedErr := errors.Wrap(err, "failed to query Jira")
			log.Fatalf("Jira query error: %+v", wrappedErr)
		}
	}

	// Generate Excel report
	err = excel.GenerateExcelReport(&config, issues)
	if err != nil {
		wrappedErr := errors.Wrap(err, "failed to generate Excel report")
		log.Fatalf("Excel generation error: %+v", wrappedErr)
	}

	fmt.Printf("Burndown report generated: %s\n", config.OutputFile)
}

// createExampleIssues creates mock Jira issues that produce the work data from example/burndown-work.csv
func createExampleIssues(config *config.Config) ([]jira.Issue, error) {
	// Define the example data based on the CSV
	exampleData := []struct {
		key        string
		summary    string
		issueType  string
		status     string
		assignee   string
		size       float64
		progress   map[string]float64 // date (MM-DD) -> percent complete
	}{
		{
			key:       "TICKET-1234",
			summary:   "Work stuff",
			issueType: "Task",
			status:    "To Do",
			assignee:  "Bob",
			size:      1,
			progress: map[string]float64{
				"10-31": 0.1,
				"11-07": 0.2,
				"11-14": 0.4,
				"11-21": 0.8,
				"11-28": 1.0,
			},
		},
		{
			key:       "TICKET-1235",
			summary:   "More work stuff",
			issueType: "Bug",
			status:    "To Do",
			assignee:  "Bob",
			size:      2,
			progress: map[string]float64{
				"10-31": 0,
				"11-07": 1.0,
				"11-14": 1.0,
				"11-21": 1.0,
				"11-28": 1.0,
			},
		},
		{
			key:       "TICKET-1236",
			summary:   "Big work stuff",
			issueType: "Bug",
			status:    "To Do",
			assignee:  "Bob",
			size:      3,
			progress: map[string]float64{
				"10-31": 0,
				"11-07": 0,
				"11-14": 0.1,
				"11-21": 0.2,
				"11-28": 0.3,
			},
		},
		{
			key:       "TICKET-1237",
			summary:   "Tall work stuff",
			issueType: "Bug",
			status:    "To Do",
			assignee:  "Bob",
			size:      5,
			progress: map[string]float64{
				"10-31": 0,
				"11-07": 0,
				"11-14": 0,
				"11-21": 1.0,
				"11-28": 1.0,
			},
		},
		{
			key:       "TICKET-1238",
			summary:   "Short work stuff",
			issueType: "Bug",
			status:    "To Do",
			assignee:  "Bob",
			size:      8,
			progress: map[string]float64{
				"10-31": 0,
				"11-07": 0,
				"11-14": 0,
				"11-21": 0,
				"11-28": 0.3,
			},
		},
		{
			key:       "TICKET-1239",
			summary:   "A big block of work stuff",
			issueType: "Bug",
			status:    "To Do",
			assignee:  "Bob",
			size:      13,
			progress: map[string]float64{
				"10-31": 0,
				"11-07": 0,
				"11-14": 0,
				"11-21": 0,
				"11-28": 0,
			},
		},
	}

	var issues []jira.Issue

	// Current year for date parsing
	currentYear := time.Now().Year()

	for _, data := range exampleData {
		issue := jira.Issue{
			Key: data.key,
			Fields: jira.Fields{
				Summary: data.summary,
				Status: struct {
					Name string `json:"name"`
				}{Name: data.status},
				Issuetype: struct {
					Name string `json:"name"`
				}{Name: data.issueType},
				Assignee: struct {
					DisplayName string `json:"displayName"`
				}{DisplayName: data.assignee},
				Created: time.Now().AddDate(0, 0, -30).Format("2006-01-02T15:04:05.000-0700"),
				Updated: time.Now().Format("2006-01-02T15:04:05.000-0700"),
				CustomFields: make(map[string]interface{}),
			},
			Changelog: struct {
				Histories []jira.History `json:"histories"`
			}{},
		}

		// Set size field
		issue.Fields.CustomFields[config.Jira.SizeField] = data.size

		// Create changelog histories for percent complete updates
		var histories []jira.History
		for dateStr, percent := range data.progress {
			// Parse date (MM-DD) and assume current year
			parsedDate, err := time.Parse("01-02-2006", dateStr+"-"+strconv.Itoa(currentYear))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse date %s", dateStr)
			}

			history := jira.History{
				Created: parsedDate.Format(jira.JIRARFC3339TimeLayout),
				Items: []struct {
					Field      string `json:"field"`
					Fieldtype  string `json:"fieldtype"`
					FromString string `json:"fromString"`
					ToString   string `json:"toString"`
				}{
					{
						Field:     config.Jira.PercentCompleteField,
						Fieldtype: "custom",
						ToString:  fmt.Sprintf("%.1f", percent),
					},
				},
				CreatedTime: parsedDate,
			}
			histories = append(histories, history)
		}

		// Sort histories by date
		sort.Slice(histories, func(i, j int) bool {
			return histories[i].CreatedTime.Before(histories[j].CreatedTime)
		})

		issue.Changelog.Histories = histories

		issues = append(issues, issue)
	}

	return issues, nil
}
