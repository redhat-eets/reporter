package main

import (
	"os"
	"sort"
	"testing"
)

func TestGetConf(t *testing.T) {
	CreateLoggers(true, true, "")

	// GetConf(conf_file string) (*proxyRestConfig)

	configFileErrors := map[string]string{
		"./test_yaml_files/proxy_config_bad0_nofile.yaml":                        "Non-existent config file path",
		"./test_yaml_files/proxy_config_bad1_nonyaml.yaml":                       "Non-yaml config file",
		"./test_yaml_files/proxy_config_bad2_missing_allowed_jira_projects.yaml": "allowed_jira_projects is missing",
		"./test_yaml_files/proxy_config_bad3_missing_tcp_listen_port.yaml":       "tcp_listen_port is missing",
		"./test_yaml_files/proxy_config_bad4_invalid_url.yaml":                   "Invalid URL",
		"./test_yaml_files/proxy_config_bad5_proxy_token_path.yaml":              "Invalid proxy_token_files path",
		"./test_yaml_files/proxy_config_bad6_jira_token_path.yaml":               "Invalid jira_token_files path",
		"./test_yaml_files/proxy_config_bad7_no_path_var.yaml":                   "No path variable",
		"./test_yaml_files/proxy_config_bad8_path_vars_diff.yaml":                "path variables are different",
	}

	// VERY dissapointing to see that this is the ONLY way to do an ordered map iteration in go :(
	// This is inefficient space-wise: you have to store the keys separately
	// and inefficient performance-wise: you have to do too many lookups in the map.
	var configFiles []string
	for cf := range configFileErrors {
		configFiles = append(configFiles, cf)
	}
	sort.Strings(configFiles)
	for _, path := range configFiles {
		proxyConfig := GetConf(path)
		if proxyConfig != nil {
			t.Fatalf(configFileErrors[path])
		}
	}

	proxyConfig := GetConf("./test_yaml_files/proxy_config_good.yaml")
	if proxyConfig == nil {
		t.Fatalf("GetConf should have returned successfully")
	}
}

func TestCreateLoggers(t *testing.T) {
	resetLoggers := func() {
		InfoLog = nil
		DebugLog = nil
		WarnLog = nil
		ErrorLog = nil
	}

	//CreateLoggers(verbose bool, log_to_stdout bool, log_to_file string)

	resetLoggers()
	CreateLoggers(true, true, "")
	if InfoLog == nil || DebugLog == nil || WarnLog == nil || ErrorLog == nil {
		t.Fatalf("CreateLoggers did not create all of the loggers: InfoLog %v, DebugLog %v, WarnLog %v, ErrorLog %v",
			InfoLog, DebugLog, WarnLog, ErrorLog)
	}
	if DebugLog.DebugLog == nil {
		t.Fatalf("CreateLoggers did not correctly create the DebugLogger")
	}

	resetLoggers()
	CreateLoggers(false, false, "/tmp/some_log_file.txt")
	if InfoLog == nil || DebugLog == nil || WarnLog == nil || ErrorLog == nil {
		t.Fatalf("CreateLoggers did not create all of the loggers: InfoLog %v, DebugLog %v, WarnLog %v, ErrorLog %v",
			InfoLog, DebugLog, WarnLog, ErrorLog)
	}
	if DebugLog.DebugLog != nil {
		t.Fatalf("CreateLoggers should not have created the DebugLogger")
	}
}

func TestParseCmdLine(t *testing.T) {
	resetGlobalArgs := func() {
		globalArgs.configfile = ""
		globalArgs.logfile = ""
		globalArgs.log_debug = false
		globalArgs.log_to_stdout = false
	}

	// Test not enough command line args
	resetGlobalArgs()
	os.Args = []string{"appName"}
	args := ParseCmdLine()
	if args != nil {
		t.Fatalf("ParseCmdLine with only 1 arg should return nil")
	}

	// Test unkown command line arg
	resetGlobalArgs()
	os.Args = []string{"appName", "--config", "configFilePath", "unknownArg"}
	args = ParseCmdLine()
	if args != nil {
		t.Fatalf("ParseCmdLine with unknown args should return nil")
	}

	// Test setting both --stdout and --logfile returns nil
	resetGlobalArgs()
	os.Args = []string{"appName", "--stdout", "--logfile", "someFilePath"}
	args = ParseCmdLine()
	if args != nil {
		t.Fatalf("ParseCmdLine with both --stdout and --logfile should return nil")
	}

	// Test the default values
	resetGlobalArgs()
	os.Args = []string{"appName", "--config", "configFilePath"}
	args = ParseCmdLine()
	if args == nil {
		t.Fatalf("ParseCmdLine should not return nil")
	}
	if args.log_debug != false ||
		args.log_to_stdout != false ||
		args.logfile != "./proxy_server.log" {
		t.Fatalf("ParseCmdLine incorrect default values: log_debug %v, log_to_stdout %v, logfile %s",
			args.log_debug, args.log_to_stdout, args.logfile)
	}

	// Test all values are parsed correctly
	resetGlobalArgs()
	os.Args = []string{"appName", "--v", "--logfile", "someLogFile", "--config", "configFilePath"}
	args = ParseCmdLine()
	if args == nil {
		t.Fatalf("ParseCmdLine should not return nil")
	}
	if args.log_debug != true ||
		args.log_to_stdout != false ||
		args.logfile != "someLogFile" ||
		args.configfile != "configFilePath" {
		t.Fatalf("ParseCmdLine incorrect values: log_debug %v, log_to_stdout %v, logfile %s, configfile %s",
			args.log_debug, args.log_to_stdout, args.logfile, args.configfile)
	}

	resetGlobalArgs()
	os.Args = []string{"appName", "--v", "--stdout", "--config", "configFilePath"}
	args = ParseCmdLine()
	if args == nil {
		t.Fatalf("ParseCmdLine should not return nil")
	}
	if args.log_debug != true ||
		args.log_to_stdout != true ||
		args.logfile != "" ||
		args.configfile != "configFilePath" {
		t.Fatalf("ParseCmdLine incorrect values: log_debug %v, log_to_stdout %v, logfile %s, configfile %s",
			args.log_debug, args.log_to_stdout, args.logfile, args.configfile)
	}
}
