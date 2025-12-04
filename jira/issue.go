package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go-burndown/config"

	"github.com/pkg/errors"
)

// Issue represents a Jira issue with its fields and changelog.
type Issue struct {
	Key       string `json:"key"`
	Fields    Fields `json:"fields"`
	Changelog struct {
		Histories []History `json:"histories"`
	} `json:"changelog"`
}

func getIssueDetails(ctx context.Context, client *http.Client, auth, jiraURL, issueKey string) (*Issue, error) {
	// Build URL for individual issue with changelog
	issueURL := fmt.Sprintf("%s/rest/api/3/issue/%s?expand=changelog", jiraURL, issueKey)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", issueURL, http.NoBody)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Set headers
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("Jira issue API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var issue Issue
	err = json.Unmarshal(body, &issue)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Parse history times
	err = issue.parseHistoryTimes()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &issue, nil
}

// GetStatus retrieves the status of the ticket.
func (issue *Issue) GetStatus() string {
	return issue.Fields.Status.Name
}

// GetType retrieves the type of the ticket.
func (issue *Issue) GetType() string {
	return issue.Fields.Issuetype.Name
}

// GetSize retrieves size using configurable field ID.
func (issue *Issue) GetSize(config *config.Config) float64 {
	if val, ok := issue.Fields.CustomFields[config.Jira.SizeField]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}

// GetPercentageComplete retrieves percentage complete using configurable field ID.
func (issue *Issue) GetPercentageComplete(config *config.Config) float64 {
	if val, ok := issue.Fields.CustomFields[config.Jira.PercentCompleteField]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}
