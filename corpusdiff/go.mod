// A differential harness over the official FHIR examples: what two revisions of
// this engine answer for what a validator evaluates.
//
// Its own module for the same reason the conformance and difftest harnesses
// are: the engine's go.mod stays free of the FHIR model packages.
module github.com/gofhir/fhirpath/corpusdiff

go 1.24.1

replace github.com/gofhir/fhirpath => ../

require (
	github.com/gofhir/fhirpath v1.4.0
	github.com/gofhir/models/r4 v1.4.0
	github.com/gofhir/models/r4b v1.4.0
	github.com/gofhir/models/r5 v1.4.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/gofhir/ucum/v4 v4.2.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
)
