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
	OutputFile     string `json:"output_file" validate:"required"`
	StartDate      string `json:"start_date" validate:"required,datetime=2006-01-02"`
	JQL            string `json:"jql" validate:"required"`
	MovingAvgWeeks uint   `json:"moving_avg_weeks" validate:"required"`
	// OverridesDir is an optional folder of hand-maintained issue JSON overlays
	// ({ISSUE_KEY}.json or {ISSUE_KEY}-*.json), applied after Jira fetch and before Excel generation.
	OverridesDir string     `json:"overrides_dir"`
	Jira         JiraConfig `json:"jira" validate:"required"`
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

// Validate checks that the configuration has all required fields and valid values for Jira mode.
func (c *Config) Validate() error {
	validate := validator.New()
	err := validate.Struct(c)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// ValidateFromOverrides checks fields needed when issues come only from overrides_dir (no Jira API).
// Requires a config file with report settings and field names, plus a non-empty overrides_dir.
// Does not require JQL or Jira credentials.
func (c *Config) ValidateFromOverrides() error {
	if c.OutputFile == "" {
		return errors.New("output_file is required")
	}
	if c.StartDate == "" {
		return errors.New("start_date is required")
	}
	validate := validator.New()
	if err := validate.Var(c.StartDate, "datetime=2006-01-02"); err != nil {
		return errors.Wrap(err, "start_date")
	}
	if c.MovingAvgWeeks == 0 {
		return errors.New("moving_avg_weeks is required")
	}
	if c.OverridesDir == "" {
		return errors.New("overrides_dir is required (or pass --overrides-dir)")
	}
	if c.Jira.SizeField == "" {
		return errors.New("jira.size_field is required")
	}
	if c.Jira.PercentCompleteField == "" {
		return errors.New("jira.percent_complete_field is required")
	}
	if len(c.Jira.DoneStatuses) == 0 {
		return errors.New("jira.done_statuses is required")
	}
	return nil
}

// TicketUrl creates the URL to a specific ticket.
//
//revive:disable:var-naming
func (c *Config) TicketUrl(ticketId string) (url string) {
	return c.Jira.JiraURL + "/browse/" + ticketId
}

// IsDoneStatus checks if the given status is considered a "done" status.
func (c *Config) IsDoneStatus(status string) bool {
	return slices.Contains(c.Jira.DoneStatuses, status)
}
