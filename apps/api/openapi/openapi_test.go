package openapi

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type document struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]operation `yaml:"paths"`
}

func TestContractMatchesRuntimeRoutes(t *testing.T) {
	payload, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var contract document
	if err := yaml.Unmarshal(payload, &contract); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	source, err := os.ReadFile("../internal/httpapi/routes.go")
	if err != nil {
		t.Fatalf("read runtime routes: %v", err)
	}
	pattern := regexp.MustCompile(
		`router\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) ([^"]+)"`)
	runtime := make(map[string]bool)
	for _, match := range pattern.FindAllStringSubmatch(string(source), -1) {
		runtime[match[1]+" "+match[2]] = true
	}
	documented := make(map[string]bool)
	for path, methods := range contract.Paths {
		for method := range methods {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}
	for route := range runtime {
		if !documented[route] {
			t.Errorf("runtime route is undocumented: %s", route)
		}
	}
	for route := range documented {
		if !runtime[route] {
			t.Errorf("documented operation has no runtime route: %s", route)
		}
	}
}

type operation struct {
	OperationID string `yaml:"operationId"`
}

func TestContractParsesAndOperationIDsAreUnique(t *testing.T) {
	payload, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var contract document
	if err := yaml.Unmarshal(payload, &contract); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if contract.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI version=%q", contract.OpenAPI)
	}
	if len(contract.Paths) != 33 {
		t.Fatalf("path count=%d want=33", len(contract.Paths))
	}
	seen := make(map[string]string)
	for path, methods := range contract.Paths {
		for method, operation := range methods {
			if operation.OperationID == "" {
				t.Fatalf("%s %s is missing operationId", method, path)
			}
			if previous, duplicate := seen[operation.OperationID]; duplicate {
				t.Fatalf("duplicate operationId %q at %s and %s",
					operation.OperationID, previous, path)
			}
			seen[operation.OperationID] = path
		}
	}
}
