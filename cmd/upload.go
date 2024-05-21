package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	reporter "github.com/redhat-eets/reporter"
	flag "github.com/spf13/pflag"
	viper "github.com/spf13/viper"
)

var UploadFlagSet = flag.NewFlagSet("upload", flag.ExitOnError)

var (
	flagConfigPath       string
	flagJUnitInputPaths  []string
	flagJiraDestIssueID  string
	flagJiraServerURL    string
	flagJiraAccessToken  string
	flagJiraSyncDisabled bool
)

const (
	defaultConfigPath       = "."
	defaultJUnitInputPath   = "input/"
	defaultJiraDestIssueID  = ""
	defaultJiraServerURL    = "https://issues.redhat.com"
	defaultJiraAccessToken  = ""
	defaultJiraSyncDisabled = false
)

func init() {
	UploadFlagSet.StringSliceVarP(
		&flagJUnitInputPaths,
		"input",
		"i",
		[]string{defaultJUnitInputPath},
		"Optional path to JUnit XML test report file. Can be provided multiple times",
	)
	UploadFlagSet.StringVarP(
		&flagJiraDestIssueID,
		"dest",
		"d",
		defaultJiraDestIssueID,
		"Optional destination to upload all test reports to. Can either be a Jira Story or Sub-task",
	)
	UploadFlagSet.StringVarP(
		&flagConfigPath,
		"config",
		"c",
		defaultConfigPath,
		"Optional path to user configuration file",
	)
	UploadFlagSet.StringVarP(
		&flagJiraServerURL,
		"jira-server-url",
		"s",
		defaultJiraServerURL,
		"Optional URL of the Jira server instance to connect to",
	)
	UploadFlagSet.StringVarP(
		&flagJiraAccessToken,
		"jira-token",
		"t",
		defaultJiraAccessToken,
		fmt.Sprintf("Service account access token for Jira. Can also be set using the \"%s\" env var", EnvNameJiraAccessToken),
	)
	UploadFlagSet.BoolVarP(
		&flagJiraSyncDisabled,
		"no-sync",
		"n",
		defaultJiraSyncDisabled,
		"Toggle to disable sending requests to the Jira API",
	)
	UploadFlagSet.Usage = func() { PrintUsage("upload", []string{}, UploadFlagSet) }

	viper.BindPFlags(UploadFlagSet)
	viper.BindEnv("jira-token", EnvNameJiraAccessToken)
}

func isValidJUnitInputFile(path string, d fs.DirEntry, err error) bool {
	supportedFileExtensions := []string{".junit", ".xml"}

	if err != nil {
		return false
	}

	if d.IsDir() {
		return false
	}

	hasSupportedExtension := false
	for _, ext := range supportedFileExtensions {
		if strings.HasSuffix(path, ext) {
			hasSupportedExtension = true
		}
	}

	return hasSupportedExtension
}

func getJUnitTestReportPaths(paths []string) (files []string, err error) {
	for _, input := range paths {
		err := filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
			// O(n) search performed for each valid path? Oops, sorry! Doesn't matter anyway.
			if isValidJUnitInputFile(path, d, err) && !slices.Contains(files, path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return files, err
		}
	}

	return files, err
}

func loadConfig(path string) (reporter.Config, error) {
	var config reporter.Config

	viper.SetEnvPrefix(EnvNamePrefix)
	viper.SetConfigName(ConfigName)
	viper.SetConfigType(ConfigType)
	viper.AddConfigPath(path)

	// Load default config
	if err := viper.ReadConfig(bytes.NewBuffer(reporter.EmbeddedDefaultConfig)); err != nil {
		return config, err
	}

	// Merge user config into default config
	if err := viper.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			InfoLog.Printf("User config not found at '%s'. Continuing", path)
		} else {
			return config, err
		}
	} else {
		InfoLog.Printf("Loaded config file at '%s'", viper.ConfigFileUsed())
	}
	if err := viper.Unmarshal(&config); err != nil {
		return config, err
	}

	if config.Spec.Reporting.Routing == nil {
		InfoLog.Println("No custom routing for test reports found. This can be configured in the 'spec.reporting.routing' section of the config file")
	} else {
		InfoLog.Printf("Routing rules loaded from config file: %d", len(config.Spec.Reporting.Routing))
	}

	return config, nil
}

func UploadCmd() {
	LogReleaseDetails()

	UploadFlagSet.Parse(os.Args[2:])

	config, err := loadConfig(flagConfigPath)
	if err != nil {
		ErrorLog.Fatalln(err)
	}

	// Ensure only JUnit test reports will be processed and not other artifacts (logs, etc)
	junitTestReportPaths, err := getJUnitTestReportPaths(flagJUnitInputPaths)
	if err != nil {
		ErrorLog.Fatalln(err)
	}

	if flagJiraDestIssueID != "" {
		InfoLog.Printf("[-d/--dest flag set] Adding a global route for Jira issue '%s'. Any routes defined in the config file will be discarded", flagJiraDestIssueID)
		globalRoutes := []reporter.ReportingRouteConfig{{
			Destination: flagJiraDestIssueID,
		}}
		config.Spec.Reporting.Routing = globalRoutes
	}

	if flagJiraServerURL != "" {
		config.Spec.Jira.Server.URL = flagJiraServerURL
	}

	InfoLog.Printf("Processing %d JUnit test reports %v", len(junitTestReportPaths), junitTestReportPaths)
	reports, err := reporter.ProcessJUnitReports(junitTestReportPaths, config.Spec.Reporting)
	if err != nil {
		ErrorLog.Fatalln(err)
	}

	reporter.LogAggregateReports(InfoLog, reports)

	if flagJiraSyncDisabled {
		InfoLog.Println("[-n/--no-sync flag set] Synchronization with Jira has been disabled. No test reports will be uploaded")
	} else {
		token := viper.GetString("jira-token")

		if token == "" {
			ErrorLog.Fatalf("Jira access token not set. Use the -t/--jira-token flag or set the '%s' env var", EnvNameJiraAccessToken)
		}

		if err := reporter.UploadAggregateReports(reports, config, token); err != nil {
			ErrorLog.Fatalln(err)
		}
	}
}
