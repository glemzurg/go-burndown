// Package config provides configuration management for the Jira burndown report generator.
package config

import (
	"encoding/json"
	"os"
	"slices"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

// Config holds configuration whats in the burndown and how it generates.
type Config struct {
	OutputFile     string     `json:"output_file" validate:"required"`
	StartDate      string     `json:"start_date" validate:"required,datetime=2006-01-02"`
	JQL            string     `json:"jql" validate:"required"`
	MovingAvgWeeks uint       `json:"moving_avg_weeks" validate:"required"`
	Jira           JiraConfig `json:"jira" validate:"required"`
}

// JiraConfig holds Jira-specific configuration settings.
type JiraConfig struct {
	JiraURL              string   `json:"jira_url" validate:"required,url"`
	Username             string   `json:"username" validate:"required"`
	APIToken             string   `json:"api_token" validate:"required"`
	SizeField            string   `json:"size_field" validate:"required"`
	PercentCompleteField string   `json:"percent_complete_field" validate:"required"`
	DoneStatuses         []string `json:"done_statuses" validate:"required,min=1"`
}

// LoadConfig loads configuration from a JSON file.
func LoadConfig(filename string) (Config, error) {
	var config Config

	file, err := os.Open(filename)
	if err != nil {
		return Config{}, errors.WithStack(err)
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&config); err != nil {
		return Config{}, errors.WithStack(err)
	}

	return config, nil
}

// Validate checks that the configuration has all required fields and valid values.
func (c *Config) Validate() error {
	validate := validator.New()
	err := validate.Struct(c)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// TicketUrl creates the URL to a specific ticket.
func (c *Config) TicketUrl(ticketId string) (url string) {
	return c.Jira.JiraURL + "/browse/" + ticketId
}

// IsDoneStatus checks if the given status is considered a "done" status.
func (c *Config) IsDoneStatus(status string) bool {
	return slices.Contains(c.Jira.DoneStatuses, status)
}
