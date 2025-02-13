package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type CmdLineArgs struct {
	log_debug     bool
	log_to_stdout bool
	logfile       string
	configfile    string
}

// Loggers configuration

type DebugLogger struct {
	DebugLog *log.Logger
}

func NewDebugLog(out io.Writer, prefix string, flag int, verbose bool) *DebugLogger {
	dl := new(DebugLogger)
	if verbose {
		dl.DebugLog = log.New(out, prefix, flag)
	} else {
		dl.DebugLog = nil
	}

	return dl
}

func (dl *DebugLogger) Printf(format string, v ...any) {
	if dl.DebugLog == nil {
		// If verbose is not enabled, dont log debug messages
		return
	}
	dl.DebugLog.Printf(format, v...)
}

var (
	Version    = "dev"
	CommitHash = "unknown"
	InfoLog    *log.Logger
	DebugLog   *DebugLogger
	WarnLog    *log.Logger
	ErrorLog   *log.Logger
	globalArgs = CmdLineArgs{}
)

func GetPathVar(pathPattern string) string {
	startIndex := strings.Index(pathPattern, "{")
	endIndex := strings.Index(pathPattern, "}")
	if (startIndex < 0 || endIndex < 0) || (endIndex < startIndex) {
		ErrorLog.Printf("Invalid allowed_jira_paths.path %s it should have a var such as {id}",
			pathPattern)
		return ""
	}

	return pathPattern[startIndex+1 : endIndex]
}

func CheckEmptyParam(param any, paramName string) bool {
	if reflect.ValueOf(param).IsZero() {
		ErrorLog.Printf("Config param [%s] cannot be empty\n", paramName)
		return true
	}

	return false
}

func GetConf(conf_file string) *proxyRestConfig {
	yamlFile, err := os.ReadFile(conf_file)
	if err != nil {
		ErrorLog.Printf("yamlFile.Get err   #%v ", err)
		return nil
	}

	config := proxyRestConfig{}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		ErrorLog.Printf("Unmarshal: %v", err)
		return nil
	}

	// Check all config params are set
	if CheckEmptyParam(config.TcpListenPort, "tcp_listen_port") {
		return nil
	}
	if CheckEmptyParam(config.JiraURLstr, "jira_url") {
		return nil
	}
	if CheckEmptyParam(config.AllowedJiraProjects, "allowed_jira_projects") {
		return nil
	}
	if CheckEmptyParam(config.ProxyTokenFiles, "proxy_token_files") {
		return nil
	}
	if CheckEmptyParam(config.JiraTokenFile, "jira_token_file") {
		return nil
	}
	if CheckEmptyParam(config.AllowedJiraProjects, "allowed_jira_paths") {
		return nil
	}
	for _, pathMethod := range config.AllowedJiraPaths {
		if CheckEmptyParam(pathMethod.Path, "allowed_jira_paths.path") {
			return nil
		}
		if CheckEmptyParam(pathMethod.Methods, "allowed_jira_paths.methods") {
			return nil
		}
	}

	// Create a URL based on the UrlStr
	config.JiraURLbase, err = url.Parse(config.JiraURLstr)
	if err != nil {
		ErrorLog.Printf("Invalid URL %s, err #%v ", config.JiraURLstr, err)
		return nil
	}

	// Read the Proxy Token file values
	for _, proxyTokenFile := range config.ProxyTokenFiles {
		proxyToken, err := os.ReadFile(proxyTokenFile)
		if err != nil {
			ErrorLog.Printf("Error reading proxyTokenFile %s, err #%v ", proxyTokenFile, err)
			return nil
		}
		config.proxyTokens = append(config.proxyTokens, strings.TrimSpace(string(proxyToken)))
	}

	// Read the Jira Token file value
	jiraTokenBytes, err := os.ReadFile(config.JiraTokenFile)
	if err != nil {
		ErrorLog.Printf("Error reading jiraTokenFile %s, err #%v ", config.JiraTokenFile, err)
		return nil
	}
	config.jiraToken = strings.TrimSpace(string(jiraTokenBytes))

	// Parse the path var pattern name, normally {id}
	config.PathVarStr = GetPathVar(config.AllowedJiraPaths[0].Path)
	if config.PathVarStr == "" {
		return nil
	}
	// Make sure the rest are the same
	for _, allowedJiraPath := range config.AllowedJiraPaths[1:] {
		otherPathVar := GetPathVar(allowedJiraPath.Path)
		if otherPathVar == "" {
			return nil
		}
		if config.PathVarStr != otherPathVar {
			ErrorLog.Printf("All path var patterns must be the same %s != %s\n", config.PathVarStr, otherPathVar)
			return nil
		}
	}

	//DebugLog.Printf("Config: %v\n", config)
	DebugLog.Printf("Config.TcpListenPort:        %d\n", config.TcpListenPort)
	DebugLog.Printf("Config.JiraURLstr:           %s\n", config.JiraURLstr)
	DebugLog.Printf("Config.AllowedJiraProjects:  %v\n", config.AllowedJiraProjects)
	DebugLog.Printf("Config.ProxyTokenFiles:      %s\n", config.ProxyTokenFiles)
	DebugLog.Printf("Config.JiraTokenFile:        %s\n", config.JiraTokenFile)
	DebugLog.Printf("Config.PathVarStr:           %s\n", config.PathVarStr)
	DebugLog.Printf("Config.JiraURLbase:          %v\n", config.JiraURLbase)

	return &config
}

