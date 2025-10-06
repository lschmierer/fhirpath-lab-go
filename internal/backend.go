package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	fhirpath "github.com/DAMEDIC/fhir-toolbox-go/fhirpath"
	"github.com/DAMEDIC/fhir-toolbox-go/model"
	"github.com/DAMEDIC/fhir-toolbox-go/model/gen/r4"
	"github.com/DAMEDIC/fhir-toolbox-go/model/gen/r4b"
	"github.com/DAMEDIC/fhir-toolbox-go/model/gen/r5"
	"github.com/DAMEDIC/fhir-toolbox-go/utils/ptr"
	"strings"
	"time"
)

//go:generate go run ./cmd/gen

type Backend struct {
	BaseURL string
}

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

func (b *Backend) CapabilityBase(ctx context.Context) (r4.CapabilityStatement, error) {
	now := time.Now().Format(time.RFC3339)
	baseURL := strings.TrimRight(b.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	return r4.CapabilityStatement{
		Status:      r4.Code{Value: ptr.To("active")},
		Kind:        r4.Code{Value: ptr.To("instance")},
		Date:        r4.DateTime{Value: ptr.To(now)},
		FhirVersion: r4.Code{Value: ptr.To("4.0")},
		Format:      []r4.Code{{Value: ptr.To("json")}},
		Software: &r4.CapabilityStatementSoftware{
			Name:    r4.String{Value: ptr.To("fhirpath-lab-go-cmd")},
			Version: &r4.String{Value: ptr.To("0.1.0")},
		},
		Implementation: &r4.CapabilityStatementImplementation{
			Description: r4.String{Value: ptr.To("FHIRPath Lab operations cmd (Go)")},
			Url:         &r4.Url{Value: &baseURL},
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
	// Build the OperationDefinition including input/output parameters per the fhirpath-lab cmd engine API specification.
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
			in("terminologyserver", 0, "1", tString, "Terminology cmd base URL for lookups, when not natively supported."),

			// Outputs
			parametersOut,
			resultOut,
			debugTraceOut,
		},
	}
}

// Concrete invoke methods
func (b *Backend) InvokeFHIRPath(ctx context.Context, parameters r4.Parameters) (r4.Parameters, error) {
	return b.InvokeFHIRPathR4(ctx, parameters)
}

func (b *Backend) InvokeFHIRPathR4(ctx context.Context, parameters r4.Parameters) (r4.Parameters, error) {
	inputs, err := parseParameters[model.R4](parameters)
	if err != nil {
		return r4.Parameters{}, opErrR4("fatal", "processing", err.Error())
	}

	result := evalFHIRPath[model.R4](ctx, inputs)
	if result.error != nil {
		return r4.Parameters{}, opErrR4("fatal", "processing", result.error.Error())
	}

	return buildR4Parameters[model.R4](result, inputs), nil
}

// InvokeFHIRPathR4B must take r4 parameters, because these are parsed by the framework.
// But we can return other types as long as they implement model.Resource.
func (b *Backend) InvokeFHIRPathR4B(ctx context.Context, parameters r4.Parameters) (r4b.Parameters, error) {
	inputs, err := parseParameters[model.R4B](parameters)
	if err != nil {
		return r4b.Parameters{}, opErrR4B("fatal", "processing", err.Error())
	}

	result := evalFHIRPath[model.R4B](ctx, inputs)
	if result.error != nil {
		return r4b.Parameters{}, opErrR4B("fatal", "processing", result.error.Error())
	}

	return buildR4BParameters[model.R4B](result, inputs), nil
}

func (b *Backend) InvokeFHIRPathR5(ctx context.Context, parameters r4.Parameters) (r5.Parameters, error) {
	inputs, err := parseParameters[model.R5](parameters)
	if err != nil {
		return r5.Parameters{}, opErrR5("fatal", "processing", err.Error())
	}

	result := evalFHIRPath[model.R5](ctx, inputs)
	if result.error != nil {
		return r5.Parameters{}, opErrR5("fatal", "processing", result.error.Error())
	}

	return buildR5Parameters[model.R5](result, inputs), nil
}

// Generic evaluation inputs and result
type evalInputs struct {
	expression string
	context    string
	resource   model.Resource
	variables  map[string]fhirpath.Element
}

type evalResult struct {
	results []resultEntry
	error   error
}

type resultEntry struct {
	contextPath string
	values      fhirpath.Collection
	traces      []traceEntry
}

// Generic evaluation function
func evalFHIRPath[R model.Release](ctx context.Context, inputs evalInputs) evalResult {
	// Prepare evaluation context
	var release R
	switch any(release).(type) {
	case model.R4:
		ctx = r4.WithContext(ctx)
	case model.R4B:
		ctx = r4b.WithContext(ctx)
	case model.R5:
		ctx = r5.WithContext(ctx)
	}

	ctx = fhirpath.WithEnv(ctx, "resource", inputs.resource.(fhirpath.Element))
	ctx = fhirpath.WithEnv(ctx, "rootResource", inputs.resource.(fhirpath.Element))

	for name, value := range inputs.variables {
		ctx = fhirpath.WithEnv(ctx, name, value)
	}

	// If the expression is empty, don't attempt to parse/evaluate; return no results.
	if strings.TrimSpace(inputs.expression) == "" {
		return evalResult{results: nil}
	}

	// Parse expressions
	exprParsed, err := fhirpath.Parse(inputs.expression)
	if err != nil {
		return evalResult{error: fmt.Errorf("expression parse error: %w", err)}
	}

	var results []resultEntry
	if strings.TrimSpace(inputs.context) != "" {
		// Evaluate context expression on the resource
		ctxExpr, err := fhirpath.Parse(inputs.context)
		if err != nil {
			return evalResult{error: fmt.Errorf("context parse error: %w", err)}
		}
		ctxItems, err := fhirpath.Evaluate(ctx, inputs.resource.(fhirpath.Element), ctxExpr)
		if err != nil {
			return evalResult{error: fmt.Errorf("context evaluation error: %w", err)}
		}

		// Evaluate main expression for each context item
		for i, item := range ctxItems {
			tracer := &fpTracer{}
			evCtx := fhirpath.WithTracer(ctx, tracer)
			val, err := fhirpath.Evaluate(evCtx, item, exprParsed)
			if err != nil {
				return evalResult{error: fmt.Errorf("evaluation error: %w", err)}
			}

			contextPath := fmt.Sprintf("%s.%s[%d]", inputs.resource.ResourceType(), inputs.context, i)
			results = append(results, resultEntry{
				contextPath: contextPath,
				values:      val,
				traces:      tracer.entries,
			})
		}
	} else {
		// Evaluate directly on the resource
		tracer := &fpTracer{}
		evCtx := fhirpath.WithTracer(ctx, tracer)
		val, err := fhirpath.Evaluate(evCtx, inputs.resource.(fhirpath.Element), exprParsed)
		if err != nil {
			return evalResult{error: fmt.Errorf("evaluation error: %w", err)}
		}

		results = append(results, resultEntry{
			values: val,
			traces: tracer.entries,
		})
	}

	return evalResult{results: results}
}

// parseParameters extracts evaluation inputs from Parameters resource.
// The release parameter specifies which FHIR release to decode resources as.
// Supports the json-value extension for cross-release resource encoding.
func parseParameters[R model.Release](parameters fhirpath.Element) (evalInputs, error) {
	// Get parameters list from the Parameters resource
	paramsList := parameters.Children("parameter")

	// Helper to find a parameter by name
	findParam := func(name string) (fhirpath.Element, bool) {
		for _, p := range paramsList {
			nameVal := p.Children("name")
			if len(nameVal) == 0 {
				continue
			}
			if nameStr, ok, _ := nameVal[0].ToString(false); ok && string(nameStr) == name {
				return p, true
			}
		}
		return nil, false
	}

	// Extract expression
	exprParam, ok := findParam("expression")
	if !ok {
		return evalInputs{}, fmt.Errorf("missing 'expression' parameter")
	}
	exprValue := exprParam.Children("value")
	if len(exprValue) == 0 {
		return evalInputs{}, fmt.Errorf("missing 'expression' value")
	}
	exprStr, okS, err := exprValue[0].ToString(false)
	if err != nil || !okS {
		return evalInputs{}, fmt.Errorf("invalid 'expression' parameter")
	}

	// Extract optional context
	var contextExprStr string
	if ctxParam, ok := findParam("context"); ok {
		ctxValue := ctxParam.Children("value")
		if len(ctxValue) > 0 {
			if s, okS, _ := ctxValue[0].ToString(false); okS {
				contextExprStr = string(s)
			}
		}
	}

	// Extract resource - check json-value extension first
	resParam, ok := findParam("resource")
	if !ok {
		return evalInputs{}, fmt.Errorf("missing 'resource' parameter")
	}

	var resourceElem model.Resource

	// Check for json-value extension
	extensions := resParam.Children("extension")
	for _, ext := range extensions {
		urlChildren := ext.Children("url")
		if len(urlChildren) > 0 {
			if urlStr, ok, _ := urlChildren[0].ToString(false); ok &&
				string(urlStr) == "http://fhir.forms-lab.com/StructureDefinition/json-value" {
				// Found json-value extension, decode the JSON
				valueChildren := ext.Children("value")
				if len(valueChildren) > 0 {
					if jsonStr, ok, _ := valueChildren[0].ToString(false); ok {
						decoded, err := decodeResourceFromJSON[R](string(jsonStr))
						if err != nil {
							return evalInputs{}, fmt.Errorf("failed to decode resource from json-value extension: %w", err)
						}
						resourceElem = decoded
						break
					}
				}
			}
		}
	}

	// If no json-value extension, try the resource field directly
	if resourceElem == nil {
		resourceList := resParam.Children("resource")
		if len(resourceList) == 0 {
			return evalInputs{}, fmt.Errorf("resource parameter missing or invalid")
		}
		var ok bool
		resourceElem, ok = resourceList[0].(model.Resource)
		if !ok {
			return evalInputs{}, fmt.Errorf("resource is not a valid FHIR resource")
		}
	}

	// Extract variables
	variables := make(map[string]fhirpath.Element)
	if varsParam, ok := findParam("variables"); ok {
		partsList := varsParam.Children("part")
		for _, vp := range partsList {
			nameVal := vp.Children("name")
			if len(nameVal) == 0 {
				continue
			}
			nameStr, ok, _ := nameVal[0].ToString(false)
			if !ok {
				continue
			}
			valueVal := vp.Children("value")
			if len(valueVal) == 0 {
				continue
			}
			variables[string(nameStr)] = valueVal[0]
		}
	}

	return evalInputs{
		expression: string(exprStr),
		context:    contextExprStr,
		resource:   resourceElem,
		variables:  variables,
	}, nil
}

// decodeResourceFromJSON decodes a JSON string into a release-specific resource
func decodeResourceFromJSON[R model.Release](jsonStr string) (model.Resource, error) {
	var release R
	var resource model.Resource
	var err error

	reader := bytes.NewReader([]byte(jsonStr))
	dec := json.NewDecoder(reader)

	switch any(release).(type) {
	case model.R4:
		var r4Res r4.ContainedResource
		err = dec.Decode(&r4Res)
		resource = r4Res.Resource
	case model.R4B:
		var r4bRes r4b.ContainedResource
		err = dec.Decode(&r4bRes)
		resource = r4bRes.Resource
	case model.R5:
		var r5Res r5.ContainedResource
		err = dec.Decode(&r5Res)
		resource = r5Res.Resource
	default:
		return nil, fmt.Errorf("unsupported release type")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource JSON: %w", err)
	}
	return resource, nil
}

func typeNameOf(e fhirpath.Element) string {
	if ti := e.TypeInfo(); ti != nil {
		if q, ok := ti.QualifiedName(); ok && q.Name != "" {
			return q.Name
		}
	}
	return "Element"
}
