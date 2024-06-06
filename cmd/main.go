package main

import (
	"errors"
	"fmt"
	"os"

	reporter "github.com/redhat-eets/reporter"
	flag "github.com/spf13/pflag"
	viper "github.com/spf13/viper"
)

var (
	Version    = "dev"
	CommitHash = "unknown"
)

// Loggers configuration

var (
	InfoLog  = reporter.InfoLog
	WarnLog  = reporter.WarnLog
	ErrorLog = reporter.ErrorLog
)

// CLI configuration

var Commands = []string{"upload"}

const (
	EnvNamePrefix          = "REPORTER"
	EnvNameJiraAccessToken = "REPORTER_JIRA_TOKEN"

	ConfigName = "config"
	ConfigType = "yaml"

	usageTemplateFile = "templates/cli_usage.tmpl"
)

func init() {
	flag.BoolP("verbose", "v", false, "Enable verbose logging")
	flag.ErrHelp = errors.New("reporter: help requested")
	flag.Usage = func() { PrintUsage("", Commands, flag.CommandLine) }
	viper.BindPFlags(flag.CommandLine)
}

func PrintUsage(command string, commands []string, flagset *flag.FlagSet) {
	data := map[string]any{
		"ProgramName":           os.Args[0],
		"SelectedCommand":       command,
		"AvailableCommands":     commands,
		"AvailableOptions":      flagset.FlagUsages(),
		"JiraAccessTokenEnvVar": EnvNameJiraAccessToken,
	}

	buf, err := reporter.RenderEmbeddedTemplate(usageTemplateFile, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "help prompt template could not be rendered: %s\n", err)
		os.Exit(1)
	}
	fmt.Print(buf.String())
}

func LogReleaseDetails() {
	InfoLog.Printf("Reporter version %s, commit %s", Version, CommitHash)
}

func main() {
	if len(os.Args) < 2 {
		PrintUsage("", Commands, flag.CommandLine)
		os.Exit(0)
	}

	switch os.Args[1] {
	case "upload":
		UploadCmd()
		os.Exit(0)

	default:
		flag.Parse()
		fmt.Fprintf(os.Stderr, "unknown command: '%s'\n", os.Args[1])
		os.Exit(1)
	}
}