func CreateLoggers(verbose bool, log_to_stdout bool, log_to_file string) {
	logFlags := log.Ldate | log.Lmsgprefix | log.Ltime

	// ParseCmdLine() guarantees that at least one of and only one of
	// log_to_stdout or log_to_file will be set

	if log_to_stdout {
		InfoLog = log.New(os.Stdout, "[INFO]  ", logFlags)
		WarnLog = log.New(os.Stdout, "[WARN]  ", logFlags)
		ErrorLog = log.New(os.Stderr, "[ERROR] ", logFlags)
		DebugLog = NewDebugLog(os.Stdout, "[DEBUG] ", logFlags, verbose)
	} else if log_to_file != "" {
		f, err := os.OpenFile(log_to_file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening log file: %v", err)
		}
		defer f.Close()

		//log.SetOutput(f)
		InfoLog = log.New(f, "[INFO]  ", logFlags)
		WarnLog = log.New(f, "[WARN]  ", logFlags)
		ErrorLog = log.New(f, "[ERROR] ", logFlags)
		DebugLog = NewDebugLog(f, "[DEBUG] ", logFlags, verbose)
	}
}

func init() {
	// Called after variable initialization and before main()
	flag.StringVar(&globalArgs.configfile, "config", "", "YAML config file path")
	flag.StringVar(&globalArgs.logfile, "logfile", "", "log to a logfile")
	flag.BoolVar(&globalArgs.log_debug, "v", false, "Debug/Verbose logging")
	flag.BoolVar(&globalArgs.log_to_stdout, "stdout", false, "Log to stdout, mutually exclusive with the --logfile option")
}

func ParseCmdLine() *CmdLineArgs {
	flag.Parse()

	if globalArgs.configfile == "" {
		fmt.Printf("ERROR The config file must be specified.\n")
		PrintUsage()
		return nil
	}

	if globalArgs.log_to_stdout && globalArgs.logfile != "" {
		fmt.Printf("ERROR Only 1 of --stdout and --logfile can be set \n")
		PrintUsage()
		return nil
	}

	if len(flag.Args()) > 0 {
		fmt.Printf("ERROR unknown extra args %v\n", flag.Args())
		PrintUsage()
		return nil
	}

	if !globalArgs.log_to_stdout && globalArgs.logfile == "" {
		globalArgs.logfile = "./proxy_server.log"
	}

	return &globalArgs
}

func PrintUsage() {
	fmt.Printf("Usage: proxy_rest --config <yaml config file path>")
	fmt.Printf("Optional arguments:\n")
	fmt.Printf("\t --v debug logging, Default false\n")
	fmt.Printf("\t --stdout to stdout, Default false\n")
	fmt.Printf("\t --logfile <path to logfile> log to a logfile, Default ./proxy_server.log\n")
}

func main() {
	args := ParseCmdLine()
	if args == nil {
		os.Exit(1)
	}

	CreateLoggers(args.log_debug, args.log_to_stdout, args.logfile)

	InfoLog.Printf("Proxy version %s, commit %s", Version, CommitHash)

	config := GetConf(args.configfile)
	if config == nil {
		return
	}

	// Start the Rest Proxy server, this call blocks until the server is stopped.
	// Example command to call the server:
	// curl --get http://localhost:10000/rest/api/2/issue/123
	RestProxy(config)
}
