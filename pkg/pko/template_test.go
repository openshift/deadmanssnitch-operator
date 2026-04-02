package pko

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

// deployPkoDir returns the path to the deploy_pko directory relative to the repo root.
// Tests are run from the repo root via `go test ./...`.
func deployPkoDir() string {
	// Walk up from the test file location to find the repo root
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "deploy_pko")); err == nil {
			return filepath.Join(dir, "deploy_pko")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "deploy_pko"
}

// templateFuncMap provides the functions available in PKO gotmpls.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"toJson": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}
}

// renderTemplate renders a gotmpl file with the given config data and returns the output.
func renderTemplate(t *testing.T, filename string, data map[string]interface{}) string {
	t.Helper()
	tmplPath := filepath.Join(deployPkoDir(), filename)
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read template %s: %v", tmplPath, err)
	}

	tmpl, err := template.New(filename).Funcs(templateFuncMap()).Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse template %s: %v", filename, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template %s: %v", filename, err)
	}
	return buf.String()
}

// parseYAMLDocument parses a single YAML document into a map.
func parseYAMLDocument(t *testing.T, data string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("failed to parse YAML: %v\nContent:\n%s", err, data)
	}
	return result
}

// getNestedString extracts a string from a nested map using dot-separated keys.
func getNestedString(m map[string]interface{}, keys ...string) string {
	var current interface{} = m
	for _, k := range keys {
		cm, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = cm[k]
	}
	s, _ := current.(string)
	return s
}

// defaultConfig returns a production-like config with populated silentAlertLegalEntityIds.
func defaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{
			"image":   "quay.io/example/deadmanssnitch-operator:test",
			"fedramp": "false",
			"silentAlertLegalEntityIds": []string{
				"1aV37K1VQv2zSStwSkdwBNOUBGI",
				"2Mo8exhgEA5ir1lsVypXUb9v902",
			},
			"deadmanssnitchOsdTags": []string{"hive-production"},
		},
	}
}

// emptyIdsConfig returns an integration-like config with empty silentAlertLegalEntityIds.
func emptyIdsConfig() map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{
			"image":                     "quay.io/example/deadmanssnitch-operator:test",
			"fedramp":                   "false",
			"silentAlertLegalEntityIds": []string{},
			"deadmanssnitchOsdTags":     []string{"hive-int"},
		},
	}
}

func TestDeploymentGotmpl(t *testing.T) {
	output := renderTemplate(t, "Deployment-deadmanssnitch-operator.yaml.gotmpl", defaultConfig())
	doc := parseYAMLDocument(t, output)

	if doc["kind"] != "Deployment" {
		t.Errorf("expected kind=Deployment, got %v", doc["kind"])
	}

	metadata := doc["metadata"].(map[string]interface{})
	if metadata["name"] != "deadmanssnitch-operator" {
		t.Errorf("expected name=deadmanssnitch-operator, got %v", metadata["name"])
	}
	if metadata["namespace"] != "deadmanssnitch-operator" {
		t.Errorf("expected namespace=deadmanssnitch-operator, got %v", metadata["namespace"])
	}

	annotations := metadata["annotations"].(map[string]interface{})
	if annotations["package-operator.run/phase"] != "deploy" {
		t.Errorf("expected phase=deploy, got %v", annotations["package-operator.run/phase"])
	}

	// Verify image substitution
	if !strings.Contains(output, "quay.io/example/deadmanssnitch-operator:test") {
		t.Error("expected config.image to be substituted in Deployment")
	}

	// Verify FEDRAMP env var
	if !strings.Contains(output, "value: 'false'") {
		t.Error("expected config.fedramp to be substituted in FEDRAMP env var")
	}
}

func TestDeploymentGotmpl_Fedramp(t *testing.T) {
	config := defaultConfig()
	config["config"].(map[string]interface{})["fedramp"] = "true"

	output := renderTemplate(t, "Deployment-deadmanssnitch-operator.yaml.gotmpl", config)

	if !strings.Contains(output, "value: 'true'") {
		t.Error("expected FEDRAMP env var to be 'true' when fedramp config is true")
	}
}

