package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOpenAPISpec_Parses guarantees the embedded YAML is structurally
// valid. It also runs at package init via the JSON cache, but an explicit
// test keeps the failure message intelligible.
func TestOpenAPISpec_Parses(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}
	if _, ok := doc["openapi"]; !ok {
		t.Fatal("openapi.yaml missing top-level 'openapi' key")
	}
	if _, ok := doc["paths"]; !ok {
		t.Fatal("openapi.yaml missing top-level 'paths' key")
	}
}

func TestOpenAPISpec_ServedAsYAML(t *testing.T) {
	srv := testServer(t)
	req, _ := http.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Errorf("content-type = %q, want */yaml", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, openapiYAML) {
		t.Error("served YAML does not match embedded bytes")
	}
}

func TestOpenAPISpec_ServedAsJSON(t *testing.T) {
	srv := testServer(t)
	req, _ := http.NewRequest(http.MethodGet, "/openapi.json", nil)
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		t.Errorf("content-type = %q, want */json", ct)
	}
	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if _, ok := doc["paths"]; !ok {
		t.Error("JSON spec missing 'paths'")
	}
}

func TestDocsPage_RendersHTML(t *testing.T) {
	srv := testServer(t)
	req, _ := http.NewRequest(http.MethodGet, "/docs", nil)
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(bytes.ToLower(body), []byte("swagger-ui")) {
		t.Error("body does not mention swagger-ui")
	}
	if !bytes.Contains(body, []byte("/openapi.json")) {
		t.Error("body does not reference /openapi.json")
	}
}

// TestOpenAPISpec_NoRouteDrift is the contract test: every route Fiber
// knows about must appear in the spec, and every spec path must map to a
// registered route. Fiber's colon-style path params (`:id`) are normalized
// to OpenAPI's brace style (`{id}`). HEAD and OPTIONS are auto-registered
// by Fiber and excluded from the comparison.
func TestOpenAPISpec_NoRouteDrift(t *testing.T) {
	srv, _ := testServerWithRegistry(t)

	fiberRoutes := make(map[string]struct{})
	colonParam := regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	for _, r := range srv.app.GetRoutes(true) {
		method := strings.ToUpper(r.Method)
		if method == http.MethodHead || method == http.MethodOptions {
			continue
		}
		path := colonParam.ReplaceAllString(r.Path, "{$1}")
		fiberRoutes[method+" "+path] = struct{}{}
	}

	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	specRoutes := make(map[string]struct{})
	for path, methods := range doc.Paths {
		for method := range methods {
			m := strings.ToUpper(method)
			// Skip non-operation keys like "parameters", "summary", etc.
			switch m {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
			default:
				continue
			}
			specRoutes[m+" "+path] = struct{}{}
		}
	}

	var missingFromSpec, extraInSpec []string
	for r := range fiberRoutes {
		if _, ok := specRoutes[r]; !ok {
			missingFromSpec = append(missingFromSpec, r)
		}
	}
	for r := range specRoutes {
		if _, ok := fiberRoutes[r]; !ok {
			extraInSpec = append(extraInSpec, r)
		}
	}
	sort.Strings(missingFromSpec)
	sort.Strings(extraInSpec)

	if len(missingFromSpec) > 0 {
		t.Errorf("routes registered but missing from openapi.yaml:\n  %s",
			strings.Join(missingFromSpec, "\n  "))
	}
	if len(extraInSpec) > 0 {
		t.Errorf("paths declared in openapi.yaml but not registered:\n  %s",
			strings.Join(extraInSpec, "\n  "))
	}
}
