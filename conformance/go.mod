// Conformance harness for the official HL7 FHIRPath suite.
//
// A separate module so that the engine's own go.mod stays free of the FHIR
// model packages: they are needed only to measure conformance, and a consumer
// of the engine should not carry them in its dependency graph.
module github.com/gofhir/fhirpath/conformance

go 1.23

// The harness always measures the engine in this working tree, never a
// published version.
replace github.com/gofhir/fhirpath => ../

require (
	github.com/gofhir/fhirpath v1.4.0
	github.com/gofhir/models/r4 v1.2.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
)
