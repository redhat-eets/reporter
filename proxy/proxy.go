package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

//
// Some example queries
// curl -H "Authorization: Bearer <access token str>" --get https://issues.redhat.com/rest/api/2/filter/12436327 | jq
// curl -H "Authorization: Bearer <access token str>" --get https://issues.redhat.com/rest/api/2/issue/CNF-9184?fields=key,summary | jq
// curl -H "Authorization: Bearer <access token str>" --get https://issues.redhat.com/rest/api/2/search?jql="key=CNF-9184"#fields=key,summary | jq
// curl -H "Authorization: Bearer <access token str>" -H "Content-Type: application/json; charset=utf-8" --data "{\"jql\": \"key=CNF-9184\", \"fields\": [\"key\", \"summary\"]}" https://issues.redhat.com/rest/api/2/search | jq
// curl -H "Authorization: Bearer <access token str>" -H "Content-Type: application/json; charset=utf-8" --data "{\"jql\": \"project in (CNF) and Issuetype = Story and fixVersion = openshift-4.16 and labels in (TELCO-V10N-ST)\", \"fields\": [\"key\", \"summary\"]}" https://issues.redhat.com/rest/api/2/search | jq
//
// curl -H "Authorization: Bearer telco_v10n_ft.token" --get http://localhost:9999/rest/api/2/issues/CNN-9184 => 404 page not found
// curl -H "Authorization: Bearer telco_v10n_ft.token" -X PUT http://localhost:9999/rest/api/2/issue/CNN-9184 => 405: Method not allowed on this path
// curl -H "Authorization: Bearer telco_v10n_ft.token" --get http://localhost:9999/rest/api/2/issue/CNN-9184 => 403: Forbidden Jira Project
// curl -H "Authorization: Bearer bad.token" --get http://localhost:9999/rest/api/2/issue/CNN-9184 => 401: Invalid Proxy Access Token
// curl --get http://localhost:9999/rest/api/2/issue/CNN-9184 => 401: Invalid Proxy Access Token

type proxyRestConfig struct {
	TcpListenPort    int    `yaml:"tcp_listen_port,omitempty"`
	JiraURLstr       string `yaml:"jira_url"`
	AllowedJiraPaths []struct {
		Path    string   `yaml:"path"`
		Methods []string `yaml:"methods"`
	} `yaml:"allowed_jira_paths"`
	AllowedJiraProjects []string `yaml:"allowed_jira_projects,omitempty"`
	ProxyTokenFiles     []string `yaml:"proxy_token_files"`
	JiraTokenFile       string   `yaml:"jira_token_file"`
	PathVarStr          string
	JiraURLbase         *url.URL
	// Private fields
	proxyTokens []string
	jiraToken   string
}

// Structure to hold request info received from client.
type clientRequestType struct {
	HttpPath       string
	RequestQuery   string
	RequestPattern string
	PatternVar     string
	HttpMethod     string
	HttpHeaders    map[string][]string
	HttpPostBody   string
}

// Structure to hold response info received from Jira.
type jiraResponseType struct {
	HttpStatus   int
	HttpError    string
	HttpRespBody string
}

var currentConfig *proxyRestConfig = nil

const (
	AuthHeader = "Authorization"
	BearerStr  = "Bearer "
)

