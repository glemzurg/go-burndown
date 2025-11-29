# Jira Burndown Report Generator

A Go command-line tool that queries Jira for issues and generates an Excel spreadsheet with burndown charts, earned value calculations, and project completion projections.

## Features

- **Jira Integration**: Queries Jira using JQL to fetch project issues with full history and changelogs
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

### Jira API Token Setup

1. Go to your Jira account settings
2. Navigate to Security → Create and manage API tokens
3. Create a new API token
4. Use your email as username and the token as api_token

## Usage

### Basic Usage (with config.json)
```bash
./burndown
```

### Build and Run
```bash
go build -o build/burndown ./cmd/burndown
./build/burndown
```

### Command Line Options
You can override configuration file settings with command-line flags:

```bash
./burndown --config="custom.json" --jql="project = MY_PROJECT" --output="report.xlsx" --start-date="2025-01-01"
```

Available flags:
- `--config`: Path to configuration file (default: "config.json")
- `--jql`: JQL query to fetch issues (overrides config)
- `--output`: Output Excel file path (overrides config)
- `--start-date`: Project start date in YYYY-MM-DD format (overrides config)

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

- **History-Based Progress**: Percent complete is calculated from Jira changelog history, ensuring accuracy and preventing decreases
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
