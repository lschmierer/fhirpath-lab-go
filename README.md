# FHIRPath Lab Go Server

Go implementation of FHIRPath Lab evaluation endpoints using the fhir-toolbox-go engine.

## Endpoints

- `POST /$fhirpath` (R4)
- `POST /$fhirpath-r4b` (R4B)
- `POST /$fhirpath-r5` (R5)

## API

Send a FHIR `Parameters` resource with:
- `expression` (required): FHIRPath expression
- `resource` (required): FHIR resource (embedded or via `json-value` extension)
- `context` (optional): focus selector
- `variables` (optional): variable bindings

Response includes `result` parts (typed values, optional trace) and echoed `parameters`.

See https://github.com/brianpos/fhirpath-lab/blob/develop/server-api.md for the full specification.

## Tests

```bash
go test ./internal/server -v
```

