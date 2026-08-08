package override

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-burndown/config"
	"go-burndown/jira"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		OutputFile:     "out.xlsx",
		StartDate:      "2026-01-01",
		JQL:            "project = X",
		MovingAvgWeeks: 3,
		Jira: config.JiraConfig{
			JiraURL:              "https://example.atlassian.net",
			Username:             "u",
			APIToken:             "t",
			SizeField:            "customfield_10028",
			PercentCompleteField: "Percentage Complete",
			DoneStatuses:         []string{"Done"},
		},
	}
}

func parseDate(t *testing.T, date string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation("2006-01-02", date, time.Local)
	require.NoError(t, err)
	return tm
}

func baseIssue(key string) jira.Issue {
	return jira.Issue{
		Key: key,
		Fields: jira.Fields{
			Summary: "Jira summary",
			Status: struct {
				Name string `json:"name"`
			}{Name: "In Progress"},
			Issuetype: struct {
				Name string `json:"name"`
			}{Name: "Story"},
			Assignee: struct {
				DisplayName string `json:"displayName"`
			}{DisplayName: "Jira Person"},
			CustomFields: map[string]interface{}{
				"customfield_10028": float64(3),
			},
		},
		Changelog: struct {
			Histories []jira.History `json:"histories"`
		}{
			Histories: []jira.History{
				{
					Created: "2026-01-10T00:00:00.000+0000",
					Items: []struct {
						Field      string `json:"field"`
						Fieldtype  string `json:"fieldtype"`
						FromString string `json:"fromString"`
						ToString   string `json:"toString"`
					}{
						{Field: "Percentage Complete", Fieldtype: "custom", ToString: "0.2"},
					},
					CreatedTime: time.Date(2026, 1, 10, 0, 0, 0, 0, time.Local),
				},
			},
		},
	}
}

func TestApply_NoDirIsNoop(t *testing.T) {
	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply("", issues, testConfig()))
	assert.Equal(t, "Jira summary", issues[0].Fields.Summary)
}

func TestApply_MissingFileLeavesIssue(t *testing.T) {
	dir := t.TempDir()
	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply(dir, issues, testConfig()))
	assert.Equal(t, "Jira summary", issues[0].Fields.Summary)
	assert.Equal(t, float64(3), issues[0].GetSize(testConfig()))
}

func TestApply_PartialOverwriteAndProgressInterleave(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"summary": "Local summary",
		"size": 8,
		"progress": [
			{"date": "2026-01-05", "percent_complete": 0.1},
			{"date": "2026-01-20", "percent_complete": 0.5}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-1.json"), []byte(content), 0o644))

	cfg := testConfig()
	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply(dir, issues, cfg))

	assert.Equal(t, "Local summary", issues[0].Fields.Summary)
	assert.Equal(t, "Story", issues[0].GetType(), "type not in overlay")
	assert.Equal(t, "Jira Person", issues[0].Fields.Assignee.DisplayName, "assignee not in overlay")
	assert.Equal(t, float64(8), issues[0].GetSize(cfg))

	require.Len(t, issues[0].Changelog.Histories, 3)
	assert.True(t, issues[0].Changelog.Histories[0].CreatedTime.Before(issues[0].Changelog.Histories[1].CreatedTime))
	assert.True(t, issues[0].Changelog.Histories[1].CreatedTime.Before(issues[0].Changelog.Histories[2].CreatedTime))

	pEarly, err := issues[0].PercentCompleteOnDate(cfg, parseDate(t, "2026-01-06"))
	require.NoError(t, err)
	assert.InDelta(t, 0.1, pEarly, 1e-9)

	pMid, err := issues[0].PercentCompleteOnDate(cfg, parseDate(t, "2026-01-10"))
	require.NoError(t, err)
	assert.InDelta(t, 0.2, pMid, 1e-9)

	pLate, err := issues[0].PercentCompleteOnDate(cfg, parseDate(t, "2026-01-20"))
	require.NoError(t, err)
	assert.InDelta(t, 0.5, pLate, 1e-9)
}

