package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func createCurrentConfig() {
	currentConfig = GetConf("./test_yaml_files/proxy_config_good.yaml")
}

func createClientRequest() *clientRequestType {
	clientRequest := &clientRequestType{
		HttpPath:       "/rest/api/2/issue/CNF-123",
		RequestQuery:   "fields=key,summary",
		RequestPattern: "/rest/api/2/issue/{id}",
		PatternVar:     "id",
		HttpMethod:     "GET",
		HttpHeaders:    map[string][]string{"Authorization": []string{"Bearer telco_v10n_ft.token"}},
		HttpPostBody:   "message body"}

	return clientRequest
}

func setup() {
	// Common test setup
	CreateLoggers(true, true, "")
	createCurrentConfig()
}

func TestVerifyPathMethods(t *testing.T) {
	setup()
	clientRequest := createClientRequest()

	// VerifyPathMethods(w http.ResponseWriter, clientRequest *clientRequestType) (result bool)

	clientRequest.HttpMethod = "PUT"
	w := httptest.NewRecorder()
	result := VerifyPathMethods(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyPathMethods should detect forbidden method on path")
	}
	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("VerifyPathMethods status code should be MethodNotAllowed")
	}

	clientRequest.RequestPattern = "/unknown/path"
	w = httptest.NewRecorder()
	result = VerifyPathMethods(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyPathMethods should detect forbidden path")
	}
	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("VerifyPathMethods status code [%d] should be Forbidden", resp.StatusCode)
	}

	// Verify extra path is not added at the end
	clientRequest.RequestPattern = "/rest/api/2/issue/{id}"
	clientRequest.PatternVar = "/rest/api/2/issue/CNF-123%2FmorePath"
	w = httptest.NewRecorder()
	result = VerifyPathMethods(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyPathMethods should detect forbidden path with extraneous suffix")
	}
	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("VerifyPathMethods extraneous suffix path should be Forbidden")
	}

	clientRequest = createClientRequest()
	w = httptest.NewRecorder()
	result = VerifyPathMethods(w, clientRequest)
	if result == false {
		t.Fatalf("VerifyPathMethods failure")
	}
}

func TestVerifyJiraProject(t *testing.T) {
	setup()
	clientRequest := createClientRequest()

	// VerifyJiraProject(w http.ResponseWriter, clientRequest *clientRequestType) (result bool)

	clientRequest.PatternVar = "FNC-567"
	w := httptest.NewRecorder()
	result := VerifyJiraProject(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyJiraProject invalid project")
	}
	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("VerifyJiraProject invalid project status code should be Forbidden")
	}

	clientRequest.PatternVar = "CNF-789"
	w = httptest.NewRecorder()
	result = VerifyJiraProject(w, clientRequest)
	if result == false {
		t.Fatalf("VerifyJiraProject should not fail for a valid project")
	}

	clientRequest.PatternVar = ""
	w = httptest.NewRecorder()
	result = VerifyJiraProject(w, clientRequest)
	if result == false {
		t.Fatalf("VerifyJiraProject should not fail if PatternVar is empty")
	}
}

func TestVerifyProxyToken(t *testing.T) {
	setup()
	clientRequest := createClientRequest()

	// VerifyProxyToken(w http.ResponseWriter, clientRequest *clientRequestType) (status bool)

	w := httptest.NewRecorder()
	result := VerifyProxyToken(w, clientRequest)
	if result == false {
		t.Fatalf("VerifyProxyToken should pass")
	}

	clientRequest.HttpHeaders = nil
	w = httptest.NewRecorder()
	result = VerifyProxyToken(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyProxyToken should fail with an empty Auth header")
	}
	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("VerifyProxyToken empty Auth header status code should be Unauthorized")
	}

	clientRequest.HttpHeaders = map[string][]string{"Authorization": []string{"Bearer unknownToken"}}
	w = httptest.NewRecorder()
	result = VerifyProxyToken(w, clientRequest)
	if result == true {
		t.Fatalf("VerifyProxyToken should fail with an unknown proxy token")
	}
	resp = w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("VerifyProxyToken unknown proxy token status code should be Unauthorized")
	}
}

func TestGetClientRequestInfo(t *testing.T) {
	setup()

	// GetClientRequestInfo(req *http.Request) (request *clientRequestType)

	req := httptest.NewRequest("GET", "/rest/api/2/issue/CNF-123", nil)
	clientRequest := GetClientRequestInfo(req)
	if clientRequest == nil {
		t.Fatalf("GetClientRequestInfo should succeed")
	}
}

func TestHandleClientRequest(t *testing.T) {
	setup()

	// HandleClientRequest(w http.ResponseWriter, req *http.Request)

	// Test MethodNotAllowed
	req := httptest.NewRequest("PUT", "http://localhost:10000/rest/api/2/issue/CNF-123", nil)
	req.Pattern = "/rest/api/2/issue/{id}"
	w := httptest.NewRecorder()
	HandleClientRequest(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("HandleClientRequest status code [%d] should be MethodNotAllowed", resp.StatusCode)
	}

	// Test Forbidden path
	req = httptest.NewRequest("GET", "http://localhost:10000/forbidden/path", nil)
	req.Pattern = "/forbidden/path"
	w = httptest.NewRecorder()
	HandleClientRequest(w, req)
	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("HandleClientRequest status code [%d] should be Forbidden", resp.StatusCode)
	}

	// Test Forbidden project
	req = httptest.NewRequest("GET", "http://localhost:10000/rest/api/2/issue/ABC-123", nil)
	req.Pattern = "/rest/api/2/issue/{id}"
	req.SetPathValue("id", "ABC-123") // this makes request.PathValue() work
	w = httptest.NewRecorder()
	HandleClientRequest(w, req)
	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("HandleClientRequest status code [%d] should be Forbidden for project", resp.StatusCode)
	}

	// Test Unauthorized proxy token
	req = httptest.NewRequest("GET", "http://localhost:10000/rest/api/2/issue/CNF-123", nil)
	req.Pattern = "/rest/api/2/issue/{id}"
	req.SetPathValue("id", "CNF-123") // this makes request.PathValue() work
	req.Header["Authorization"] = []string{"Bearer badToken"}
	w = httptest.NewRecorder()
	HandleClientRequest(w, req)
	resp = w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("HandleClientRequest status code [%d] should be Unauthorized for project", resp.StatusCode)
	}

	// Test Success
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	currentConfig.JiraURLstr = testServer.URL
	currentConfig.JiraURLbase, _ = url.Parse(testServer.URL)

	req = httptest.NewRequest("GET", "http://localhost:10000/rest/api/2/issue/CNF-123", nil)
	req.Pattern = "/rest/api/2/issue/{id}"
	req.SetPathValue("id", "CNF-123") // this makes request.PathValue() work
	req.Header["Authorization"] = []string{"Bearer telco_v10n_ft.token"}
	w = httptest.NewRecorder()
	HandleClientRequest(w, req)
	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HandleClientRequest status code [%d] should be ok", resp.StatusCode)
	}
}
