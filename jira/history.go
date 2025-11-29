// Package jira provides functionality for querying Jira issues and analyzing their data.
package jira

import (
	"math"
	"sort"
	"strconv"
	"time"

	"go-burndown/config"

	"github.com/pkg/errors"
)

const (
	_JIRA_RFC3339_TIME_LAYOUT = `2006-01-02T15:04:05.000-0700`
)

// History represents a changelog entry in Jira.
type History struct {
	Created string `json:"created"`
	Items   []struct {
		Field      string `json:"field"`
		Fieldtype  string `json:"fieldtype"`
		FromString string `json:"fromString"`
		ToString   string `json:"toString"`
	} `json:"items"`
	// Internal private members.
	createdTime time.Time
}

func (issue *Issue) parseHistoryTimes() (err error) {

	// Gater updated histories with a parsed times.
	var updatedHistories []History
	for _, history := range issue.Changelog.Histories {
		createdTime, err := time.Parse(_JIRA_RFC3339_TIME_LAYOUT, history.Created)
		if err != nil {
			return errors.WithStack(err)
		}
		history.createdTime = createdTime
		updatedHistories = append(updatedHistories, history)
	}
	issue.Changelog.Histories = updatedHistories

	// Ensure the histories are sorted by date.
	sort.Slice(issue.Changelog.Histories, func(i, j int) bool {
		timeI := issue.Changelog.Histories[i].createdTime
		timeJ := issue.Changelog.Histories[j].createdTime
		return timeI.Before(timeJ)
	})

	return nil
}

// PercentCompleteOnDate returns the percent complete for an issue at a given date. Percent complete is 0.0 (0%) to 1.0 (100%).
func (issue *Issue) PercentCompleteOnDate(config *config.Config, date time.Time) (percentComplete float64, err error) {

	// Build up the percent to this date.
	percentComplete = 0.0
	// We want to capture everything that happens on that date or before.
	// To do that we should be less than the moment the next day begins.
	beginningOfNextDay := date.AddDate(0, 0, 1)
	for _, history := range issue.Changelog.Histories {
		historyTime := history.createdTime
		if historyTime.Before(beginningOfNextDay) {
			for _, item := range history.Items {

				switch item.Field {

				case config.Jira.PercentCompleteField:
					val, err := strconv.ParseFloat(item.ToString, 64)
					if err != nil {
						return 0.0, errors.WithStack(err)
					}
					percentComplete = math.Max(percentComplete, val)

				case "status":
					if config.IsDoneStatus(item.ToString) {
						percentComplete = 1.0
					}
				}
			}
		}
	}

	// Clamp to 0-100 in case a bad value was set.
	if percentComplete < 0.0 {
		percentComplete = 0.0
	}
	if percentComplete > 1.0 {
		percentComplete = 1.0
	}

	return percentComplete, nil
}
