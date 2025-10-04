package server

import (
	"context"
	"encoding/json"
	"fmt"
	fhirpath "github.com/DAMEDIC/fhir-toolbox-go/fhirpath"
	"github.com/DAMEDIC/fhir-toolbox-go/model"
	"github.com/DAMEDIC/fhir-toolbox-go/model/gen/basic"
	"github.com/DAMEDIC/fhir-toolbox-go/model/gen/r4"
	"github.com/DAMEDIC/fhir-toolbox-go/utils/ptr"
	"strings"
	"time"
)

type Backend struct {
	// BaseURL is used in the CapabilityStatement implementation.url and for
	// building canonical OperationDefinition URLs. If empty, defaults to
	// "http://localhost".
	BaseURL string
}

// fpTracer captures trace() calls during FHIRPath evaluation
type fpTracer struct {
	entries []traceEntry
}
type traceEntry struct {
	name   string
	values fhirpath.Collection
}

func (t *fpTracer) Log(name string, collection fhirpath.Collection) error {
	t.entries = append(t.entries, traceEntry{name: name, values: append(fhirpath.Collection(nil), collection...)})
	return nil
}

// CapabilityBase implements capabilities.ConcreteCapabilities for discovery and operation wiring.
func (b *Backend) CapabilityBase(ctx context.Context) (basic.CapabilityStatement, error) {
	now := time.Now().Format(time.RFC3339)
	baseURL := strings.TrimRight(b.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	// Minimal CapabilityStatement with system-level operations; wrapper augments it.
	return basic.CapabilityStatement{
		Status:      basic.Code{Value: ptr.To("active")},
		Kind:        basic.Code{Value: ptr.To("instance")},
		Date:        basic.DateTime{Value: ptr.To(now)},
		FhirVersion: basic.Code{Value: ptr.To("4.0")},
		Format:      []basic.Code{{Value: ptr.To("json")}},
		Software: &basic.CapabilityStatementSoftware{
			Name:    basic.String{Value: ptr.To("fhirpath-lab-go-server")},
			Version: &basic.String{Value: ptr.To("0.1.0")},
		},
		Implementation: &basic.CapabilityStatementImplementation{
			Description: basic.String{Value: ptr.To("FHIRPath Lab operations server (Go)")},
			Url:         &basic.Url{Value: &baseURL},
		},
	}, nil
}

func (b *Backend) FHIRPathOperationDefinition() r4.OperationDefinition {
	return operationDefinition("fhirpath")
}

func (b *Backend) FHIRPathR4OperationDefinition() r4.OperationDefinition {
	return operationDefinition("fhirpath-r4")
}

func (b *Backend) FHIRPathR4BOperationDefinition() r4.OperationDefinition {
	return operationDefinition("fhirpath-r4b")
}

func (b *Backend) FHIRPathR5OperationDefinition() r4.OperationDefinition {
	return operationDefinition("fhirpath-r5")
}

func operationDefinition(codeAndID string) r4.OperationDefinition {
	// Build the OperationDefinition including input/output parameters per
	// the fhirpath-lab server engine API specification.
	in := func(name string, min int32, max string, typ *string, doc string) r4.OperationDefinitionParameter {
		p := r4.OperationDefinitionParameter{
			Name: r4.Code{Value: ptr.To(name)},
			Use:  r4.Code{Value: ptr.To("in")},
			Min:  r4.Integer{Value: ptr.To(min)},
			Max:  r4.String{Value: ptr.To(max)},
		}
		if doc != "" {
			p.Documentation = &r4.String{Value: ptr.To(doc)}
		}
		if typ != nil {
			p.Type = &r4.Code{Value: typ}
		}
		return p
	}
	out := func(name string, min int32, max string, typ *string, doc string) r4.OperationDefinitionParameter {
		p := r4.OperationDefinitionParameter{
			Name: r4.Code{Value: ptr.To(name)},
			Use:  r4.Code{Value: ptr.To("out")},
			Min:  r4.Integer{Value: ptr.To(min)},
			Max:  r4.String{Value: ptr.To(max)},
		}
		if doc != "" {
			p.Documentation = &r4.String{Value: ptr.To(doc)}
		}
		if typ != nil {
			p.Type = &r4.Code{Value: typ}
		}
		return p
	}

	// Helper type codes
	tString := ptr.To("string")
	tResource := ptr.To("Resource")

	// variables multipart (input) – with parts: name (string), value[x] (any)
	variablesIn := in("variables", 0, "*", nil, "Variables to bind; provide one or more named variables.")
	variablesIn.Part = []r4.OperationDefinitionParameter{
		in("name", 1, "1", tString, "Variable name to bind."),
		// value[x] is polymorphic; document instead of constraining type.
		in("value[x]", 0, "1", nil, "Variable value using appropriate value[x] in Parameters (any FHIR type or Resource)."),
	}

	// variables multipart (output echo) – mirroring input for debugging visibility
	variablesOut := out("variables", 0, "*", nil, "Echo of variables passed to the evaluation.")
	variablesOut.Part = []r4.OperationDefinitionParameter{
		out("name", 1, "1", tString, "Variable name."),
		out("value[x]", 0, "1", nil, "Variable value using appropriate value[x] (any FHIR type or Resource)."),
	}

	// parameters (output) – describes request echo and additional metadata
	parametersOut := out("parameters", 1, "1", nil, "Input parameters and evaluation metadata.")
	parametersOut.Part = []r4.OperationDefinitionParameter{
		out("evaluator", 1, "1", tString, "Engine and version label, e.g. 'Java 6.6.5 (R4B)'."),
		out("parseDebugTree", 0, "1", tString, "Parser debug AST (JSON as string)."),
		out("expression", 1, "1", tString, "The expression that was executed."),
		out("context", 0, "1", tString, "The context expression used, if any."),
		out("resource", 1, "1", tResource, "The resource used as evaluation input."),
		variablesOut,
		out("expectedReturnType", 0, "1", tString, "Optional static analysis expected return type."),
		out("parseDebug", 0, "1", tString, "Optional unformatted parser debug messages."),
	}

	// result (output) – one per context item; includes results and traces
	resultOut := out(
		"result",
		0,
		"*",
		nil,
		"Results for each context item. The parameter valueString identifies the context (e.g., 'Patient.name[0]'). Parts include the evaluated results (named by datatype) and optional 'trace' parts containing traced values.",
	)
	// Model only the 'trace' container here and document the rest (dynamic names per datatype)
	resultOut.Part = []r4.OperationDefinitionParameter{
		out("trace", 0, "*", tString, "Trace output; valueString carries the trace label. Child parts contain traced values named by datatype with appropriate value[x], plus optional resource-path/json-value extensions."),
	}

	// debug-trace (output) – optional per-context execution trace
	debugTraceOut := out(
		"debug-trace",
		0,
		"*",
		nil,
		"Optional step-by-step execution trace per context. The parameter valueString identifies the context. Each part is named '{position},{length},{function}' and contains sub-parts such as resource-path, focus-resource-path, this-resource-path, index, and evaluated values.",
	)

	return r4.OperationDefinition{
		Id:       &r4.Id{Value: ptr.To(codeAndID)},
		Name:     r4.String{Value: ptr.To("FHIRPath Evaluate")},
		Status:   r4.PublicationStatusActive,
		Kind:     r4.OperationKindOperation,
		Code:     r4.Code{Value: ptr.To(codeAndID)},
		System:   r4.Boolean{Value: ptr.To(true)},
		Type:     r4.Boolean{Value: ptr.To(false)},
		Instance: r4.Boolean{Value: ptr.To(false)},
		Parameter: []r4.OperationDefinitionParameter{
			// Inputs
			in("expression", 1, "1", tString, "FHIRPath expression to execute."),
			in("context", 0, "1", tString, "Context expression to select focus items within the resource."),
			variablesIn,
			in("resource", 1, "1", tResource, "Resource to evaluate against. Alternatively provide as extension with http://fhir.forms-lab.com/StructureDefinition/json-value or http://fhir.forms-lab.com/StructureDefinition/xml-value."),
			in("terminologyserver", 0, "1", tString, "Terminology server base URL for lookups, when not natively supported."),

			// Outputs
			parametersOut,
			resultOut,
			debugTraceOut,
		},
	}
}

// Concrete invoke methods that the wrapper reflects to
func (b *Backend) InvokeFHIRPath(ctx context.Context, parameters basic.Parameters) (basic.Parameters, error) {
	return b.evalFHIRPathCommon(ctx, "fhirpath", parameters)
}

func (b *Backend) InvokeFHIRPathR4(ctx context.Context, parameters basic.Parameters) (basic.Parameters, error) {
	return b.evalFHIRPathCommon(ctx, "fhirpath-r4", parameters)
}

func (b *Backend) InvokeFHIRPathR4B(ctx context.Context, parameters basic.Parameters) (basic.Parameters, error) {
	return b.evalFHIRPathCommon(ctx, "fhirpath-r4b", parameters)
}

func (b *Backend) InvokeFHIRPathR5(ctx context.Context, parameters basic.Parameters) (basic.Parameters, error) {
	return b.evalFHIRPathCommon(ctx, "fhirpath-r5", parameters)
}

func (b *Backend) evalFHIRPathCommon(ctx context.Context, code string, parameters basic.Parameters) (basic.Parameters, error) {
	// Extract inputs
	exprParam, ok := findParamBasic(parameters.Parameter, "expression")
	if !ok || exprParam.Value == nil {
		return basic.Parameters{}, opErr("fatal", "processing", "missing 'expression' parameter")
	}
	exprStr, okS, err := exprParam.Value.ToString(false)
	if err != nil || !okS {
		return basic.Parameters{}, opErr("fatal", "processing", "invalid 'expression' parameter")
	}
	var contextExprStr string
	if ctxParam, ok := findParamBasic(parameters.Parameter, "context"); ok && ctxParam.Value != nil {
		if s, okS, _ := ctxParam.Value.ToString(false); okS {
			contextExprStr = string(s)
		}
	}
	resParam, ok := findParamBasic(parameters.Parameter, "resource")
	if !ok {
		return basic.Parameters{}, opErr("fatal", "processing", "missing 'resource' parameter")
	}
	resourceElem, err := decodeResource(resParam)
	if err != nil {
		return basic.Parameters{}, err
	}

	// Variables
	varsParam, _ := findParamBasic(parameters.Parameter, "variables")
	// Prepare evaluation context prototype
	baseCtx := r4.Context()
	baseCtx = fhirpath.WithEnv(baseCtx, "resource", resourceElem.(fhirpath.Element))
	baseCtx = fhirpath.WithEnv(baseCtx, "rootResource", resourceElem.(fhirpath.Element))

	// Support variables shape: either parts with arbitrary names or name/value[x] pairs
	for _, vp := range varsParam.Part {
		// case 1: arbitrary name
		if vp.Name.Value != nil && vp.Value != nil {
			if s, okS, _ := vp.Value.ToString(false); okS {
				baseCtx = fhirpath.WithEnv(baseCtx, *vp.Name.Value, fhirpath.String(s))
			}
			continue
		}
		// case 2: name + value[x]
		var varName string
		var haveName bool
		var varVal *string
		for _, pp := range vp.Part {
			if pp.Name.Value != nil && *pp.Name.Value == "name" {
				if s, okS, _ := pp.Value.ToString(false); okS {
					varName = string(s)
					haveName = true
				}
			} else if pp.Value != nil {
				if s, okS, _ := pp.Value.ToString(false); okS {
					vs := string(s)
					varVal = &vs
				}
			}
		}
		if haveName && varVal != nil {
			baseCtx = fhirpath.WithEnv(baseCtx, varName, fhirpath.String(*varVal))
		}
	}

	// Parse expressions
	exprParsed, err := fhirpath.Parse(string(exprStr))
	if err != nil {
		return basic.Parameters{}, opErr("fatal", "processing", "expression parse error: "+err.Error())
	}

	out := basic.Parameters{}
	if strings.TrimSpace(contextExprStr) != "" {
		// Evaluate context expression on the resource
		ctxExpr, err := fhirpath.Parse(contextExprStr)
		if err != nil {
			return basic.Parameters{}, opErr("fatal", "processing", "context parse error: "+err.Error())
		}
		ctxItems, err := fhirpath.Evaluate(baseCtx, resourceElem.(fhirpath.Element), ctxExpr)
		if err != nil {
			return basic.Parameters{}, opErr("fatal", "processing", "context evaluation error: "+err.Error())
		}
		// Evaluate main expression for each context item and build separate result entries
		for i, item := range ctxItems {
			// Create tracer per context
			tracer := &fpTracer{}
			evCtx := fhirpath.WithTracer(baseCtx, tracer)
			val, err := fhirpath.Evaluate(evCtx, item, exprParsed)
			if err != nil {
				return basic.Parameters{}, opErr("fatal", "processing", "evaluation error: "+err.Error())
			}
			// Build a result parameter for this context
			resName := "result"
			// valueString: context[index]
			vs := contextExprStr + "[" + fmt.Sprintf("%d", i) + "]"
			resParam := basic.ParametersParameter{
				Name:  basic.String{Value: &resName},
				Value: basic.String{Value: &vs},
				Part:  buildResultStringParts(val),
			}
			// Append trace parts
			for _, te := range tracer.entries {
				tname := "trace"
				label := te.name
				tr := basic.ParametersParameter{Name: basic.String{Value: &tname}, Value: basic.String{Value: &label}} // valueString is label
				// add traced values as child parts
				tr.Part = append(tr.Part, makeTraceParts(te.values)...)
				resParam.Part = append(resParam.Part, tr)
			}
			out.Parameter = append(out.Parameter, resParam)
			// debug-trace intentionally omitted for now
		}
	} else {
		// Evaluate directly on the resource
		tracer := &fpTracer{}
		evCtx := fhirpath.WithTracer(baseCtx, tracer)
		val, err := fhirpath.Evaluate(evCtx, resourceElem.(fhirpath.Element), exprParsed)
		if err != nil {
			return basic.Parameters{}, opErr("fatal", "processing", "evaluation error: "+err.Error())
		}
		res := basic.ParametersParameter{Name: basic.String{Value: ptr.To("result")}, Part: buildResultStringParts(val)}
		// Append trace parts
		for _, te := range tracer.entries {
			tname := "trace"
			label := te.name
			tr := basic.ParametersParameter{Name: basic.String{Value: &tname}, Value: basic.String{Value: &label}}
			tr.Part = append(tr.Part, makeTraceParts(te.values)...)
			res.Part = append(res.Part, tr)
		}
		out.Parameter = append(out.Parameter, res)
		// debug-trace intentionally omitted for now
	}

	// parameters part
	var paramsPart basic.ParametersParameter
	paramsPart.Name = basic.String{Value: ptr.To("parameters")}
	// evaluator label differs by op code
	evalLabel := "fhir-toolbox-go (R4)"
	if code != "fhirpath" {
		evalLabel = fmt.Sprintf("fhir-toolbox-go (%s)", strings.ToUpper(strings.TrimPrefix(code, "fhirpath-")))
	}
	paramsPart.Part = append(paramsPart.Part, basic.ParametersParameter{Name: basic.String{Value: ptr.To("evaluator")}, Value: basic.String{Value: &evalLabel}})
	// echo expression
	exprStrS := string(exprStr)
	paramsPart.Part = append(paramsPart.Part, basic.ParametersParameter{Name: basic.String{Value: ptr.To("expression")}, Value: basic.String{Value: &exprStrS}})
	if contextExprStr != "" {
		paramsPart.Part = append(paramsPart.Part, basic.ParametersParameter{Name: basic.String{Value: ptr.To("context")}, Value: basic.String{Value: &contextExprStr}})
	}
	// echo resource (typed if possible)
	paramsPart.Part = append(paramsPart.Part, basic.ParametersParameter{Name: basic.String{Value: ptr.To("resource")}, Resource: resourceElem})
	// echo variables when present (simplified)
	if len(varsParam.Part) > 0 {
		vname := "variables"
		varParts := []basic.ParametersParameter{}
		for _, vp := range varsParam.Part {
			if vp.Name.Value != nil && vp.Value != nil {
				if s, okS, _ := vp.Value.ToString(false); okS {
					vs := string(s)
					varParts = append(varParts, basic.ParametersParameter{Name: basic.String{Value: vp.Name.Value}, Value: basic.String{Value: &vs}})
				}
			}
		}
		paramsPart.Part = append(paramsPart.Part, basic.ParametersParameter{Name: basic.String{Value: &vname}, Part: varParts})
	}
	out.Parameter = append(out.Parameter, paramsPart)
	return out, nil
}

// decodeResource attempts to obtain an r4.Resource from Parameters.resource or its json-value extension.
func decodeResource(p basic.ParametersParameter) (model.Resource, error) {
	// Direct resource payload
	if p.Resource != nil {
		switch rr := p.Resource.(type) {
		case basic.RawResource:
			// Try to decode JSON into r4.ContainedResource
			var cr r4.ContainedResource
			if rr.IsJSON {
				if err := json.Unmarshal([]byte(rr.Content), &cr); err == nil && cr.Resource != nil {
					return cr.Resource, nil
				}
			}
			// As a fallback, return raw resource (won't evaluate children)
			return rr, nil
		default:
			return rr, nil
		}
	}
	// Check extensions for json-value
	for _, ext := range p.Extension {
		if strings.Contains(ext.Url, "json-value") {
			if s, ok, _ := ext.Value.ToString(false); ok {
				var cr r4.ContainedResource
				if err := json.Unmarshal([]byte(s), &cr); err == nil && cr.Resource != nil {
					return cr.Resource, nil
				}
			}
		}
	}
	return nil, opErr("fatal", "processing", "resource parameter missing or invalid")
}

// Helpers to extract parameters
func findParamBasic(params []basic.ParametersParameter, name string) (basic.ParametersParameter, bool) {
	for _, p := range params {
		if p.Name.Value != nil && *p.Name.Value == name {
			return p, true
		}
	}
	return basic.ParametersParameter{}, false
}

func buildResultStringParts(values fhirpath.Collection) []basic.ParametersParameter {
	var parts []basic.ParametersParameter
	for _, v := range values {
		if s, ok, err := v.ToString(false); err == nil && ok {
			name := "string"
			sv := string(s)
			parts = append(parts, basic.ParametersParameter{
				Name:  basic.String{Value: &name},
				Value: basic.String{Value: &sv},
			})
		}
	}
	return parts
}

func typeNameOf(e fhirpath.Element) string {
	if ti := e.TypeInfo(); ti != nil {
		if q, ok := ti.QualifiedName(); ok && q.Name != "" {
			return q.Name
		}
	}
	return "Element"
}

func makeTraceParts(values fhirpath.Collection) []basic.ParametersParameter {
	var parts []basic.ParametersParameter
	for _, v := range values {
		// Try primitive string conversion first
		if s, ok, err := v.ToString(false); err == nil && ok {
			n := "string"
			sv := string(s)
			parts = append(parts, basic.ParametersParameter{
				Name:  basic.String{Value: &n},
				Value: basic.String{Value: &sv},
			})
			continue
		}
		// Fallback: complex value via json-value extension
		n := typeNameOf(v)
		jsonStr := v.String()
		extURL := "http://fhir.forms-lab.com/StructureDefinition/json-value"
		parts = append(parts, basic.ParametersParameter{
			Name: basic.String{Value: &n},
			Extension: []basic.Extension{{
				Url:   extURL,
				Value: basic.String{Value: &jsonStr},
			}},
		})
	}
	return parts
}

// opErr builds an OperationOutcome as error for unified error handling
func opErr(severity, code, diagnostics string) basic.OperationOutcome {
	return basic.OperationOutcome{Issue: []basic.OperationOutcomeIssue{{
		Severity: basic.Code{Value: &severity},
		Code:     basic.Code{Value: &code},
		Details:  nil,
		Diagnostics: func() *basic.String {
			if diagnostics == "" {
				return nil
			}
			return &basic.String{Value: &diagnostics}
		}(),
	}}}
}
