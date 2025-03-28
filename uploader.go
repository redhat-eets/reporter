package reporter

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
)

// IssueDesiredStateFields stores values that are expected to be sent to Jira
// in order for the Jira Issue to reach the desired state.
type IssueDesiredStateFields struct {
	Summary     string
	Description string
	Labels      []string
}

func getIssueDesiredStateFields(report AggregateReport, config Config) (f IssueDesiredStateFields, err error) {
	desiredState := config.Spec.Jira.DesiredState

	f.Summary = desiredState.Summary.Contents
	if desiredState.Summary.IncludeTestCounts {
		f.Summary = fmt.Sprintf("%s (%d/%d PASSED)", f.Summary, report.Counts.Passed, report.Counts.Total-report.Counts.Skipped)
	}

	descTemplatePath := desiredState.Description.TemplatePath
	embeddedTemplatePrefix := "embedded:"

	var buf bytes.Buffer
	if strings.HasPrefix(descTemplatePath, embeddedTemplatePrefix) {
		path := strings.Replace(descTemplatePath, embeddedTemplatePrefix, "", 1)
		buf, err = RenderEmbeddedTemplate(path, report)
		if err != nil {
			return f, fmt.Errorf("embedded description template could not be rendered: %w", err)
		}
	} else {
		buf, err = RenderLocalTemplate(descTemplatePath, report)
		if err != nil {
			return f, fmt.Errorf("local description template could not be rendered: %w", err)
		}
	}
	f.Description = buf.String()

	f.Labels = desiredState.OnFailure.Labels
	if report.Counts.Failed == 0 && report.Counts.Errored == 0 {
		f.Labels = desiredState.OnSuccess.Labels
	}

	return f, nil
}

// LogUploadSummary logs a simple summary message displaying how many AggregateReports were successfully uploaded.
func LogUploadSummary(logger *log.Logger, uploadedCount int, reports []AggregateReport) {
	logger.Printf("Summary: %d/%d Aggregate Reports uploaded to Jira", uploadedCount, len(reports))
}

// UploadSingleAggregateReport uploads a given AggregateReport to its destination.
func UploadSingleAggregateReport(report AggregateReport, config Config, token string) error {
	if report.Destination == "" {
		return errors.New("given report does not have a valid destination")
	}

	if report.Counts.Total <= 0 {
		return errors.New("given report is empty")
	}

	fields, err := getIssueDesiredStateFields(report, config)
	if err != nil {
		return err
	}

	if err := updateStatusInJira(config, token, report.Destination, fields); err != nil {
		return err
	}

	return nil
}

// UploadAggregateReports takes multiple AggregateReports and uploads them all to their corresponding destinations.
func UploadAggregateReports(reports []AggregateReport, config Config, token string) error {
	uploadedCount := 0

	for i, report := range reports {
		if err := UploadSingleAggregateReport(report, config, token); err != nil {
			WarnLog.Printf("Aggregate Report %d) could not be uploaded: %s", i+1, err)
		} else {
			uploadedCount++
		}
	}

	LogUploadSummary(InfoLog, uploadedCount, reports)

	if uploadedCount != len(reports) {
		return fmt.Errorf("%d Aggregate Report(s) failed to be uploaded", len(reports)-uploadedCount)
	}

	return nil
}

func isJiraSubtaskValidDestination(issue *JiraIssue, config Config) bool {
	desiredSummaryContents := config.Spec.Jira.DesiredState.Summary.Contents
	return strings.Contains(issue.Summary, desiredSummaryContents)
}

func updateStatusInJira(config Config, token string, issueID string, fields IssueDesiredStateFields) error {
	client := JiraClient{
		ServerURL:   config.Spec.Jira.Server.URL,
		AccessToken: token,
	}

	issue, err := client.GetIssue(issueID)
	if err != nil {
		return err
	}

	InfoLog.Printf("Processing issue '%s' type: '%s', summary: '%s'", issueID, issue.Type, issue.Summary)
	if issue.Type == "Sub-task" {
		// Check the Issue Summary to ensure we are not overwriting an incorrect Sub-task by mistake
		if isJiraSubtaskValidDestination(&issue, config) {
			if err := client.UpdateIssue(issueID, fields.Summary, fields.Description, fields.Labels); err != nil {
				return fmt.Errorf("sub-task could not be updated: %w", err)
			}
		} else {
			desiredSummaryContents := config.Spec.Jira.DesiredState.Summary.Contents
			return fmt.Errorf("summary of target Sub-task '%s' does not contain '%s'", issueID, desiredSummaryContents)
		}
	} else if issue.Type == "Story" {
		// Ensure the Story has a proper prefix and labels
		requiredPrefix := config.Spec.Jira.Discovery.Summary.RequiredPrefix
		if requiredPrefix != "" && !strings.HasPrefix(issue.Summary, requiredPrefix) {
			return fmt.Errorf("summary of target Story '%s' does not have the required prefix '%s'", issueID, requiredPrefix)
		}

		requiredLabels := config.Spec.Jira.Discovery.Labels.RequiredAnyOf
		if requiredLabels != nil && !issue.IsLabeledWithAnyOf(requiredLabels) {
			return fmt.Errorf("target Story '%s' is not labeled with any of the following: %v", issueID, requiredLabels)
		}

		var subtaskID string
		for _, child := range issue.SubTasks {
			if isJiraSubtaskValidDestination(child, config) {
				subtaskID = child.ID
				InfoLog.Printf("Found a matching Sub-task '%s' for issue '%s'", subtaskID, issueID)
				break
			}
		}

		if subtaskID != "" {
			if err := client.UpdateIssue(subtaskID, fields.Summary, fields.Description, fields.Labels); err != nil {
				return fmt.Errorf("sub-task could not be updated: %w", err)
			}
		} else {
			newSubtaskID, err := client.CreateSubtask(issueID, fields.Summary, fields.Description, fields.Labels)
			if err != nil {
				return fmt.Errorf("sub-task could not be created: %w", err)
			}
			InfoLog.Printf("Created new Sub-task '%s' for issue '%s'", newSubtaskID, issueID)
		}
	} else {
		return fmt.Errorf("target issue has to be either a Story or a Sub-task. Got '%s' instead", issue.Type)
	}

	return nil
}
