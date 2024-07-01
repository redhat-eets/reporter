package reporter

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/joshdk/go-junit"
	"golang.org/x/exp/maps"
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

func groupRouteConfigsByDestination(routeConfigs []ReportingRouteConfig) []ReportingRouteConfig {
	configs := map[string][]ReportingTestSuiteConfig{}
	for _, config := range routeConfigs {
		configs[config.Destination] = append(configs[config.Destination], config.TestSuites...)
	}

	destinations := maps.Keys(configs)
	sort.Strings(destinations)

	groupedRouteConfigs := []ReportingRouteConfig{}
	for _, dest := range destinations {
		suites := configs[dest]
		groupedRouteConfigs = append(groupedRouteConfigs, ReportingRouteConfig{
			Destination: dest,
			TestSuites:  suites,
		})
	}

	return groupedRouteConfigs
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

	routes := groupRouteConfigsByDestination(routing)
	for _, route := range routes {
		report := ProcessJUnitSuites(suites, route)
		reports = append(reports, report)
	}

	return reports, nil
}

func isEntityMatchedByNameRule(entityName string, nameRule string) bool {
	if nameRule == "" {
		return false
	}

	return nameRule == MatchAllSymbol || nameRule == entityName
}

func isEntityMatchedByPropertyRule(entityProperties map[string]string, propertyRule string) bool {
	if propertyRule == "" {
		return false
	}

	separator := "="
	p := strings.Split(propertyRule, separator)
	if len(p) < 2 {
		return false
	}

	name := p[0]
	value := strings.Join(p[1:], separator)

	return entityProperties[name] == value
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
			for _, rule := range route.TestSuites {
				if isEntityMatchedByNameRule(suite.Name, rule.Name) {
					if rule.TestCases != nil {
						testCaseRules = append(testCaseRules, rule.TestCases...)
					} else {
						testCaseRules = append(testCaseRules, testCaseRuleMatchAll)
					}
				} else if isEntityMatchedByPropertyRule(suite.Properties, rule.Property) {
					testCaseRules = append(testCaseRules, rule.TestCases...)
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
					if isEntityMatchedByNameRule(test.Name, rule.Name) || isEntityMatchedByPropertyRule(test.Properties, rule.Property) {
						processedTestSuite.Counts.Add(test)
					}
				}
			}

			report.TestSuites = append(report.TestSuites, processedTestSuite)
		}
	}

	report.AggregateCounts()

	return report
}
