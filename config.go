package reporter

type Config struct {
	APIVersion string     `mapstructure:"apiVersion"`
	Spec       ConfigSpec `mapstructure:"spec"`
}

type ConfigSpec struct {
	Jira      JiraConfig      `mapstructure:"jira"`
	Reporting ReportingConfig `mapstructure:"reporting"`
}

type JiraConfig struct {
	Server       JiraServerConfig            `mapstructure:"server"`
	Discovery    JiraIssueDiscoveryConfig    `mapstructure:"discovery"`
	DesiredState JiraIssueDesiredStateConfig `mapstructure:"desiredState"`
}

type JiraServerConfig struct {
	URL string `mapstructure:"url"`
}

// Jira issue auto-discovery configuration

type JiraIssueDiscoveryConfig struct {
	Summary JiraIssueDiscoverySummaryConfig `mapstructure:"summary"`
	Labels  JiraIssueDiscoveryLabelsConfig  `mapstructure:"labels"`
}

type JiraIssueDiscoverySummaryConfig struct {
	RequiredPrefix string `mapstructure:"requiredPrefix"`
}

type JiraIssueDiscoveryLabelsConfig struct {
	RequiredAnyOf []string `mapstructure:"requiredAnyOf"`
}

// Jira issue desired state configuration

type JiraIssueDesiredStateConfig struct {
	Summary     JiraIssueDesiredStateSummaryConfig     `mapstructure:"summary"`
	Description JiraIssueDesiredStateDescriptionConfig `mapstructure:"description"`
	OnSuccess   JiraIssueDesiredStateConditionalConfig `mapstructure:"onSuccess"`
	OnFailure   JiraIssueDesiredStateConditionalConfig `mapstructure:"onFailure"`
}

type JiraIssueDesiredStateSummaryConfig struct {
	Contents          string `mapstructure:"contents"`
	IncludeTestCounts bool   `mapstructure:"includeTestCounts"`
}

type JiraIssueDesiredStateDescriptionConfig struct {
	TemplatePath string `mapstructure:"templatePath"`
}

type JiraIssueDesiredStateConditionalConfig struct {
	Labels []string `mapstructure:"labels"`
}

// Reporting configuration

type ReportingConfig struct {
	Routing []ReportingRouteConfig `mapstructure:"routing"`
}

type ReportingRouteConfig struct {
	Destination string                     `mapstructure:"destination"`
	TestSuites  []ReportingTestSuiteConfig `mapstructure:"testSuites"`
}

type ReportingTestSuiteConfig struct {
	Name      string                    `mapstructure:"name"`
	Property  string                    `mapstructure:"property"`
	TestCases []ReportingTestCaseConfig `mapstructure:"testCases"`
}

type ReportingTestCaseConfig struct {
	Name     string `mapstructure:"name"`
	Property string `mapstructure:"property"`
}
