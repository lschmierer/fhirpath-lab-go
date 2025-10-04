package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// minimal helpers to navigate Parameters JSON
type param struct {
	Name        string  `json:"name"`
	ValueString *string `json:"valueString,omitempty"`
	Resource    any     `json:"resource,omitempty"`
	Part        []param `json:"part,omitempty"`
	Extension   []struct {
		Url         string  `json:"url"`
		ValueString *string `json:"valueString,omitempty"`
	} `json:"extension,omitempty"`
}
type parameters struct {
	ResourceType string  `json:"resourceType"`
	Parameter    []param `json:"parameter"`
}

func makeR4ResourcePatientAliceJSON(t *testing.T) string {
	t.Helper()
	r := map[string]any{
		"resourceType": "Patient",
		"name": []any{
			map[string]any{"given": []string{"Alice", "B."}, "family": "Smith"},
			map[string]any{"given": []string{"Jim"}},
		},
	}
	cr := map[string]any{"resourceType": "Parameters", "parameter": []any{}}
	_ = cr
	b, _ := json.Marshal(map[string]any{"resourceType": "Patient", "name": r["name"]})
	// wrap as ContainedResource shape is not strictly required for this test path
	return string(b)
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body any) parameters {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var p parameters
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p
}

func findParam(parts []param, name string) *param {
	for i := range parts {
		if parts[i].Name == name {
			return &parts[i]
		}
	}
	return nil
}

func findParams(parts []param, name string) []param {
	var out []param
	for _, p := range parts {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}

func TestIntegration(t *testing.T) {
	mux := NewMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	patientJSON := makeR4ResourcePatientAliceJSON(t)

	tests := []struct {
		name         string
		path         string
		body         parameters
		wantEval     string
		wantContains []string
	}{
		{
			name: "R4 simple",
			path: "/$fhirpath",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("Patient.name.given.first()")},
				{Name: "resource", Resource: map[string]any{"resourceType": "Patient", "name": []any{map[string]any{"given": []string{"Alice", "B."}, "family": "Smith"}}}},
			}},
			wantEval:     "fhir-toolbox-go (R4)",
			wantContains: []string{"Alice"},
		},
		{
			name: "R4 context",
			path: "/$fhirpath",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("given.first()")},
				{Name: "context", ValueString: ptrStr("name")},
				{Name: "resource", Resource: map[string]any{"resourceType": "Patient", "name": []any{map[string]any{"given": []string{"Alice", "B."}}, map[string]any{"given": []string{"Jim"}}}}},
			}},
			wantEval:     "fhir-toolbox-go (R4)",
			wantContains: []string{"Alice", "Jim"},
		},
		{
			name: "R4 variables",
			path: "/$fhirpath",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("%v")},
				{Name: "variables", Part: []param{{Name: "v", ValueString: ptrStr("testMe")}}},
				{Name: "resource", Resource: map[string]any{"resourceType": "Patient"}},
			}},
			wantEval:     "fhir-toolbox-go (R4)",
			wantContains: []string{"testMe"},
		},
		{
			name: "R4 resource via extension",
			path: "/$fhirpath",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("Patient.name.given.first()")},
				{Name: "resource", Extension: []struct {
					Url         string  `json:"url"`
					ValueString *string `json:"valueString,omitempty"`
				}{{Url: "http:/.forms-lab.com/StructureDefinition/json-value", ValueString: ptrStr(patientJSON)}}},
			}},
			wantEval:     "fhir-toolbox-go (R4)",
			wantContains: []string{"Alice"},
		},
		{
			name: "R4B eval label",
			path: "/$fhirpath-r4b",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("1 = 1")},
				{Name: "resource", Resource: map[string]any{"resourceType": "Patient"}},
			}},
			wantEval: "fhir-toolbox-go (R4B)",
		},
		{
			name: "R5 eval label",
			path: "/$fhirpath-r5",
			body: parameters{ResourceType: "Parameters", Parameter: []param{
				{Name: "expression", ValueString: ptrStr("1 = 1")},
				{Name: "resource", Resource: map[string]any{"resourceType": "Patient"}},
			}},
			wantEval: "fhir-toolbox-go (R5)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := postJSON(t, ts, tc.path, tc.body)
			if got.ResourceType != "Parameters" {
				t.Fatalf("unexpected resourceType: %s", got.ResourceType)
			}
			// evaluator string
			paramsPart := findParam(got.Parameter, "parameters")
			if paramsPart == nil {
				t.Fatalf("parameters part missing")
			}
			eval := findParam(paramsPart.Part, "evaluator")
			if eval == nil || eval.ValueString == nil || *eval.ValueString != tc.wantEval {
				gotVal := "<nil>"
				if eval != nil && eval.ValueString != nil {
					gotVal = *eval.ValueString
				}
				t.Fatalf("evaluator mismatch: got=%q want=%q", gotVal, tc.wantEval)
			}
			// expected values across all result entries
			if len(tc.wantContains) > 0 {
				results := findParams(got.Parameter, "result")
				if len(results) == 0 {
					t.Fatalf("result missing")
				}
				found := 0
				for _, res := range results {
					for _, p := range res.Part {
						if p.Name == "string" && p.ValueString != nil {
							for _, want := range tc.wantContains {
								if *p.ValueString == want {
									found++
								}
							}
						}
					}
				}
				if found == 0 {
					t.Fatalf("expected to find values %v in any result", tc.wantContains)
				}
			}
		})
	}
}

func TestTraceParts(t *testing.T) {
	mux := NewMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := parameters{ResourceType: "Parameters", Parameter: []param{
		{Name: "expression", ValueString: ptrStr("trace('trc').given.first()")},
		{Name: "context", ValueString: ptrStr("name")},
		{Name: "resource", Resource: map[string]any{"resourceType": "Patient", "name": []any{map[string]any{"given": []string{"Alice", "B."}, "family": "Smith"}, map[string]any{"given": []string{"Jim"}}}}},
	}}

	got := postJSON(t, ts, "/$fhirpath", body)
	if got.ResourceType != "Parameters" {
		t.Fatalf("unexpected resourceType: %s", got.ResourceType)
	}
	// Verify at least one trace part exists with label 'trc'
	results := findParams(got.Parameter, "result")
	if len(results) == 0 {
		t.Fatalf("result missing")
	}
	foundTrace := false
	for _, res := range results {
		for _, p := range res.Part {
			if p.Name == "trace" && p.ValueString != nil && *p.ValueString == "trc" {
				foundTrace = true
			}
		}
	}
	if !foundTrace {
		t.Fatalf("expected at least one trace part with valueString 'trc'")
	}
	// No debug-trace expected for now
}

func ptrStr(s string) *string { return &s }
