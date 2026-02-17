---
title: "Contributing"
linkTitle: "Contributing"
weight: 99
description: >
  How to set up the development environment, run tests and benchmarks, add new functions, and submit changes to the FHIRPath Go library.
---

Thank you for your interest in contributing to FHIRPath Go! This guide covers everything you need to get started, from setting up your local environment to submitting a pull request.

## Development Setup

### Prerequisites

- **Go 1.23 or later** -- the minimum version specified in `go.mod`
- **Git**
- (Optional) **golangci-lint** for running the linter locally

### Clone the Repository

```bash
git clone https://github.com/gofhir/fhirpath.git
cd fhirpath
```

### Install Dependencies

```bash
go mod download
```

### Verify Everything Works

```bash
go test -v -race ./...
```

If all tests pass, you are ready to start developing.

## Project Structure

The repository is organized into the following packages:

```text
fhirpath/
  fhirpath.go          # Top-level API: Evaluate, MustEvaluate
  compiler.go          # Expression compilation (Compile, MustCompile)
  expression.go        # Expression type and Evaluate/EvaluateWithContext
  resource.go          # Resource interface, typed helpers (EvaluateToBoolean, etc.)
  cache.go             # ExpressionCache with LRU eviction
  options.go           # EvalOptions, functional options, ReferenceResolver
  eval/
    evaluator.go       # Core evaluation engine (tree walker)
    operators.go       # Operator implementations (+, -, =, >, and, or, etc.)
    errors.go          # Evaluation error types
  funcs/
    registry.go        # Function registry (Register, GetRegistry)
    existence.go       # exists(), empty(), count(), distinct(), all(), etc.
    filtering.go       # where(), select(), repeat(), ofType()
    subsetting.go      # first(), last(), tail(), skip(), take()
    strings.go         # startsWith(), endsWith(), contains(), replace(), etc.
    math.go            # abs(), ceiling(), floor(), ln(), log(), power(), etc.
    typechecking.go    # is(), as(), ofType() type-checking functions
    conversion.go      # toBoolean(), toInteger(), toDecimal(), toString(), etc.
    temporal.go        # now(), today(), dateTime arithmetic
    aggregate.go       # aggregate() function
    utility.go         # trace(), iif(), and other utility functions
    regex.go           # matches(), replaceMatches()
    fhir.go            # FHIR-specific: extension(), resolve(), memberOf(), etc.
  types/
    value.go           # Value interface
    collection.go      # Collection type and methods
    boolean.go         # Boolean type
    integer.go         # Integer type
    decimal.go         # Decimal type (uses shopspring/decimal)
    string.go          # String type
    date.go            # Date type with partial precision
    datetime.go        # DateTime type with partial precision
    time.go            # Time type
    quantity.go        # Quantity type with UCUM support
    object.go          # ObjectValue for JSON objects
    pool.go            # Object pooling for memory efficiency
    errors.go          # Type-system error types
  parser/
    grammar/           # ANTLR-generated lexer, parser, and visitor
  internal/
    ucum/
      ucum.go          # UCUM unit normalization and conversion
```

### Key Design Decisions

- **No FHIR model dependency.** The library works directly with raw JSON bytes via `github.com/buger/jsonparser`. This keeps the dependency tree small and lets users bring any FHIR model library (or none at all).
- **ANTLR-generated parser.** The FHIRPath grammar is parsed with `antlr4-go`. The grammar files live in `parser/grammar/`. Do not edit the generated Go files directly; regenerate them from the `.g4` grammar file if needed.
- **Arbitrary-precision decimals.** Decimal values use `github.com/shopspring/decimal` to avoid floating-point surprises.

## Running Tests

### Full Test Suite

```bash
go test -v -race ./...
```

The `-race` flag enables the Go race detector, which is important because the library is designed for concurrent use.

### A Single Package

```bash
go test -v -race ./funcs/
go test -v -race ./eval/
go test -v -race ./types/
```

### A Single Test

```bash
go test -v -race -run TestEvaluateToBoolean ./...
```

## Running Benchmarks

Performance benchmarks live alongside the tests in `*_bench_test.go` files.

```bash
go test -bench=. -benchmem ./...
```

To benchmark only the top-level package:

```bash
go test -bench=. -benchmem -benchtime=5s .
```

Compare before and after your change:

```bash
# Before
go test -bench=. -benchmem -count=6 . > old.txt

# Make your changes, then:
go test -bench=. -benchmem -count=6 . > new.txt

# Compare (requires golang.org/x/perf/cmd/benchstat)
benchstat old.txt new.txt
```

## Linting

The project uses [golangci-lint](https://golangci-lint.run/) for static analysis:

```bash
golangci-lint run
```

Install it with:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

The linter configuration lives in `.golangci.yml` at the repository root. Please make sure `golangci-lint run` passes cleanly before submitting a pull request.

## Adding a New Function

The function registry in `funcs/` makes it straightforward to add new FHIRPath functions. Follow these steps:

### 1. Choose the Right File

Place your function in the file that matches its category:

| Category | File |
|----------|------|
| Existence / counting | `funcs/existence.go` |
| Filtering / projection | `funcs/filtering.go` |
| Subsetting (first, last, etc.) | `funcs/subsetting.go` |
| String manipulation | `funcs/strings.go` |
| Math | `funcs/math.go` |
| Type checking / conversion | `funcs/typechecking.go` or `funcs/conversion.go` |
| Date / time | `funcs/temporal.go` |
| Aggregation | `funcs/aggregate.go` |
| FHIR-specific | `funcs/fhir.go` |
| Regex | `funcs/regex.go` |
| Utility | `funcs/utility.go` |

### 2. Implement the Function

Every function has the signature:

```go
func fnMyFunction(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error)
```

- `ctx` -- the evaluation context (access to the resource, variables, limits)
- `input` -- the collection the function is invoked on (left side of the dot)
- `args` -- the arguments passed to the function

Example skeleton:

```go
func fnMyFunction(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
    if input.Empty() {
        return types.Collection{}, nil
    }

    // Implement your logic here

    return result, nil
}
```

### 3. Register the Function

In the same file's `init()` block, register your function with the registry:

```go
func init() {
    // ... existing registrations ...

    Register(FuncDef{
        Name:    "myFunction",      // the name used in FHIRPath expressions
        MinArgs: 0,                 // minimum number of arguments
        MaxArgs: 1,                 // maximum number of arguments
        Fn:      fnMyFunction,
    })
}
```

### 4. Write Tests

Add tests in the corresponding `*_test.go` file. Test at minimum:

- Normal case with expected input
- Empty input (should return empty collection)
- Edge cases (nil values, wrong types, boundary values)
- Error cases (wrong number of arguments, type mismatches)

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        resource string
        expr     string
        want     string
    }{
        {
            name:     "basic case",
            resource: `{"resourceType": "Patient", "id": "1"}`,
            expr:     "Patient.id.myFunction()",
            want:     "expected-value",
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            // Assert result matches tt.want
        })
    }
}
```

### 5. Document the Function

If the function is part of the FHIRPath specification, reference the spec section. If it is a custom function specific to this library, document it clearly in the function's doc comment.

## Code Style

- Follow the conventions already present in the codebase.
- Run `gofmt` (or `goimports`) before committing.
- Ensure `golangci-lint run` passes without warnings.
- Keep functions focused. If a function grows beyond ~50 lines, consider extracting helpers.
- Write clear Go doc comments on all exported symbols.
- Prefer returning `(types.Collection, error)` to panicking.
- Handle empty collections explicitly -- returning an empty collection is usually the correct behavior per the FHIRPath specification.

## Submitting Changes

### 1. Fork the Repository

Fork `github.com/gofhir/fhirpath` to your own GitHub account.

### 2. Create a Feature Branch

```bash
git checkout -b feature/my-new-function
```

Use a descriptive branch name:

- `feature/add-encode-function` for new features
- `fix/where-empty-collection` for bug fixes
- `docs/update-readme` for documentation changes

### 3. Make Your Changes

Commit in small, logical units. Each commit should compile and pass tests.

### 4. Run the Full Suite

```bash
go test -v -race ./...
golangci-lint run
```

### 5. Push and Open a Pull Request

```bash
git push origin feature/my-new-function
```

Open a pull request against `main`. In the PR description:

- Describe **what** you changed and **why**.
- Reference any related issues (e.g., `Fixes #42`).
- If you added a new function, include example usage.
- If you changed performance-sensitive code, include benchmark results.

### 6. Respond to Review

A maintainer will review your PR. Please be responsive to feedback and push follow-up commits to the same branch.

## License

FHIRPath Go is released under the [MIT License](https://github.com/gofhir/fhirpath/blob/main/LICENSE). By contributing, you agree that your contributions will be licensed under the same terms.