func TestApply_EmptyStringDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	content := `{"summary": "  ", "type": "", "assignee": ""}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-1.json"), []byte(content), 0o644))

	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply(dir, issues, testConfig()))
	assert.Equal(t, "Jira summary", issues[0].Fields.Summary)
	assert.Equal(t, "Story", issues[0].GetType())
	assert.Equal(t, "Jira Person", issues[0].Fields.Assignee.DisplayName)
}

func TestApply_InvalidPercent(t *testing.T) {
	dir := t.TempDir()
	content := `{"progress": [{"date": "2026-01-01", "percent_complete": 1.5}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-1.json"), []byte(content), 0o644))
	err := Apply(dir, []jira.Issue{baseIssue("PROJ-1")}, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percent_complete")
}

func TestApply_FullFieldOverwrite(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"summary": "S",
		"type": "Bug",
		"assignee": "Pat",
		"size": 13
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-2.json"), []byte(content), 0o644))
	cfg := testConfig()
	issues := []jira.Issue{baseIssue("PROJ-2")}
	require.NoError(t, Apply(dir, issues, cfg))
	assert.Equal(t, "S", issues[0].Fields.Summary)
	assert.Equal(t, "Bug", issues[0].GetType())
	assert.Equal(t, "Pat", issues[0].Fields.Assignee.DisplayName)
	assert.Equal(t, float64(13), issues[0].GetSize(cfg))
}

func TestApply_StatusHistoryInterleaved(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"statuses": [
			{"date": "2026-01-05", "status": "To Do"},
			{"date": "2026-01-12", "status": "In Progress"},
			{"date": "2026-01-25", "status": "Done"}
		],
		"progress": [
			{"date": "2026-01-12", "percent_complete": 0.3}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-1.json"), []byte(content), 0o644))

	cfg := testConfig()
	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply(dir, issues, cfg))

	// Latest status wins for the Work sheet Status column.
	assert.Equal(t, "Done", issues[0].GetStatus())

	// Done on 2026-01-25 → 100% from that date (even if earlier percent was lower).
	pDone, err := issues[0].PercentCompleteOnDate(cfg, parseDate(t, "2026-01-25"))
	require.NoError(t, err)
	assert.InDelta(t, 1.0, pDone, 1e-9)

	// Before Done, local percent 0.3 on 01-12 and Jira 0.2 on 01-10 → 0.3 by 01-15.
	pMid, err := issues[0].PercentCompleteOnDate(cfg, parseDate(t, "2026-01-15"))
	require.NoError(t, err)
	assert.InDelta(t, 0.3, pMid, 1e-9)

	// Status history entries present and sorted with other history.
	require.GreaterOrEqual(t, len(issues[0].Changelog.Histories), 4)
	for i := 1; i < len(issues[0].Changelog.Histories); i++ {
		assert.False(t, issues[0].Changelog.Histories[i].CreatedTime.Before(issues[0].Changelog.Histories[i-1].CreatedTime))
	}
}

func TestApply_DescriptiveFilenameSuffix(t *testing.T) {
	dir := t.TempDir()
	content := `{"summary": "From descriptive name", "size": 8}`
	// Spaces and descriptive text after KEY- are allowed.
	name := "PROJ-1-Big Ticket To Do.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))

	cfg := testConfig()
	issues := []jira.Issue{baseIssue("PROJ-1")}
	require.NoError(t, Apply(dir, issues, cfg))
	assert.Equal(t, "From descriptive name", issues[0].Fields.Summary)
	assert.Equal(t, float64(8), issues[0].GetSize(cfg))
}

func TestApply_DoesNotMatchLongerIssueKeyPrefix(t *testing.T) {
	dir := t.TempDir()
	// PROJ-10 must not pick up PROJ-100.json or PROJ-100-foo.json
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-100.json"), []byte(`{"summary":"wrong"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROJ-100-extra.json"), []byte(`{"summary":"also wrong"}`), 0o644))

	issues := []jira.Issue{baseIssue("PROJ-10")}
	require.NoError(t, Apply(dir, issues, testConfig()))
	assert.Equal(t, "Jira summary", issues[0].Fields.Summary)
}

func TestMatchesIssueOverrideFile(t *testing.T) {
	tests := []struct {
		name     string
		issueKey string
		want     bool
	}{
		{name: "PROJ-123.json", issueKey: "PROJ-123", want: true},
		{name: "PROJ-123-Big Ticket To Do.json", issueKey: "PROJ-123", want: true},
		{name: "PROJ-123-notes.json", issueKey: "PROJ-123", want: true},
		{name: "PROJ-1230.json", issueKey: "PROJ-123", want: false},
		{name: "PROJ-12.json", issueKey: "PROJ-123", want: false},
		{name: "PROJ-123.txt", issueKey: "PROJ-123", want: false},
		{name: "other.json", issueKey: "PROJ-123", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, matchesIssueOverrideFile(tc.name, tc.issueKey))
		})
	}
}