// Given a clientRequest, proxy a message to Jira and send the response back
// via the jiraResponseChannel. This will be launched in its own thread.
func SendToJira(clientRequest *clientRequestType, jiraResponseChannel chan *jiraResponseType) {
	var jiraResponse jiraResponseType

	//
	// Send a request to Jira
	//
	forceQuery := false
	if clientRequest.RequestQuery != "" {
		forceQuery = true
	}
	u := url.URL{
		Scheme:     currentConfig.JiraURLbase.Scheme,
		Host:       currentConfig.JiraURLbase.Host,
		Path:       clientRequest.HttpPath,
		RawQuery:   clientRequest.RequestQuery,
		ForceQuery: forceQuery}

	httpReq, err := http.NewRequest(clientRequest.HttpMethod,
		u.String(),
		strings.NewReader(clientRequest.HttpPostBody))
	if err != nil {
		ErrorLog.Printf("NewRequest error %v\n", err)
		jiraResponse.HttpStatus = http.StatusInternalServerError
		jiraResponse.HttpError = err.Error()
		jiraResponseChannel <- &jiraResponse

		return
	}

	// Add the headers
	for header, value := range clientRequest.HttpHeaders {
		if header == AuthHeader {
			continue
		}
		DebugLog.Printf("Adding header %s : %s\n", header, value)
		// TODO for now only add the first header value
		httpReq.Header.Add(header, value[0])
	}
	httpReq.Header.Add(AuthHeader, fmt.Sprintf("Bearer %s", currentConfig.jiraToken))

	//
	// Process the response
	//
	DebugLog.Printf("Sending [%s] to Proxy %s\n", httpReq.Method, u.String())
	client := http.Client{}
	httpResp, err := client.Do(httpReq)

	if err != nil {
		ErrorLog.Printf("client.Do() error %v\n", err)
		jiraResponse.HttpStatus = http.StatusInternalServerError
		jiraResponse.HttpError = err.Error()
		jiraResponseChannel <- &jiraResponse

		return
	}
	DebugLog.Printf("Proxy response received: %d\n", httpResp.StatusCode)

	defer httpResp.Body.Close()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		ErrorLog.Printf("ReadAll(httpResp.Body) error %v\n", err)
		jiraResponse.HttpStatus = http.StatusInternalServerError
		jiraResponse.HttpError = err.Error()
		jiraResponseChannel <- &jiraResponse

		return
	}

	jiraResponse.HttpStatus = httpResp.StatusCode
	jiraResponse.HttpRespBody = string(bodyBytes)

	// Return the jira response on the jiraResponseChannel
	jiraResponseChannel <- &jiraResponse
}

func VerifyPathMethods(w http.ResponseWriter, clientRequest *clientRequestType) (result bool) {
	// Check for trailing paths tricked by "%2F", only if the pattern ends with the {var}
	if strings.HasSuffix(clientRequest.RequestPattern, "}") {
		if strings.Contains(clientRequest.PatternVar, "/") || strings.Contains(clientRequest.PatternVar, "%2F") {
			ErrorLog.Printf("Path not allowed [%s], patternVar [%s]",
				clientRequest.HttpPath, clientRequest.PatternVar)
			http.Error(w, "Forbidden Path", http.StatusForbidden)
			return false
		}
	}

	pathFound := false
	for _, pathMethod := range currentConfig.AllowedJiraPaths {
		if clientRequest.RequestPattern == pathMethod.Path {
			pathFound = true
			// Reply with 405 "Method Not Allowed" if not allowed
			if slices.Contains(pathMethod.Methods, clientRequest.HttpMethod) == false {
				ErrorLog.Printf("Method [%s] not allowed on this path [%s]",
					clientRequest.HttpMethod, clientRequest.HttpPath)
				http.Error(w, "Method not allowed on this path", http.StatusMethodNotAllowed)
				return false
			}
		}
	}

	// Reply with 403 http.StatusForbidden if not allowed
	if !pathFound {
		ErrorLog.Printf("Path not allowed [%s], reqPattern [%s]",
			clientRequest.HttpPath, clientRequest.RequestPattern)
		http.Error(w, "Forbidden Path", http.StatusForbidden)
		return false
	}

	return true
}

func VerifyJiraProject(w http.ResponseWriter, clientRequest *clientRequestType) (result bool) {
	if clientRequest.PatternVar != "" {
		for _, allowedJiraProject := range currentConfig.AllowedJiraProjects {
			if strings.Index(clientRequest.PatternVar, allowedJiraProject) < 0 {
				ErrorLog.Printf("Forbidden Jira Project [%s]\n", clientRequest.PatternVar)
				http.Error(w, "Forbidden Jira Project", http.StatusForbidden)
				return false
			}
		}
	}

	return true
}

func VerifyProxyToken(w http.ResponseWriter, clientRequest *clientRequestType) (status bool) {
	authHeaders := clientRequest.HttpHeaders[AuthHeader]
	if len(authHeaders) == 0 || authHeaders[0] == "" {
		ErrorLog.Printf("Client Error, missing mandatory Authorization header\n")
		http.Error(w, "Invalid Proxy Access Token", http.StatusUnauthorized)
		return false
	}

	for _, authHeader := range authHeaders {
		startIndex := strings.Index(authHeader, BearerStr)
		if startIndex >= 0 {
			startIndex = len(BearerStr)
		} else {
			startIndex = 0
		}
		if slices.Contains(currentConfig.proxyTokens, authHeader[startIndex:]) {
			return true
		}
	}

	ErrorLog.Printf("Authorization Header did not match a configured ProxyToken\n")
	http.Error(w, "Invalid Proxy Access Token", http.StatusUnauthorized)

	return false
}

