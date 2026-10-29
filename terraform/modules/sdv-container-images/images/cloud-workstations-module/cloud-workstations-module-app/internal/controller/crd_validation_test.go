// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"sigs.k8s.io/yaml"
)

func TestHorizonCRDRulesRejectImmutableEdits(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..", ".."))
	crd := filepath.Join(root, "gitops", "modules", "cloud-workstations-module", "core", "crds", "workstations.yaml")
	raw, err := os.ReadFile(crd)
	if err != nil {
		t.Fatalf("read %s: %v", crd, err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	props := crdSpecProperties(t, doc)
	configRef := props["configRef"].(map[string]interface{})
	rules := configRef["x-kubernetes-validations"].([]interface{})
	rule := rules[0].(map[string]interface{})["rule"].(string)
	if rule != "self == oldSelf" {
		t.Fatalf("configRef rule = %q", rule)
	}
	if allowed, err := celBool(rule,
		map[string]interface{}{"self": map[string]string{"name": "b"}, "oldSelf": map[string]string{"name": "a"}}); err != nil {
		t.Fatal(err)
	} else if allowed {
		t.Fatal("configRef change was accepted by the CRD rule")
	}
	if allowed, err := celBool(rule,
		map[string]interface{}{"self": map[string]string{"name": "a"}, "oldSelf": map[string]string{"name": "a"}}); err != nil {
		t.Fatal(err)
	} else if !allowed {
		t.Fatal("unchanged configRef was rejected")
	}
}

func crdSpecProperties(t *testing.T, doc map[string]interface{}) map[string]interface{} {
	t.Helper()
	spec := doc["spec"].(map[string]interface{})
	versions := spec["versions"].([]interface{})
	version := versions[0].(map[string]interface{})
	schema := version["schema"].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})["spec"].(map[string]interface{})["properties"].(map[string]interface{})
	return props
}

