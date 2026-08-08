# Jira Burndown Report Generator

A Go command-line tool that queries Jira for issues and generates an Excel spreadsheet with burndown charts, earned value calculations, and project completion projections.

## Features

- **Jira Integration**: Queries Jira using JQL to fetch project issues with full history and changelogs
- **Local issue overlays**: Optional hand-maintained JSON files per issue key to fill gaps or correct Jira fields and progress history
- **Issue History Analysis**: Analyzes complete changelog for each issue to track status changes, percent complete updates, and completion dates
- **Excel Export**: Creates a two-sheet Excel workbook:
  - **Work Sheet**: Lists all Jira tickets with details (key, summary, type, status, assignee, size) and weekly progress data
  - **Projections Sheet**: Shows weekly burndown progress with earned value, velocity calculations, and completion date projections
- **Accurate Progress Tracking**: Calculates percent complete based on configurable fields and history, with non-decreasing progress
- **Flexible Configuration**: Supports configuration files with optional command-line overrides
- **Project Completion Forecasting**: Predicts completion dates using moving averages and statistical projections

## Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```
3. Build the application:
   ```bash
   go build -o burndown .
   ```

## Configuration

Create a `config.json` file or use command-line parameters:

### Configuration File (config.json)
```json
{
  "output_file": "burndown.xlsx",
  "start_date": "2025-01-01",
  "jql": "project = \"YOUR_PROJECT\" AND type = Story",
  "moving_avg_weeks": 12,
  "overrides_dir": "overrides",
  "jira": {
    "jira_url": "https://yourcompany.atlassian.net",
    "username": "your.email@company.com",
    "api_token": "your_jira_api_token_here",
    "size_field": "customfield_10016",
    "percent_complete_field": "Percentage Complete",
    "done_statuses": ["Done", "Closed", "Resolved", "Complete", "Completed"]
  }
}
```

`overrides_dir` is optional. When set (or when `--overrides-dir` is passed), local JSON files in that folder are merged after Jira data is loaded.

### Jira API Token Setup

1. Go to your Jira account settings
2. Navigate to Security → Create and manage API tokens
3. Create a new API token
4. Use your email as username and the token as api_token

## Usage

There are **three ways** to generate a burndown:

| Mode | Flag(s) | Data source | Jira? |
|------|---------|-------------|-------|
| **Jira** | (default, needs `config.json`) | Jira JQL, then optional local files | Yes |
| **Overrides only** | `--from-overrides --overrides-dir=…` | Local JSON files only (keys from filenames) | No |
| **Example** | `--example` | Built-in mock tickets (+ optional overlays) | No |

### Build
```bash
go build -o build/burndown ./cmd/burndown
```

### 1. Jira mode (optional local overlays)
Requires `config.json` with Jira credentials and JQL:

```bash
./build/burndown
./build/burndown --overrides-dir=overrides
./build/burndown --config=custom.json --jql='project = MY_PROJECT' --output=report.xlsx
```

### 2. Overrides-only mode (no Jira)
Treat a folder of hand-maintained JSON files as the full issue database. Issue keys come from the **start of each filename** (`PROJ-123.json` or `PROJ-123-description.json`).

```bash
./build/burndown --from-overrides --overrides-dir=example/overrides
./build/burndown --from-overrides --overrides-dir=overrides --start-date=2026-06-01 --output=local.xlsx
```

No Jira credentials or network access. Optional `config.json` may still supply `start_date`, `moving_avg_weeks`, field names, and `done_statuses` if present; flags win when set. `--overrides-dir` (or `overrides_dir` in config) is required.

### 3. Example mode
Built-in mock project data (no config file or Jira):

```bash
./build/burndown --example
./build/burndown --example --output=demo.xlsx --start-date=2026-01-06
./build/burndown --example --overrides-dir=example/overrides
```

The demo timeline starts six weeks before the last Tuesday on or before today, with fictional weekly progress history.

### Command Line Options

```bash
./burndown --config="custom.json" --jql="project = MY_PROJECT" --output="report.xlsx" --start-date="2025-01-01" --overrides-dir=overrides
```

Available flags:
- `--config`: Path to configuration file (default: `config.json` in Jira mode; optional offline merge for `--from-overrides`)
- `--jql`: JQL query to fetch issues (Jira mode only)
- `--output`: Output Excel file path
- `--start-date`: Project start date in YYYY-MM-DD format
- `--overrides-dir`: Folder of issue JSON files (overlay in Jira/example modes; **source of truth** with `--from-overrides`)
- `--from-overrides`: Load issues only from `--overrides-dir` (no Jira)
- `--example`: Built-in mock data (no config file or Jira; mutually exclusive with `--from-overrides`)

## Pipelines

### Jira (+ optional overlays)
1. **Fetch from Jira** — JQL loads issues with changelog history.
2. **Apply local overlays** — Merge matching files from `overrides_dir` when set.
3. **Generate Excel**.

### Overrides only (`--from-overrides`)
1. **Load all matching JSON files** from `overrides_dir` as issues (key = filename prefix).
2. **Generate Excel** (same engine as Jira mode).

### Example (`--example`)
1. **Build mock issues** in process.
2. **Optional overlays** from `overrides_dir`.
3. **Generate Excel**.

### Local issue files

| Rule | Detail |
|------|--------|
| Location | Directory from `overrides_dir` / `--overrides-dir` |
| Filename | `{ISSUE_KEY}.json` **or** `{ISSUE_KEY}-*.json` (optional descriptive suffix after a hyphen) |
| Issue key | Leading `PROJECT-123` style id at the start of the filename (`TICKET-1236-Big work stuff.json` → `TICKET-1236`) |
| Examples | `PROJ-123.json`, `PROJ-123-Big Ticket To Do.json` |
| Matching (Jira/example overlay) | File applies only to that issue key; `PROJ-1230.json` does not match `PROJ-123` |
| Multiple files | Same key: applied in sorted filename order |
| Orphan files (Jira/example) | JSON for keys not in the issue set is ignored |
| Overrides-only mode | Every matching file becomes an issue; non-matching names (e.g. `notes.json`) are skipped |

