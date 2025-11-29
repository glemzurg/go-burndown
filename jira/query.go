package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go-burndown/config"

	"github.com/pkg/errors"
)

const (
	// When querying jira, how many results to request per page.
	_RESULTS_PER_PAGE = 100
)

// Response represents the response from Jira search API.
type Response struct {
	Issues []Issue `json:"issues"`
}

// QueryJira queries Jira using the provided configuration and returns the list of issues.
func QueryJira(ctx context.Context, config *config.Config) ([]Issue, error) {
	// Create HTTP client
	client := &http.Client{}

	// Encode credentials
	auth := base64.StdEncoding.EncodeToString([]byte(config.Jira.Username + ":" + config.Jira.APIToken))

	// Fetch all issues with pagination
	var allIssues []Issue
	startAt := 0
	maxResults := _RESULTS_PER_PAGE

	for {
		encodedJQL := url.QueryEscape(config.JQL)
		searchURL := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&startAt=%d&maxResults=%d&fields=key", config.Jira.JiraURL, encodedJQL, startAt, maxResults)

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, http.NoBody)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		req.Header.Set("Authorization", "Basic "+auth)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if resp.StatusCode != 200 {
			return nil, errors.Errorf("Jira search API returned status %d: %s", resp.StatusCode, string(body))
		}

		var searchResp Response
		err = json.Unmarshal(body, &searchResp)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// Fetch full details for each issue
		for i := range searchResp.Issues {
			issue := &searchResp.Issues[i]
			issueDetails, err := getIssueDetails(ctx, client, auth, config.Jira.JiraURL, issue.Key)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			allIssues = append(allIssues, *issueDetails)
		}

		// If this last query returned fewer than maxResults, we're done, there are no more issues.
		if len(searchResp.Issues) < maxResults {
			break
		}
		// Still here? Move the starting point for the next query.
		startAt += maxResults

		// Simple rate limiting: sleep 1 second between requests
		time.Sleep(time.Second)
	}

	return allIssues, nil
}