func celBool(rule string, vars map[string]interface{}) (bool, error) {
	env, err := cel.NewEnv(
		cel.Variable("self", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("oldSelf", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		return false, err
	}
	ast, iss := env.Compile(rule)
	if iss != nil && iss.Err() != nil {
		return false, iss.Err()
	}
	prog, err := env.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := prog.Eval(vars)
	if err != nil {
		return false, err
	}
	value, ok := out.Value().(bool)
	if !ok {
		return false, err
	}
	return value, nil
}

func TestWorkstationCRDOmitsPowerState(t *testing.T) {
	doc := readCRD(t, "workstations.yaml")
	props := crdSpecProperties(t, doc)
	if _, ok := props["powerState"]; ok {
		t.Fatal("Workstation spec still declares powerState")
	}
	for _, column := range crdPrinterColumns(t, doc) {
		if column["jsonPath"] == ".spec.powerState" || column["name"] == "PowerState" {
			t.Fatalf("printer column still exposes power: %+v", column)
		}
	}
	users := props["users"].(map[string]interface{})
	if users["x-kubernetes-list-type"] != "set" {
		t.Fatalf("users list type = %v", users["x-kubernetes-list-type"])
	}
	pattern := users["items"].(map[string]interface{})["pattern"].(string)
	assertPrincipalPattern(t, pattern)
	status := crdStatusProperties(t, doc)
	if _, ok := status["externalRef"]; !ok {
		t.Fatal("workstation status is missing externalRef")
	}
}

func TestWorkstationConfigCRDRules(t *testing.T) {
	doc := readCRD(t, "workstationconfigs.yaml")
	props := crdSpecProperties(t, doc)
	if props["disableSsh"].(map[string]interface{})["default"] != true {
		t.Fatalf("disableSsh default = %v", props["disableSsh"].(map[string]interface{})["default"])
	}
	ports := props["allowedPorts"].(map[string]interface{})
	if asInt(t, ports["minItems"]) != 1 {
		t.Fatalf("allowedPorts minItems = %v", ports["minItems"])
	}
	defaults := ports["default"].([]interface{})
	if len(defaults) != 2 {
		t.Fatalf("allowedPorts default = %v", defaults)
	}
	rule := ports["x-kubernetes-validations"].([]interface{})[0].(map[string]interface{})["rule"].(string)
	if !evalDyn(t, rule, portList(22, 22)) || !evalDyn(t, rule, portList(80, 80)) || !evalDyn(t, rule, portList(1024, 65535)) {
		t.Fatal("legal port ranges were rejected")
	}
	if evalDyn(t, rule, portList(22, 80)) || evalDyn(t, rule, portList(21, 21)) || evalDyn(t, rule, portList(1023, 1024)) || evalDyn(t, rule, portList(70000, 70000)) {
		t.Fatal("illegal port range was accepted")
	}
	gce := props["host"].(map[string]interface{})["properties"].(map[string]interface{})["gceInstance"].(map[string]interface{})
	gceProps := gce["properties"].(map[string]interface{})
	if _, ok := gceProps["serviceAccountRef"]; ok {
		t.Fatal("host.gceInstance.serviceAccountRef must not be user-configurable")
	}
	pool := gceProps["poolSize"].(map[string]interface{})
	if asInt(t, pool["default"]) != 0 {
		t.Fatalf("poolSize default = %v", pool["default"])
	}
	admins := props["admins"].(map[string]interface{})
	if admins["x-kubernetes-list-type"] != "set" {
		t.Fatal("admins is not a set")
	}
	assertPrincipalPattern(t, admins["items"].(map[string]interface{})["pattern"].(string))
	rules := props["ephemeralDirectories"].(map[string]interface{})["items"].(map[string]interface{})["properties"].(map[string]interface{})["gcePD"].(map[string]interface{})["x-kubernetes-validations"].([]interface{})
	if len(rules) != 3 {
		t.Fatalf("ephemeral rules = %d", len(rules))
	}
	both := map[string]interface{}{"sourceSnapshot": "snap", "sourceImage": "image", "readOnly": true}
	readOnlyOnly := map[string]interface{}{"readOnly": true}
	snapshotOnly := map[string]interface{}{"sourceSnapshot": "snap"}
	snapshotReadOnly := map[string]interface{}{"sourceSnapshot": "snap", "readOnly": true}
	empty := map[string]interface{}{}
	for i, raw := range rules {
		rule := raw.(map[string]interface{})["rule"].(string)
		if i == 0 && evalDyn(t, rule, both) {
			t.Fatal("sourceSnapshot and sourceImage together were accepted")
		}
		if i == 1 && evalDyn(t, rule, readOnlyOnly) {
			t.Fatal("readOnly without sourceSnapshot was accepted")
		}
		if i == 2 && evalDyn(t, rule, snapshotOnly) {
			t.Fatal("sourceSnapshot without readOnly was accepted")
		}
		if !evalDyn(t, rule, snapshotReadOnly) || !evalDyn(t, rule, empty) {
			t.Fatalf("rule %d rejected a legal disk: %s", i, rule)
		}
	}
	status := crdStatusProperties(t, doc)
	if _, ok := status["externalRef"]; !ok {
		t.Fatal("config status is missing externalRef")
	}
	if _, ok := status["posture"]; !ok {
		t.Fatal("config status is missing posture")
	}
}

// Undeclared fields are pruned by the API server; without this, waits on Ready for the current generation never end.
func TestHorizonCRDConditionsKeepObservedGeneration(t *testing.T) {
	for _, name := range []string{"workstationclusters.yaml", "workstationconfigs.yaml", "workstations.yaml"} {
		status := crdStatusProperties(t, readCRD(t, name))
		conditions, ok := status["conditions"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: status.conditions is not declared", name)
		}
		items, ok := conditions["items"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: status.conditions.items is not declared", name)
		}
		props, ok := items["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: status.conditions.items.properties is not declared", name)
		}
		field, ok := props["observedGeneration"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: status.conditions[] does not declare observedGeneration", name)
		}
		if field["type"] != "integer" || field["format"] != "int64" {
			t.Fatalf("%s: conditions[].observedGeneration = %+v, want integer/int64", name, field)
		}
	}
}

func TestControllerRolePermissions(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "terraform", "modules", "base", "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, `resource "google_project_iam_custom_role" "cws_module_controller"`)
	if start < 0 {
		t.Fatal("controller custom role is missing")
	}
	rest := text[start:]
	end := strings.Index(rest[1:], "\nresource ")
	block := rest
	if end >= 0 {
		block = rest[:end+1]
	}
	granted := map[string]bool{}
	for _, match := range regexp.MustCompile(`"(workstations\.[^"]+)"`).FindAllStringSubmatch(block, -1) {
		granted[match[1]] = true
	}
	required := []string{
		"workstations.workstationConfigs.get",
		"workstations.workstationConfigs.update",
		"workstations.workstationConfigs.getIamPolicy",
		"workstations.workstationConfigs.setIamPolicy",
		"workstations.workstations.getIamPolicy",
		"workstations.workstations.setIamPolicy",
	}
	for _, permission := range required {
		if !granted[permission] {
			t.Fatalf("role is missing %s", permission)
		}
	}
	forbidden := []string{
		"workstations.workstations.start",
		"workstations.workstations.stop",
		"workstations.workstations.get",
		"workstations.operations.get",
	}
	for _, permission := range forbidden {
		if granted[permission] {
			t.Fatalf("role grants %s", permission)
		}
	}
	if len(granted) != len(required) {
		t.Fatalf("role permissions = %v", granted)
	}

	// The controller never actAs the host VM SA itself (that would re-grant the broad bypass this
	// role was created to avoid). Only Config Connector (creates/updates the VM) and the
	// Workstations service agent (starts/stops the VM) may actAs gke-cws-host-vm-sa.
	hostBindingStart := strings.Index(text, `resource "google_service_account_iam_member" "cws_host_sa_user"`)
	if hostBindingStart < 0 {
		t.Fatal("host service account actAs binding is missing")
	}
	hostBinding := text[hostBindingStart:]
	hostBindingEnd := strings.Index(hostBinding[1:], "\nresource ")
	if hostBindingEnd >= 0 {
		hostBinding = hostBinding[:hostBindingEnd+1]
	}
	for _, expected := range []string{
		"gke-cws-host-vm-sa@",
		`role               = "roles/iam.serviceAccountUser"`,
		"config_connector",
		"serviceAccount:gke-config-connector-sa@",
		"workstations_agent",
		"google_project_service_identity.workstations.email",
	} {
		if !strings.Contains(hostBinding, expected) {
			t.Fatalf("host actAs binding is missing %q", expected)
		}
	}
	if strings.Contains(hostBinding, "gke-cws-module-sa@") {
		t.Fatal("host actAs binding must not grant the controller SA itself")
	}

	// The workstations.googleapis.com service identity must exist before it can be granted actAs.
	if !strings.Contains(text, `resource "google_project_service_identity" "workstations"`) {
		t.Fatal("workstations service identity resource is missing")
	}

	envRaw, err := os.ReadFile(filepath.Join(root, "terraform", "env", "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	envText := string(envRaw)
	sa14Start := strings.Index(envText, "      sa14 = {")
	sa15Start := strings.Index(envText, "      sa15 = {")
	if sa14Start < 0 || sa15Start < 0 || sa15Start <= sa14Start {
		t.Fatal("sa14 or sa15 is missing")
	}
	sa14 := envText[sa14Start:sa15Start]
	if strings.Contains(sa14, "roles/workstations.admin") || strings.Contains(sa14, "roles/iam.serviceAccountUser") {
		t.Fatal("sa14 must not have broad predefined controller roles")
	}
	sa15 := envText[sa15Start:]
	sa15End := strings.Index(sa15, "\n    }\n  )")
	if sa15End >= 0 {
		sa15 = sa15[:sa15End]
	}
	if !strings.Contains(sa15, `account_id   = "gke-cws-host-vm-sa"`) || !strings.Contains(sa15, "gke_sas = []") {
		t.Fatal("sa15 host identity is not configured without Workload Identity")
	}
	hostRoles := map[string]bool{}
	for _, match := range regexp.MustCompile(`"(roles/[^"]+)"`).FindAllStringSubmatch(sa15, -1) {
		hostRoles[match[1]] = true
	}
	expectedHostRoles := []string{
		"roles/artifactregistry.reader",
		"roles/logging.logWriter",
		"roles/monitoring.metricWriter",
	}
	for _, role := range expectedHostRoles {
		if !hostRoles[role] {
			t.Fatalf("sa15 is missing %s", role)
		}
	}
	if len(hostRoles) != len(expectedHostRoles) {
		t.Fatalf("sa15 roles = %v", hostRoles)
	}
}

func readCRD(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "gitops", "modules", "cloud-workstations-module", "core", "crds", name))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func crdStatusProperties(t *testing.T, doc map[string]interface{}) map[string]interface{} {
	t.Helper()
	spec := doc["spec"].(map[string]interface{})
	version := spec["versions"].([]interface{})[0].(map[string]interface{})
	schema := version["schema"].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})
	return schema["properties"].(map[string]interface{})["status"].(map[string]interface{})["properties"].(map[string]interface{})
}