### Overlay JSON shape

All fields are optional. **Omitted keys or empty strings leave the Jira value unchanged.** Present non-empty values overwrite. `size` overwrites when the key is present (including `0`).

```json
{
  "summary": "Optional replacement for Summary",
  "type": "Optional replacement for Type (issue type)",
  "assignee": "Optional replacement for Assignee display name",
  "size": 5,
  "progress": [
    {
      "date": "2026-03-15",
      "percent_complete": 0.4
    },
    {
      "date": "2026-03-22",
      "percent_complete": 0.75
    }
  ],
  "statuses": [
    {
      "date": "2026-03-01",
      "status": "To Do"
    },
    {
      "date": "2026-03-15",
      "status": "In Progress"
    },
    {
      "date": "2026-04-01",
      "status": "Done"
    }
  ]
}
```

| Field | Work column | Behavior |
|-------|-------------|----------|
| `summary` | Summary | Overwrite if non-empty string |
| `type` | Type | Overwrite if non-empty string |
| `assignee` | Assignee | Overwrite if non-empty string |
| `size` | Size | Overwrite when key is present (uses configured `size_field`) |
| `progress` | weekly % / EV | Each point is **interleaved** into changelog history as an extra percent-complete sample |
| `statuses` | Status (+ weekly % if done) | Each point is a **timestamped status change** interleaved into changelog history |

### Progress and status interleaving

- `date` is `YYYY-MM-DD` for both `progress` and `statuses`.
- `percent_complete` is **0.0–1.0** (same scale as the rest of the tool).
- Local progress and status points are appended to the issue’s history and sorted by time with Jira changelog entries.
- Weekly percent complete still uses the existing rule: the maximum percent known on or before that week’s date (so local and Jira points combine; progress never decreases from a later lower sample).
- A local status that appears in config `done_statuses` (e.g. `Done`) counts as **100% complete** from that date forward (same as a Jira status change to Done).
- The Work sheet **Status** column is set to the **latest** overlay status by date when `statuses` is present.

Use overlays when Jira is missing percent-complete history, size is wrong, or you want to record offline progress and status without editing Jira.

Sample file: `example/overrides/TICKET-1236-Big work stuff.json`.

## Excel Output

### Work Sheet
Contains one row per Jira issue with columns:
- Issue Key (hyperlinked to Jira)
- Summary
- Type (issue type)
- Status
- Assignee
- Size
- Weekly progress data: % Complete and Earned Value for each week (newest to oldest)

### Projections Sheet
Shows weekly project progress and forecasts with columns:
- Date
- Completed (cumulative earned value)
- Remaining (total size minus completed)
- Velocity (weekly earned value)
- Avg (12w) (moving average velocity)
- StdDev (12w) (standard deviation of velocity)
- Fast (p68), Mean, Slow (p68) (projected completion dates based on velocity percentiles)
- V. Fast (p68), V. Slow (p68) (standard deviation computations)

## JQL Examples

```sql
-- All stories in a project
project = "MY_PROJECT" AND type = Story

-- Stories in current sprint
project = "MY_PROJECT" AND type = Story AND sprint in openSprints()

-- Stories with specific labels
project = "MY_PROJECT" AND type = Story AND labels = "backend"

-- Stories assigned to team
project = "MY_PROJECT" AND type = Story AND assignee in (user1, user2, user3)
```

## Notes

- **History-Based Progress**: Percent complete is calculated from Jira changelog history (plus local overlay progress), ensuring accuracy and preventing decreases
- **Local overlays**: Optional per-key JSON after Jira fetch; see [Pipeline: Jira → local overlays → Excel](#pipeline-jira--local-overlays--excel)
- **Configurable Fields**: Size and percent complete fields are configurable custom fields
- **Done Statuses**: Configurable list of statuses that mark issues as completed
- **Pagination Support**: Handles large result sets with automatic pagination
- **Rate Limiting**: 1-second delays between API requests to respect Jira rate limits
- **Weekly Reporting**: Progress is tracked and projected on a weekly basis
- **Statistical Projections**: Uses moving averages and standard deviations for completion forecasts

## Dependencies

- `github.com/pkg/errors`: Error handling with stack traces and error wrapping
- `github.com/xuri/excelize/v2`: Excel file generation
- Standard Go libraries for HTTP, JSON, and time handling

## Error Handling

The application uses the `pkg/errors` library for comprehensive error reporting:

### Error Details Access Methods

1. **Stack Trace with %+v**: `fmt.Printf("%+v", err)` - Shows full stack trace

3. **Sentry Integration**: Automatic Sentry error reporting support

### Error Logging Format

When errors occur, they are logged with:
- **Full Stack Trace**: Complete call stack for debugging (`%+v`)
- **Safe Details**: Sanitized error information for external reporting
- **Context Preservation**: Error wrapping maintains the complete error chain

### Example Error Output

```
Configuration error: missing required configuration: jira_url, username, api_token, and jql are required
main.main()
	/home/user/project/main.go:95
runtime.main()
	/usr/local/go/src/runtime/proc.go:250
Safe details: missing required configuration
```

The stack trace is considered safe for reporting and provides detailed debugging information.

## API Compatibility

This tool uses Jira REST API v3 (`/rest/api/3/search/jql`). If you encounter API compatibility issues, ensure your Jira instance supports API v3.
