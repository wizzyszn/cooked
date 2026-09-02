package api

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestOpenAPIContractParsesAndContainsFoundationPaths(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var document struct {
		OpenAPI string                    `yaml:"openapi"`
		Paths   map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if document.OpenAPI == "" {
		t.Fatal("openapi version is required")
	}
	for _, path := range []string{
		"/health/live",
		"/health/ready",
		"/auth/register",
		"/auth/verify-email",
		"/auth/login",
		"/auth/refresh",
		"/auth/logout",
		"/auth/logout-all",
		"/auth/forgot-password",
		"/auth/reset-password",
		"/auth/google/start",
		"/auth/google/callback",
		"/auth/google/exchange",
		"/users/me",
		"/users/me/dietary-preferences",
		"/profiles/{username}",
		"/admin/users/{id}/roles/{role}",
	} {
		if _, ok := document.Paths[path]; !ok {
			t.Errorf("contract is missing %s", path)
		}
	}

	var rawDocument map[string]any
	if err := yaml.Unmarshal(raw, &rawDocument); err != nil {
		t.Fatalf("parse raw contract: %v", err)
	}
	assertLocalReferencesResolve(t, rawDocument, rawDocument)
	assertOperationIDsUnique(t, document.Paths)
}

func assertLocalReferencesResolve(t *testing.T, root map[string]any, current any) {
	t.Helper()
	switch value := current.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "$ref" {
				reference, ok := child.(string)
				if !ok || !strings.HasPrefix(reference, "#/") {
					t.Fatalf("only local OpenAPI references are supported, got %v", child)
				}
				if !localReferenceExists(root, reference) {
					t.Errorf("unresolved OpenAPI reference %s", reference)
				}
				continue
			}
			assertLocalReferencesResolve(t, root, child)
		}
	case []any:
		for _, child := range value {
			assertLocalReferencesResolve(t, root, child)
		}
	}
}

func localReferenceExists(root map[string]any, reference string) bool {
	var current any = root
	for _, rawPart := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(rawPart, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[part]
		if !ok {
			return false
		}
	}
	return true
}

func assertOperationIDsUnique(t *testing.T, paths map[string]map[string]any) {
	t.Helper()
	seen := make(map[string]string)
	for path, operations := range paths {
		for method, rawOperation := range operations {
			operation, ok := rawOperation.(map[string]any)
			if !ok {
				continue
			}
			operationID, _ := operation["operationId"].(string)
			if operationID == "" {
				t.Errorf("%s %s is missing operationId", strings.ToUpper(method), path)
				continue
			}
			if previous, exists := seen[operationID]; exists {
				t.Errorf("operationId %q is duplicated by %s and %s %s", operationID, previous, strings.ToUpper(method), path)
			}
			seen[operationID] = strings.ToUpper(method) + " " + path
		}
	}
}
