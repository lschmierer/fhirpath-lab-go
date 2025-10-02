module fhirpath-lab-go-server

go 1.23.0

toolchain go1.24.5

require github.com/DAMEDIC/fhir-toolbox-go v0.0.0-00010101000000-000000000000

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
)

replace github.com/DAMEDIC/fhir-toolbox-go => ../fhir-toolbox-go
