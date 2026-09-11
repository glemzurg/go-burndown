// Package excel provides functionality for generating Excel reports from Jira data.
package excel

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/xuri/excelize/v2"

	"go-burndown/config"
	"go-burndown/jira"
)

// GenerateExcelReport creates an Excel report from Jira issues and saves it to a file.
func GenerateExcelReport(config *config.Config, issues []jira.Issue) error {
	// Create a new Excel file
	f := excelize.NewFile()

	// Create first sheet: Issues with weekly progress data
	workSheet := "Work"
	if _, err := f.NewSheet(workSheet); err != nil {
		return errors.Wrap(err, "failed to create work sheet")
	}

	// Calculate start date
	startDate, err := time.Parse("2006-01-02", config.StartDate)
	if err != nil {
		return errors.Wrapf(err, "invalid start date format: %s", config.StartDate)
	}

	// Generate weekly dates: oldest on the right, newest on the left
	currentDate := time.Now()
	weeks := []time.Time{}

	// Calculate complete weeks from start date to current date
	for d := startDate; d.Before(currentDate) || d.Equal(currentDate); d = d.AddDate(0, 0, 7) {
		weeks = append(weeks, d)
	}

	// Create reversed weeks for Work sheet columns (oldest on right)
	reversedWeeks := make([]time.Time, len(weeks))
	copy(reversedWeeks, weeks)
	for i, j := 0, len(reversedWeeks)-1; i < j; i, j = i+1, j-1 {
		reversedWeeks[i], reversedWeeks[j] = reversedWeeks[j], reversedWeeks[i]
	}

	// Hyperlink style (blue, underlined).
	hyperlinkStyleID, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Color:     "0000FF", // Blue color.
			Underline: "single",
		},
	})
	if err != nil {
		return err
	}

	// Create percentage style
	percentFmt := "0%"
	percentStyleID, err := f.NewStyle(&excelize.Style{CustomNumFmt: &percentFmt})
	if err != nil {
		return err
	}

	// Create date style
	dateFmt := "yyyy-mm-dd"
	dateStyleID, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	if err != nil {
		return err
	}

	// Create number style for one decimal place
	numFmt := "0.0"
	numStyleID, err := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})
	if err != nil {
		return err
	}

	// Create headers: Issue Key, Summary, Type, Status, Assignee, Size, then weekly pairs
	headers := []string{"Issue Key", "Summary", "Type", "Status", "Assignee", "Size"}

	// Add weekly headers (oldest on right, newest on left) - compact format
	for _, weekDate := range reversedWeeks {
		dateStr := weekDate.Format("01-02")
		headers = append(headers, fmt.Sprintf("%% %s", dateStr), fmt.Sprintf("EV %s", dateStr))
	}

	// Set headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(workSheet, cell, header); err != nil {
			return errors.WithStack(err)
		}
	}

	// Add issues data
	for i := range issues {
		issue := &issues[i]
		rowNum := i + 2

		// The work ticket id.
		idCell := fmt.Sprintf("A%d", rowNum)
		if err := f.SetCellValue(workSheet, idCell, issue.Key); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellHyperLink(workSheet, idCell, config.TicketUrl(issue.Key), "External", excelize.HyperlinkOpts{
			Display: &issue.Key,
		}); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellStyle(workSheet, idCell, idCell, hyperlinkStyleID); err != nil {
			return errors.WithStack(err)
		}

		// Other work ticket details.
		if err := f.SetCellValue(workSheet, fmt.Sprintf("B%d", rowNum), issue.Fields.Summary); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellValue(workSheet, fmt.Sprintf("C%d", rowNum), issue.GetType()); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellValue(workSheet, fmt.Sprintf("D%d", rowNum), issue.GetStatus()); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellValue(workSheet, fmt.Sprintf("E%d", rowNum), issue.Fields.Assignee.DisplayName); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellValue(workSheet, fmt.Sprintf("F%d", rowNum), issue.GetSize(config)); err != nil {
			return errors.WithStack(err)
		}

		// Weekly data - loop over reversedWeeks to match header order
		col := 7 // Start after Size column (F); CoordinatesToCellName supports AA+ columns
		for _, weekDate := range reversedWeeks {
			// Get percent complete for this issue at this week date
			percentComplete, err := issue.PercentCompleteOnDate(config, weekDate)
			if err != nil {
				return errors.WithStack(err)
			}

			// Set percent complete value (as fraction for Excel)
			// Leave field blank is percent complete is zero.
			percentCell, err := excelize.CoordinatesToCellName(col, rowNum)
			if err != nil {
				return errors.WithStack(err)
			}
			if percentComplete > 0 {
				if err := f.SetCellValue(workSheet, percentCell, percentComplete); err != nil { // 0.0-1.0
					return errors.WithStack(err)
				}
			}
			if err := f.SetCellStyle(workSheet, percentCell, percentCell, percentStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Earned Value formula: percent * size, blank if percent is zero for easy display.
			earnedCell, err := excelize.CoordinatesToCellName(col+1, rowNum)
			if err != nil {
				return errors.WithStack(err)
			}
			// Find the value in the row that is under the Size column and then multiply that by percent complete.
			earnedFormula := fmt.Sprintf(`=IF(%s=0, "", %s * HLOOKUP("Size", 1:%d, %d, 0))`, percentCell, percentCell, rowNum, rowNum)
			if err := f.SetCellFormula(workSheet, earnedCell, earnedFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(workSheet, earnedCell, earnedCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}

			col += 2
		}
	}

	if err := writeProjectionsSheet(f, config, weeks, numStyleID, dateStyleID); err != nil {
		return err
	}

	// Remove the default sheet
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return errors.WithStack(err)
	}

	// Set active sheet to Issues sheet
	f.SetActiveSheet(0)

	// Save file
	if err := f.SaveAs(config.OutputFile); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

const (
	sheetProjections = "Projections"
	headerDate       = "Date"
	headerCompleted  = "Completed"
	headerRemaining  = "Remaining"
	headerVelocity   = "Velocity"
	headerDiff       = "Diff"
	headerMean       = "Mean"
)

// projectionCols is 1-based Excel column numbers for the Projections sheet.
// Remaining and forecast columns are 0 when the report is a burnup.
type projectionCols struct {
	date, completed, remaining, velocity, avg, diff int
	std, fast, mean, slow, vFast, vSlow             int
}

func newProjectionCols(burnup bool) projectionCols {
	if burnup {
		// Remaining is omitted; Velocity and Avg shift left; Diff replaces forecasts.
		return projectionCols{
			date:      1,
			completed: 2,
			velocity:  3,
			avg:       4,
			diff:      5,
		}
	}
	return projectionCols{
		date:      1,
		completed: 2,
		remaining: 3,
		velocity:  4,
		avg:       5,
		std:       6,
		fast:      7,
		mean:      8,
		slow:      9,
		vFast:     10,
		vSlow:     11,
	}
}

func writeProjectionsSheet(f *excelize.File, cfg *config.Config, weeks []time.Time, numStyleID, dateStyleID int) error {
	if _, err := f.NewSheet(sheetProjections); err != nil {
		return errors.WithStack(err)
	}

	burnup := cfg.IsBurnup()
	cols := newProjectionCols(burnup)
	movingAvgWeeks := cfg.MovingAvgWeeks

	if err := writeProjectionHeaders(f, cols, burnup, movingAvgWeeks); err != nil {
		return err
	}

	signedFmt := "+0.0;-0.0;0.0"
	signedStyleID, err := f.NewStyle(&excelize.Style{CustomNumFmt: &signedFmt})
	if err != nil {
		return err
	}

	velocityName, err := excelize.ColumnNumberToName(cols.velocity)
	if err != nil {
		return errors.WithStack(err)
	}
	// First velocity is the second week (row 3); OFFSET windows start there.
	firstVelocityCell := fmt.Sprintf("%s$3", velocityName)

	for weekIndex, weekDate := range weeks {
		rowNum := weekIndex + 2
		if err := writeProjectionRow(f, projectionRowArgs{
			cols:              cols,
			burnup:            burnup,
			weekIndex:         weekIndex,
			rowNum:            rowNum,
			weekDate:          weekDate,
			movingAvgWeeks:    movingAvgWeeks,
			firstVelocityCell: firstVelocityCell,
			numStyleID:        numStyleID,
			dateStyleID:       dateStyleID,
			signedStyleID:     signedStyleID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func writeProjectionHeaders(f *excelize.File, cols projectionCols, burnup bool, movingAvgWeeks uint) error {
	if err := setHeader(f, cols.date, headerDate); err != nil {
		return err
	}
	if err := setHeader(f, cols.completed, headerCompleted); err != nil {
		return err
	}
	if !burnup {
		if err := setHeader(f, cols.remaining, headerRemaining); err != nil {
			return err
		}
	}
	if err := setHeader(f, cols.velocity, headerVelocity); err != nil {
		return err
	}
	if err := setHeader(f, cols.avg, fmt.Sprintf("Avg (%dw)", movingAvgWeeks)); err != nil {
		return err
	}
	if burnup {
		return setHeader(f, cols.diff, fmt.Sprintf("%s (%dw)", headerDiff, movingAvgWeeks))
	}
	if err := setHeader(f, cols.std, fmt.Sprintf("StdDev (%dw)", movingAvgWeeks)); err != nil {
		return err
	}
	if err := setHeader(f, cols.fast, "Fast (p90)"); err != nil {
		return err
	}
	if err := setHeader(f, cols.mean, headerMean); err != nil {
		return err
	}
	if err := setHeader(f, cols.slow, "Slow (p90)"); err != nil {
		return err
	}
	if err := setHeader(f, cols.vFast, "V. Fast (p90)"); err != nil {
		return err
	}
	return setHeader(f, cols.vSlow, "V. Slow (p90)")
}

func setHeader(f *excelize.File, col int, value string) error {
	cell, err := excelize.CoordinatesToCellName(col, 1)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(sheetProjections, cell, value); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type projectionRowArgs struct {
	cols              projectionCols
	burnup            bool
	weekIndex         int
	rowNum            int
	weekDate          time.Time
	movingAvgWeeks    uint
	firstVelocityCell string
	numStyleID        int
	dateStyleID       int
	signedStyleID     int
}

func writeProjectionRow(f *excelize.File, args projectionRowArgs) error {
	dateCell, err := excelize.CoordinatesToCellName(args.cols.date, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(sheetProjections, dateCell, args.weekDate.Format("2006-01-02")); err != nil {
		return errors.WithStack(err)
	}

	completedCell, err := excelize.CoordinatesToCellName(args.cols.completed, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	completedFormula := fmt.Sprintf(`=SUM(INDEX(Work!$2:$10000, , MATCH("EV "&TEXT(%s,"mm-dd"), Work!$1:$1, 0)))`, dateCell)
	if err := setStyledFormula(f, completedCell, completedFormula, args.numStyleID); err != nil {
		return err
	}

	if !args.burnup {
		remainingCell, err := excelize.CoordinatesToCellName(args.cols.remaining, args.rowNum)
		if err != nil {
			return errors.WithStack(err)
		}
		remainingFormula := fmt.Sprintf(`=SUM(INDEX(Work!$2:$10000, , MATCH("Size", Work!$1:$1, 0)))-%s`, completedCell)
		if err := setStyledFormula(f, remainingCell, remainingFormula, args.numStyleID); err != nil {
			return err
		}
	}

	velocityCell, err := excelize.CoordinatesToCellName(args.cols.velocity, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	if args.weekIndex > 0 {
		priorCompletedCell, err := excelize.CoordinatesToCellName(args.cols.completed, args.rowNum-1)
		if err != nil {
			return errors.WithStack(err)
		}
		velocityFormula := fmt.Sprintf(`=%s-%s`, completedCell, priorCompletedCell)
		if err := setStyledFormula(f, velocityCell, velocityFormula, args.numStyleID); err != nil {
			return err
		}
	}

	avgVelocityCell, err := excelize.CoordinatesToCellName(args.cols.avg, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	if args.weekIndex > 1 {
		avgVelocityFormula := fmt.Sprintf(
			`=AVERAGE(OFFSET(%s, -1 * (MIN(COUNT(%s:%s),%d) -1), 0, MIN(COUNT(%s:%s),%d), 1))`,
			velocityCell, args.firstVelocityCell, velocityCell, args.movingAvgWeeks, args.firstVelocityCell, velocityCell, args.movingAvgWeeks,
		)
		if err := setStyledFormula(f, avgVelocityCell, avgVelocityFormula, args.numStyleID); err != nil {
			return err
		}
	}

	if args.burnup {
		return writeBurnupDiff(f, args, velocityCell, avgVelocityCell)
	}
	return writeBurndownForecasts(f, args, dateCell, velocityCell, avgVelocityCell)
}

func writeBurnupDiff(f *excelize.File, args projectionRowArgs, velocityCell, avgVelocityCell string) error {
	if args.weekIndex <= 1 {
		return nil
	}
	diffCell, err := excelize.CoordinatesToCellName(args.cols.diff, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	// Positive Diff means this week was faster than the moving average.
	diffFormula := fmt.Sprintf(`=%s-%s`, velocityCell, avgVelocityCell)
	return setStyledFormula(f, diffCell, diffFormula, args.signedStyleID)
}

func writeBurndownForecasts(f *excelize.File, args projectionRowArgs, dateCell, velocityCell, avgVelocityCell string) error {
	if args.weekIndex <= 2 {
		return nil
	}

	remainingCell, err := excelize.CoordinatesToCellName(args.cols.remaining, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	stdVelocityCell, err := excelize.CoordinatesToCellName(args.cols.std, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	stdVelocityFormula := fmt.Sprintf(
		`=STDEV(OFFSET(%s, -1 * (MIN(COUNT(%s:%s),%d) -1), 0, MIN(COUNT(%s:%s),%d), 1))`,
		velocityCell, args.firstVelocityCell, velocityCell, args.movingAvgWeeks, args.firstVelocityCell, velocityCell, args.movingAvgWeeks,
	)
	if err := setStyledFormula(f, stdVelocityCell, stdVelocityFormula, args.numStyleID); err != nil {
		return err
	}

	fastVelocityCell, err := excelize.CoordinatesToCellName(args.cols.vFast, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	slowVelocityCell, err := excelize.CoordinatesToCellName(args.cols.vSlow, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}

	fastProjectionCell, err := excelize.CoordinatesToCellName(args.cols.fast, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	fastProjectionFormula := fmt.Sprintf(`=WORKDAY(%s, CEILING((%s/%s)*5, 1))`, dateCell, remainingCell, fastVelocityCell)
	if err := setStyledFormula(f, fastProjectionCell, fastProjectionFormula, args.dateStyleID); err != nil {
		return err
	}

	meanProjectionCell, err := excelize.CoordinatesToCellName(args.cols.mean, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	meanProjectionFormula := fmt.Sprintf(`=WORKDAY(%s, CEILING((%s/%s)*5, 1))`, dateCell, remainingCell, avgVelocityCell)
	if err := setStyledFormula(f, meanProjectionCell, meanProjectionFormula, args.dateStyleID); err != nil {
		return err
	}

	// Slow projection: if the lower CI velocity is <= 0, a finish date is not
	// meaningful (leave V. Slow as-is; show "unknown" here instead of WORKDAY).
	slowProjectionCell, err := excelize.CoordinatesToCellName(args.cols.slow, args.rowNum)
	if err != nil {
		return errors.WithStack(err)
	}
	slowProjectionFormula := fmt.Sprintf(
		`=IF(%s<=0,"unknown",WORKDAY(%s,CEILING((%s/%s)*5,1)))`,
		slowVelocityCell, dateCell, remainingCell, slowVelocityCell,
	)
	if err := setStyledFormula(f, slowProjectionCell, slowProjectionFormula, args.dateStyleID); err != nil {
		return err
	}

	// Fast / slow velocity bounds (90% CI half-width via t-distribution).
	//
	// OpenXML requires the _xlfn. prefix on CONFIDENCE.T or Excel shows #NAME?
	// and will not compute the cell. Sample size is the velocity COUNT, not ROWS.
	sampleSizeExpr := fmt.Sprintf(`MIN(%d,COUNT(%s:%s))`, args.movingAvgWeeks, args.firstVelocityCell, velocityCell)
	fastVelocityFormula := fmt.Sprintf(
		`=%s+_xlfn.CONFIDENCE.T(0.1,%s,%s)`,
		avgVelocityCell, stdVelocityCell, sampleSizeExpr,
	)
	if err := setStyledFormula(f, fastVelocityCell, fastVelocityFormula, args.numStyleID); err != nil {
		return err
	}

	slowVelocityFormula := fmt.Sprintf(
		`=%s-_xlfn.CONFIDENCE.T(0.1,%s,%s)`,
		avgVelocityCell, stdVelocityCell, sampleSizeExpr,
	)
	return setStyledFormula(f, slowVelocityCell, slowVelocityFormula, args.numStyleID)
}

func setStyledFormula(f *excelize.File, cell, formula string, styleID int) error {
	if err := f.SetCellFormula(sheetProjections, cell, formula); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellStyle(sheetProjections, cell, cell, styleID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
