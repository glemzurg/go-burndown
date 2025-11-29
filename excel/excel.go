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
	movingAvgWeeks := config.MovingAvgWeeks

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
		col := 7 // Start after Size column (F)
		for _, weekDate := range reversedWeeks {

			// Get percent complete for this issue at this week date
			percentComplete, err := issue.PercentCompleteOnDate(config, weekDate)
			if err != nil {
				return errors.WithStack(err)
			}

			// Set percent complete value (as fraction for Excel)
			// Leave field blank is percent complete is zero.
			percentCell := fmt.Sprintf("%s%d", string(rune('A'+col-1)), rowNum)
			if percentComplete > 0 {
				if err := f.SetCellValue(workSheet, percentCell, percentComplete); err != nil { // 0.0-1.0
					return errors.WithStack(err)
				}
			}
			if err := f.SetCellStyle(workSheet, percentCell, percentCell, percentStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Earned Value formula: percent * size, blank if percent is zero for easy display.
			earnedCell := fmt.Sprintf("%s%d", string(rune('A'+col)), rowNum)
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

	// Create Projections sheet
	projectionsSheet := "Projections"
	if _, err := f.NewSheet(projectionsSheet); err != nil {
		return errors.WithStack(err)
	}

	// Headers for projections sheet
	if err := f.SetCellValue(projectionsSheet, "A1", "Date"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "B1", "Completed"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "C1", "Remaining"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "D1", "Velocity"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "E1", fmt.Sprintf("Avg (%dw)", movingAvgWeeks)); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "F1", fmt.Sprintf("StdDev (%dw)", movingAvgWeeks)); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "G1", "Fast (p68)"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "H1", "Mean"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "I1", "Slow (p68)"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "J1", "V. Fast (p68)"); err != nil {
		return errors.WithStack(err)
	}
	if err := f.SetCellValue(projectionsSheet, "K1", "V. Slow (p68)"); err != nil {
		return errors.WithStack(err)
	}

	// Add projection data - one row per week
	for weekIndex, weekDate := range weeks {
		rowNum := weekIndex + 2

		// Set the date
		dateCell := fmt.Sprintf("A%d", rowNum)
		if err := f.SetCellValue(projectionsSheet, dateCell, weekDate.Format("2006-01-02")); err != nil {
			return errors.WithStack(err)
		}

		// The work completed.
		completedCell := fmt.Sprintf("B%d", rowNum)
		completedFormula := fmt.Sprintf(`=SUM(INDEX(Work!$2:$10000, , MATCH("EV "&TEXT(%s,"mm-dd"), Work!$1:$1, 0)))`, dateCell)
		if err := f.SetCellFormula(projectionsSheet, completedCell, completedFormula); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellStyle(projectionsSheet, completedCell, completedCell, numStyleID); err != nil {
			return errors.WithStack(err)
		}

		// The remaining work.
		remainingCell := fmt.Sprintf("C%d", rowNum)
		remainingFormula := fmt.Sprintf(`=SUM(INDEX(Work!$2:$10000, , MATCH("Size", Work!$1:$1, 0)))-%s`, completedCell)
		if err := f.SetCellFormula(projectionsSheet, remainingCell, remainingFormula); err != nil {
			return errors.WithStack(err)
		}
		if err := f.SetCellStyle(projectionsSheet, remainingCell, remainingCell, numStyleID); err != nil {
			return errors.WithStack(err)
		}

		// We can only compute velocity if we're not the first data cell (need two data entries.)
		velocityCell := fmt.Sprintf("D%d", rowNum)
		firstVelocityCell := "D$3" // The cell where the first velocity is found.
		if weekIndex > 0 {
			// The velocity computation.
			priorCompletedCell := fmt.Sprintf("B%d", rowNum-1)
			velocityFormula := fmt.Sprintf(`=%s-%s`, completedCell, priorCompletedCell)
			if err := f.SetCellFormula(projectionsSheet, velocityCell, velocityFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, velocityCell, velocityCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}
		}

		// Moving average velocity computation.
		// We need at least two velocities.
		avgVelocityCell := fmt.Sprintf("E%d", rowNum)
		if weekIndex > 1 {
			// The average velocity computation.
			avgVelocityFormula := fmt.Sprintf(`=AVERAGE(OFFSET(%s, -1 * (MIN(COUNT(%s:%s),%d) -1), 0, MIN(COUNT(%s:%s),%d), 1))`, velocityCell, firstVelocityCell, velocityCell, movingAvgWeeks, firstVelocityCell, velocityCell, movingAvgWeeks)
			if err := f.SetCellFormula(projectionsSheet, avgVelocityCell, avgVelocityFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, avgVelocityCell, avgVelocityCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}
		}

		// All other computations require at least two average velocities.
		if weekIndex > 2 {
			// Standard deviation (of velocities).
			stdVelocityCell := fmt.Sprintf("F%d", rowNum)
			stdVelocityFormula := fmt.Sprintf(`=STDEV(OFFSET(%s, -1 * (MIN(COUNT(%s:%s),%d) -1), 0, MIN(COUNT(%s:%s),%d), 1))`, velocityCell, firstVelocityCell, velocityCell, movingAvgWeeks, firstVelocityCell, velocityCell, movingAvgWeeks)
			if err := f.SetCellFormula(projectionsSheet, stdVelocityCell, stdVelocityFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, stdVelocityCell, stdVelocityCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}

			// What are the p68 velocity cell names.
			fastVelocityCell := fmt.Sprintf("J%d", rowNum)
			slowVelocityCell := fmt.Sprintf("K%d", rowNum)

			// Fast projection.
			fastProjectionCell := fmt.Sprintf("G%d", rowNum)
			fastProjectionFormula := fmt.Sprintf(`=WORKDAY(%s, CEILING((%s/%s)*5, 1))`, dateCell, remainingCell, fastVelocityCell)
			if err := f.SetCellFormula(projectionsSheet, fastProjectionCell, fastProjectionFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, fastProjectionCell, fastProjectionCell, dateStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Mean projection.
			meanProjectionCell := fmt.Sprintf("H%d", rowNum)
			meanProjectionFormula := fmt.Sprintf(`=WORKDAY(%s, CEILING((%s/%s)*5, 1))`, dateCell, remainingCell, avgVelocityCell)
			if err := f.SetCellFormula(projectionsSheet, meanProjectionCell, meanProjectionFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, meanProjectionCell, meanProjectionCell, dateStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Slow projection).
			slowProjectionCell := fmt.Sprintf("I%d", rowNum)
			slowProjectionFormula := fmt.Sprintf(`=WORKDAY(%s, CEILING((%s/%s)*5, 1))`, dateCell, remainingCell, slowVelocityCell)
			if err := f.SetCellFormula(projectionsSheet, slowProjectionCell, slowProjectionFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, slowProjectionCell, slowProjectionCell, dateStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Fast velocity (p68).
			fastVelocityFormula := fmt.Sprintf(`=%s+(1*%s)`, avgVelocityCell, stdVelocityCell)
			if err := f.SetCellFormula(projectionsSheet, fastVelocityCell, fastVelocityFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, fastVelocityCell, fastVelocityCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}

			// Slow velocity (p68).
			slowVelocityFormula := fmt.Sprintf(`=%s-(1*%s)`, avgVelocityCell, stdVelocityCell)
			if err := f.SetCellFormula(projectionsSheet, slowVelocityCell, slowVelocityFormula); err != nil {
				return errors.WithStack(err)
			}
			if err := f.SetCellStyle(projectionsSheet, slowVelocityCell, slowVelocityCell, numStyleID); err != nil {
				return errors.WithStack(err)
			}
		}
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
