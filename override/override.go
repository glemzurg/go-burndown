// Package override merges hand-maintained per-issue JSON files into Jira issue data.
package override

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-burndown/config"
	"go-burndown/jira"

	"github.com/pkg/errors"
)

// IssueFile is the on-disk shape for one issue overlay JSON file.
// Omitted or empty optional fields leave the Jira value unchanged.
type IssueFile struct {
	Summary  *string  `json:"summary"`
	Type     *string  `json:"type"`
	Assignee *string  `json:"assignee"`
	Size     *float64 `json:"size"`
	// Progress is interleaved into the issue changelog as percent-complete history.
	Progress []ProgressPoint `json:"progress"`
}

// ProgressPoint is a known percent complete on a calendar date (YYYY-MM-DD).
// PercentComplete is 0.0–1.0 (same scale as Jira history in this tool).
type ProgressPoint struct {
	Date            string  `json:"date"`
	PercentComplete float64 `json:"percent_complete"`
}

// Apply merges local JSON overlays from dir into issues matched by issue key.
// Matching filenames (case-sensitive to the issue key):
//   - {ISSUE_KEY}.json
//   - {ISSUE_KEY}-*.json  (descriptive suffix after a hyphen)
//
// Examples for PROJ-123: PROJ-123.json, PROJ-123-Big Ticket To Do.json
// Non-matching prefixes (e.g. PROJ-1230.json for key PROJ-123) are ignored.
// Multiple matches for one key are applied in sorted filename order.
// dir empty is a no-op.
func Apply(dir string, issues []jira.Issue, cfg *config.Config) error {
	if dir == "" {
		return nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		return errors.Wrapf(err, "overrides directory %q", dir)
	}
	if !info.IsDir() {
		return errors.Errorf("overrides path is not a directory: %s", dir)
	}

	for i := range issues {
		if err := applyOne(dir, &issues[i], cfg); err != nil {
			return err
		}
	}
	return nil
}

// matchesIssueOverrideFile reports whether name is an overlay for issueKey.
func matchesIssueOverrideFile(name, issueKey string) bool {
	if !strings.HasSuffix(name, ".json") {
		return false
	}
	base := strings.TrimSuffix(name, ".json")
	return base == issueKey || strings.HasPrefix(base, issueKey+"-")
}

func listOverrideFiles(dir, issueKey string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "read overrides directory %s", dir)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if matchesIssueOverrideFile(name, issueKey) {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func applyOne(dir string, issue *jira.Issue, cfg *config.Config) error {
	paths, err := listOverrideFiles(dir, issue.Key)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := applyFile(path, issue, cfg); err != nil {
			return err
		}
	}
	return nil
}

func applyFile(path string, issue *jira.Issue, cfg *config.Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "read override %s", path)
	}

	var overlay IssueFile
	if err := json.Unmarshal(data, &overlay); err != nil {
		return errors.Wrapf(err, "parse override %s", path)
	}

	applyFields(issue, overlay, cfg)
	if err := applyProgress(issue, overlay.Progress, cfg); err != nil {
		return errors.Wrapf(err, "progress in override %s", path)
	}
	return nil
}

// applyFields overwrites Jira fields only when the overlay supplies a non-empty value.
func applyFields(issue *jira.Issue, overlay IssueFile, cfg *config.Config) {
	if overlay.Summary != nil && strings.TrimSpace(*overlay.Summary) != "" {
		issue.Fields.Summary = *overlay.Summary
	}
	if overlay.Type != nil && strings.TrimSpace(*overlay.Type) != "" {
		issue.Fields.Issuetype.Name = *overlay.Type
	}
	if overlay.Assignee != nil && strings.TrimSpace(*overlay.Assignee) != "" {
		issue.Fields.Assignee.DisplayName = *overlay.Assignee
	}
	if overlay.Size != nil {
		if issue.Fields.CustomFields == nil {
			issue.Fields.CustomFields = make(map[string]interface{})
		}
		issue.Fields.CustomFields[cfg.Jira.SizeField] = *overlay.Size
	}
}

func applyProgress(issue *jira.Issue, points []ProgressPoint, cfg *config.Config) error {
	if len(points) == 0 {
		return nil
	}

	for _, point := range points {
		dateStr := strings.TrimSpace(point.Date)
		if dateStr == "" {
			return errors.New("progress entry missing date")
		}
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			return errors.Wrapf(err, "progress date %q (want YYYY-MM-DD)", dateStr)
		}
		if point.PercentComplete < 0 || point.PercentComplete > 1 {
			return errors.Errorf("progress percent_complete %v on %s must be between 0.0 and 1.0", point.PercentComplete, dateStr)
		}

		issue.Changelog.Histories = append(issue.Changelog.Histories, jira.History{
			Created: parsed.Format(jira.JIRARFC3339TimeLayout),
			Items: []struct {
				Field      string `json:"field"`
				Fieldtype  string `json:"fieldtype"`
				FromString string `json:"fromString"`
				ToString   string `json:"toString"`
			}{
				{
					Field:     cfg.Jira.PercentCompleteField,
					Fieldtype: "local",
					ToString:  strconv.FormatFloat(point.PercentComplete, 'g', -1, 64),
				},
			},
			CreatedTime: parsed,
		})
	}

	sort.Slice(issue.Changelog.Histories, func(i, j int) bool {
		return issue.Changelog.Histories[i].CreatedTime.Before(issue.Changelog.Histories[j].CreatedTime)
	})
	return nil
}
