// Package main provides the command-line interface for the Jira burndown report generator.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

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

	// Query Jira
	issues, err := jira.QueryJira(ctx, &config)
	if err != nil {
		wrappedErr := errors.Wrap(err, "failed to query Jira")
		log.Fatalf("Jira query error: %+v", wrappedErr)
	}

	// Generate Excel report
	err = excel.GenerateExcelReport(&config, issues)
	if err != nil {
		wrappedErr := errors.Wrap(err, "failed to generate Excel report")
		log.Fatalf("Excel generation error: %+v", wrappedErr)
	}

	fmt.Printf("Burndown report generated: %s\n", config.OutputFile)
}