func crdPrinterColumns(t *testing.T, doc map[string]interface{}) []map[string]interface{} {
	t.Helper()
	version := doc["spec"].(map[string]interface{})["versions"].([]interface{})[0].(map[string]interface{})
	raw, _ := version["additionalPrinterColumns"].([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]interface{}))
	}
	return out
}

func assertPrincipalPattern(t *testing.T, pattern string) {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, ok := range []string{"user:alice@example.com", "group:eng@example.com", "serviceAccount:sa@p.iam.gserviceaccount.com", "domain:example.com", "principal://iam.googleapis.com/a", "principalSet://iam.googleapis.com/b"} {
		if !re.MatchString(ok) {
			t.Fatalf("pattern %s rejected %s", pattern, ok)
		}
	}
	if re.MatchString("alice@example.com") || re.MatchString("user:") {
		t.Fatalf("pattern %s accepted a bad principal", pattern)
	}
}

func portList(first, last int64) []map[string]interface{} {
	return []map[string]interface{}{{"first": first, "last": last}}
}

func evalDyn(t *testing.T, rule string, self interface{}) bool {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("self", cel.DynType))
	if err != nil {
		t.Fatal(err)
	}
	ast, iss := env.Compile(rule)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prog, err := env.Program(ast)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prog.Eval(map[string]interface{}{"self": self})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := out.Value().(bool)
	if !ok {
		t.Fatalf("rule %s returned %v", rule, out.Value())
	}
	return value
}

func asInt(t *testing.T, value interface{}) int {
	t.Helper()
	switch n := value.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case uint64:
		return int(n)
	default:
		t.Fatalf("not a number: %T %v", value, value)
		return 0
	}
}