func GetClientRequestInfo(req *http.Request) (request *clientRequestType) {
	// Create and populate a new client request struct
	clientRequest := clientRequestType{}
	clientRequest.HttpPath = strings.Clone(req.URL.Path)
	clientRequest.RequestQuery = strings.Clone(req.URL.RawQuery)
	clientRequest.HttpMethod = strings.Clone(req.Method)
	// This is the path with the variables that matched: "/rest/api/2/issue/{id}"
	clientRequest.RequestPattern = strings.Clone(req.Pattern)
	// This is the value of the path variable: "/rest/api/2/issue/CNF-123" => "CNF-123"
	clientRequest.PatternVar = strings.Clone(req.PathValue(currentConfig.PathVarStr))

	// Simple header copy, dont need Authorization, etc
	clientRequest.HttpHeaders = make(map[string][]string)
	for key, value := range req.Header {
		if key == "Content-Length" {
			continue
		}
		clientRequest.HttpHeaders[key] = make([]string, len(value))
		copy(clientRequest.HttpHeaders[key], value)
	}

	postBodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		// TODO how to react?
		ErrorLog.Printf("GetClientRequestInfo error reading request body #%v", err)
	}
	clientRequest.HttpPostBody = string(postBodyBytes)

	return &clientRequest
}

// A function to handle the Client HTTP Requests.
// This will receive the request from the client, process it and concurrently
// call make a call to Jira, then send the response back to the client.
func HandleClientRequest(w http.ResponseWriter, req *http.Request) {
	clientRequest := GetClientRequestInfo(req)

	DebugLog.Printf("Endpoint Hit: %s => %s from: %s\n",
		clientRequest.HttpMethod, clientRequest.HttpPath, req.RemoteAddr)

	// Check the path is allowed, and the method on this path is allowed
	if !VerifyPathMethods(w, clientRequest) {
		return
	}

	// Check the Jira project is allowed, reply with 403 http.StatusForbidden
	if !VerifyJiraProject(w, clientRequest) {
		return
	}

	if !VerifyProxyToken(w, clientRequest) {
		return
	}

	// Make a channel to proxy the client request to Jira
	jiraResponseChannel := make(chan *jiraResponseType)
	go SendToJira(clientRequest, jiraResponseChannel)

	// Wait for the response from Jira
	jiraResponse := <-jiraResponseChannel

	// https://en.wikipedia.org/wiki/List_of_HTTP_status_codes
	if jiraResponse.HttpStatus < 200 || jiraResponse.HttpStatus > 299 {
		http.Error(w, jiraResponse.HttpError, jiraResponse.HttpStatus)
		return
	}

	// Send the response back to the client
	w.WriteHeader(jiraResponse.HttpStatus)
	if jiraResponse.HttpRespBody != "" {
		fmt.Fprintf(w, jiraResponse.HttpRespBody)
	}
}

func RestProxy(config *proxyRestConfig) {
	currentConfig = config

	// Iterate the paths, and call HandleFunc() with each entry
	for _, allowedJiraPaths := range config.AllowedJiraPaths {
		http.HandleFunc(allowedJiraPaths.Path, HandleClientRequest)
	}

	InfoLog.Printf("Starting server on port %d\n", config.TcpListenPort)
	InfoLog.Printf("Allowed paths/methods:\n")
	for _, pathMethod := range currentConfig.AllowedJiraPaths {
		InfoLog.Printf("\t %s => %v", pathMethod.Path, pathMethod.Methods)
	}
	InfoLog.Printf("Allowed Jira projects: %v\n", config.AllowedJiraProjects)
	InfoLog.Printf("Proxy URL: %s\n", config.JiraURLstr)

	tcpPortStr := fmt.Sprintf(":%d", config.TcpListenPort)
	ErrorLog.Fatal(http.ListenAndServe(tcpPortStr, nil))
}