func TestDMSIGotmpl_WithSilentAlertIds(t *testing.T) {
	output := renderTemplate(t, "DeadmansSnitchIntegration-osd.yaml.gotmpl", defaultConfig())
	doc := parseYAMLDocument(t, output)

	if doc["kind"] != "DeadmansSnitchIntegration" {
		t.Errorf("expected kind=DeadmansSnitchIntegration, got %v", doc["kind"])
	}

	metadata := doc["metadata"].(map[string]interface{})
	if metadata["name"] != "osd" {
		t.Errorf("expected name=osd, got %v", metadata["name"])
	}

	annotations := metadata["annotations"].(map[string]interface{})
	if annotations["package-operator.run/phase"] != "integrations" {
		t.Errorf("expected phase=integrations, got %v", annotations["package-operator.run/phase"])
	}

	// Verify silentAlertLegalEntityIds renders as legal-entity-id matchExpression
	if !strings.Contains(output, "api.openshift.com/legal-entity-id") {
		t.Error("expected legal-entity-id matchExpression when silentAlertLegalEntityIds is populated")
	}

	// Verify tags renders as JSON array
	if !strings.Contains(output, `["hive-production"]`) {
		t.Error("expected deadmanssnitchOsdTags to render as JSON array")
	}

	// Verify support-exception is excluded (NotIn for osd, In for supportex)
	if !strings.Contains(output, "ext-managed.openshift.io/support-exception") {
		t.Error("expected support-exception matchExpression")
	}
}

func TestDMSIGotmpl_EmptySilentAlertIds(t *testing.T) {
	// This is the regression test for PR #271
	output := renderTemplate(t, "DeadmansSnitchIntegration-osd.yaml.gotmpl", emptyIdsConfig())

	// Should NOT contain legal-entity-id expression when list is empty
	if strings.Contains(output, "api.openshift.com/legal-entity-id") {
		t.Error("legal-entity-id matchExpression should NOT be rendered when silentAlertLegalEntityIds is empty")
	}

	// Should still be valid YAML
	parseYAMLDocument(t, output)

	// Tags should still render
	if !strings.Contains(output, `["hive-int"]`) {
		t.Error("expected deadmanssnitchOsdTags to render as JSON array")
	}
}

func TestDMSISupportexGotmpl(t *testing.T) {
	output := renderTemplate(t, "DeadmansSnitchIntegration-osd-supportex.yaml.gotmpl", defaultConfig())
	doc := parseYAMLDocument(t, output)

	metadata := doc["metadata"].(map[string]interface{})
	if metadata["name"] != "osd-supportex" {
		t.Errorf("expected name=osd-supportex, got %v", metadata["name"])
	}

	annotations := metadata["annotations"].(map[string]interface{})
	if annotations["package-operator.run/phase"] != "integrations" {
		t.Errorf("expected phase=integrations, got %v", annotations["package-operator.run/phase"])
	}

	// supportex variant should have support-exception with operator: In
	// (the osd variant has NotIn)
	spec := doc["spec"].(map[string]interface{})
	selector := spec["clusterDeploymentSelector"].(map[string]interface{})
	expressions := selector["matchExpressions"].([]interface{})

	foundSupportException := false
	for _, expr := range expressions {
		e := expr.(map[string]interface{})
		if e["key"] == "ext-managed.openshift.io/support-exception" {
			if e["operator"] != "In" {
				t.Errorf("expected support-exception operator=In for supportex, got %v", e["operator"])
			}
			foundSupportException = true
		}
	}
	if !foundSupportException {
		t.Error("expected support-exception matchExpression in supportex template")
	}

	// supportex should NOT have limited-support exclusion
	for _, expr := range expressions {
		e := expr.(map[string]interface{})
		if e["key"] == "api.openshift.com/limited-support" {
			t.Error("supportex should NOT have limited-support exclusion")
		}
	}
}

func TestDMSISupportexGotmpl_EmptySilentAlertIds(t *testing.T) {
	output := renderTemplate(t, "DeadmansSnitchIntegration-osd-supportex.yaml.gotmpl", emptyIdsConfig())

	if strings.Contains(output, "api.openshift.com/legal-entity-id") {
		t.Error("legal-entity-id matchExpression should NOT be rendered when silentAlertLegalEntityIds is empty")
	}

	parseYAMLDocument(t, output)
}

func TestAllGotmplsRenderWithDefaults(t *testing.T) {
	dir := deployPkoDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.gotmpl"))
	if err != nil {
		t.Fatalf("failed to glob gotmpls: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("no .gotmpl files found in deploy_pko/")
	}

	config := defaultConfig()
	for _, match := range matches {
		filename := filepath.Base(match)
		t.Run(filename, func(t *testing.T) {
			output := renderTemplate(t, filename, config)
			// Verify output is valid YAML
			parseYAMLDocument(t, output)
		})
	}
}
