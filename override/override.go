// Package override merges hand-maintained per-issue JSON files into Jira issue data.
package override

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-burndown/config"
	"go-burndown/jira"

	"github.com/pkg/errors"
)

// issueKeyFilenameRE extracts a Jira-style key (PROJECT-123) from the start of a filename base.
// Optional descriptive suffix after a hyphen is allowed: PROJ-123-Big Ticket.json → PROJ-123.
var issueKeyFilenameRE = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*-\d+)(?:-.*)?$`)

// IssueFile is the on-disk shape for one issue overlay JSON file.
// Omitted or empty optional fields leave the Jira value unchanged.
type IssueFile struct {
	Summary  *string  `json:"summary"`
	Type     *string  `json:"type"`
	Assignee *string  `json:"assignee"`
	Size     *float64 `json:"size"`
	// Progress is interleaved into the issue changelog as percent-complete history.
	Progress []ProgressPoint `json:"progress"`
	// Statuses is interleaved into the changelog as status transitions on given dates.
	// A done status (per config done_statuses) also counts as 100% complete from that date.
	Statuses []StatusPoint `json:"statuses"`
}

// ProgressPoint is a known percent complete on a calendar date (YYYY-MM-DD).
// PercentComplete is 0.0–1.0 (same scale as Jira history in this tool).
type ProgressPoint struct {
	Date            string  `json:"date"`
	PercentComplete float64 `json:"percent_complete"`
}

// StatusPoint is a status value known on a calendar date (YYYY-MM-DD).
type StatusPoint struct {
	Date   string `json:"date"`
	Status string `json:"status"`
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

// issueKeyFromFilename returns the issue key from an overlay filename, if it follows
// {KEY}.json or {KEY}-*.json where KEY is PROJECT-123 style.
func issueKeyFromFilename(name string) (string, bool) {
	if !strings.HasSuffix(name, ".json") {
		return "", false
	}
	base := strings.TrimSuffix(name, ".json")
	m := issueKeyFilenameRE.FindStringSubmatch(base)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// matchesIssueOverrideFile reports whether name is an overlay for issueKey.
func matchesIssueOverrideFile(name, issueKey string) bool {
	key, ok := issueKeyFromFilename(name)
	return ok && key == issueKey
}

// LoadAll builds issues solely from overlay JSON files in dir (no Jira).
// Each file whose name starts with a PROJECT-123 style key becomes (or merges into)
// that issue. Multiple files for the same key are applied in sorted filename order.
// Files that do not match the naming convention are skipped.
func LoadAll(dir string, cfg *config.Config) ([]jira.Issue, error) {
	if dir == "" {
		return nil, errors.New("overrides directory is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "overrides directory %q", dir)
	}
	if !info.IsDir() {
		return nil, errors.Errorf("overrides path is not a directory: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "read overrides directory %s", dir)
	}

	// key → sorted file paths
	filesByKey := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		key, ok := issueKeyFromFilename(name)
		if !ok {
			continue
		}
		filesByKey[key] = append(filesByKey[key], filepath.Join(dir, name))
	}
	if len(filesByKey) == 0 {
		return nil, errors.Errorf("no issue overlay JSON files found in %s (want KEY.json or KEY-description.json)", dir)
	}

	keys := make([]string, 0, len(filesByKey))
	for key := range filesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	issues := make([]jira.Issue, 0, len(keys))
	for _, key := range keys {
		paths := filesByKey[key]
		sort.Strings(paths)
		issue := newBlankIssue(key)
		for _, path := range paths {
			if err := applyFile(path, &issue, cfg); err != nil {
				return nil, err
			}
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func newBlankIssue(key string) jira.Issue {
	return jira.Issue{
		Key: key,
		Fields: jira.Fields{
			Status: struct {
				Name string `json:"name"`
			}{Name: "To Do"},
			Issuetype: struct {
				Name string `json:"name"`
			}{Name: "Task"},
			CustomFields: make(map[string]interface{}),
		},
	}
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
	if err := applyStatuses(issue, overlay.Statuses); err != nil {
		return errors.Wrapf(err, "statuses in override %s", path)
	}
	sortHistories(issue)
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
	for _, point := range points {
		parsed, err := parseOverlayDate(point.Date, "progress")
		if err != nil {
			return err
		}
		if point.PercentComplete < 0 || point.PercentComplete > 1 {
			return errors.Errorf("progress percent_complete %v on %s must be between 0.0 and 1.0", point.PercentComplete, point.Date)
		}

		issue.Changelog.Histories = append(issue.Changelog.Histories, newHistory(
			parsed,
			cfg.Jira.PercentCompleteField,
			"local",
			strconv.FormatFloat(point.PercentComplete, 'g', -1, 64),
		))
	}
	return nil
}

func applyStatuses(issue *jira.Issue, points []StatusPoint) error {
	var latestStatus string
	var latestTime time.Time
	haveLatest := false

	for _, point := range points {
		status := strings.TrimSpace(point.Status)
		if status == "" {
			return errors.New("status entry missing status")
		}
		parsed, err := parseOverlayDate(point.Date, "status")
		if err != nil {
			return err
		}

		issue.Changelog.Histories = append(issue.Changelog.Histories, newHistory(
			parsed,
			"status",
			"local",
			status,
		))

		if !haveLatest || !parsed.Before(latestTime) {
			latestStatus = status
			latestTime = parsed
			haveLatest = true
		}
	}

	// Work sheet Status column reflects the latest local status transition when present.
	if haveLatest {
		issue.Fields.Status.Name = latestStatus
	}
	return nil
}

func parseOverlayDate(dateStr, kind string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, errors.Errorf("%s entry missing date", kind)
	}
	parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "%s date %q (want YYYY-MM-DD)", kind, dateStr)
	}
	return parsed, nil
}

func newHistory(when time.Time, field, fieldType, toString string) jira.History {
	return jira.History{
		Created: when.Format(jira.JIRARFC3339TimeLayout),
		Items: []struct {
			Field      string `json:"field"`
			Fieldtype  string `json:"fieldtype"`
			FromString string `json:"fromString"`
			ToString   string `json:"toString"`
		}{
			{
				Field:     field,
				Fieldtype: fieldType,
				ToString:  toString,
			},
		},
		CreatedTime: when,
	}
}

func sortHistories(issue *jira.Issue) {
	sort.Slice(issue.Changelog.Histories, func(i, j int) bool {
		return issue.Changelog.Histories[i].CreatedTime.Before(issue.Changelog.Histories[j].CreatedTime)
	})
}
