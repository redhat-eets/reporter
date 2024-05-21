package reporter

import (
	"fmt"
	"log"

	"github.com/joshdk/go-junit"
)

// MatchAllSymbol defines which symbol will be used to represent a rule that accepts any string as input.
const MatchAllSymbol = "*"

// Counts provides a container for storing information about test counts.
// "Failed" counts both JUnit "Failures" and "Errors" under a common field, as Reporter does not need
// to distinguish between the two types: both cause the test run to be marked as unsuccessful.
type Counts struct {
	Passed  int
	Failed  int
	Skipped int
	Total   int
}

// Add increases test counts based on the status of the Test Case given by the user.
func (c *Counts) Add(test junit.Test) {
	if test.Status == junit.StatusPassed {
		c.Passed++
	} else if test.Status == junit.StatusSkipped {
		c.Skipped++
	} else {
		c.Failed++
	}

	c.Total++
}

// TestSuite represents a Test Suite from a test report.
type TestSuite struct {
	Name   string
	Counts Counts
}

// AggregateReport stores information about all Test Suites and Test Cases
// that should be reported to a selected Jira Issue (Destination).
type AggregateReport struct {
	Destination string
	TestSuites  []TestSuite
	Counts      Counts
}

// AggregateCounts takes all Test Suites contained in the report and calculates the total sum on all Counters.
func (r *AggregateReport) AggregateCounts() {
	r.Counts = Counts{}

	for _, suite := range r.TestSuites {
		r.Counts.Passed += suite.Counts.Passed
		r.Counts.Failed += suite.Counts.Failed
		r.Counts.Skipped += suite.Counts.Skipped
		r.Counts.Total += suite.Counts.Total
	}
}

// LogAggregateReports dumps the AggregateReports in a human-readable form to a given logger.
func LogAggregateReports(logger *log.Logger, reports []AggregateReport) {
	logger.Println("Printing Aggregate Reports created based on configured routing rules")
	for i, report := range reports {
		c := report.Counts

		dest := "(no destination specified)"
		if report.Destination != "" {
			dest = report.Destination
		}

		note := ""
		if c.Total <= 0 {
			note = "(no data to upload)"
		}

		logger.Printf("%-3s Passed %-4d Failed %-4d Skipped %-4d Total %-4d -> Jira %s %s", fmt.Sprintf("%d)", i+1), c.Passed, c.Failed, c.Skipped, c.Total, dest, note)
	}
}

// ProcessJUnitReports loads and analyzes JUnit Test Reports according to the routing config defined by the user.
func ProcessJUnitReports(paths []string, config ReportingConfig) (reports []AggregateReport, err error) {
	suites, err := junit.IngestFiles(paths)
	if err != nil {
		return nil, err
	}

	routing := config.Routing
	if routing == nil {
		routing = []ReportingRouteConfig{{}}
	}

	for _, route := range routing {
		report := ProcessJUnitSuites(suites, route)
		reports = append(reports, report)
	}

	return reports, nil
}

// ProcessJUnitSuites processes all loaded Test Suites according to a given routing configuration.
// A single AggregateReport will be created for each route defined by the user. If any Test Suites
// or Test Cases match any of the rules defined for this route, they will be added to the Report.
func ProcessJUnitSuites(suites []junit.Suite, route ReportingRouteConfig) (report AggregateReport) {
	report.Destination = route.Destination

	for _, suite := range suites {
		var testCaseRules []ReportingTestCaseConfig

		processedTestSuite := TestSuite{
			Name: suite.Name,
		}
		testCaseRuleMatchAll := ReportingTestCaseConfig{
			Name: MatchAllSymbol,
		}

		if route.TestSuites != nil {
			for _, testSuiteRule := range route.TestSuites {
				if testSuiteRule.Name == MatchAllSymbol || testSuiteRule.Name == suite.Name {
					if testSuiteRule.TestCases != nil {
						testCaseRules = append(testCaseRules, testSuiteRule.TestCases...)
					} else {
						testCaseRules = append(testCaseRules, testCaseRuleMatchAll)
					}
				}
			}
		} else {
			// Apply a Match-All rule when a route has no been configured
			testCaseRules = append(testCaseRules, testCaseRuleMatchAll)
		}

		if len(testCaseRules) > 0 {
			// This loop could be optimized, but it probably is not worth the extra effort
			for _, test := range suite.Tests {
				for _, rule := range testCaseRules {
					if rule.Name == MatchAllSymbol || test.Name == rule.Name {
						processedTestSuite.Counts.Add(test)
					} else if rule.Property != "" {
						// TODO: add test case matching by JUnit property
					}
				}
			}

			report.TestSuites = append(report.TestSuites, processedTestSuite)
		}
	}

	report.AggregateCounts()

	return report
}
