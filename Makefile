ANTLR_VERSION := 4.13.1
ANTLR_JAR     := build/antlr-$(ANTLR_VERSION)-complete.jar
ANTLR_URL     := https://www.antlr.org/download/antlr-$(ANTLR_VERSION)-complete.jar
GRAMMAR       := grammar/fhirpath.g4

.PHONY: help generate generate-check test test-race bench bench-compare lint conformance clean

help:
	@echo "generate        Regenerate the parser from $(GRAMMAR)"
	@echo "generate-check  Verify the generated parser matches the grammar"
	@echo "test            Run the test suite"
	@echo "test-race       Run the test suite with the race detector"
	@echo "bench           Run the benchmarks"
	@echo "bench-compare   Compare the benchmarks against another revision"
	@echo "lint            Run golangci-lint"
	@echo "conformance     Report conformance against the official FHIRPath suite"
	@echo "conformance-update  Re-baseline the conformance known-failures lists"
	@echo "clean           Remove build artifacts"

$(ANTLR_JAR):
	@mkdir -p build
	curl -sSL $(ANTLR_URL) -o $@

# Regenerates parser/grammar from the grammar. ANTLR mirrors the grammar's own
# directory under -o, so grammar/fhirpath.g4 lands in parser/grammar.
# Requires a Java runtime; the ANTLR jar is downloaded on first use.
generate: $(ANTLR_JAR)
	java -jar $(ANTLR_JAR) -Dlanguage=Go -package grammar -visitor -no-listener -o parser $(GRAMMAR)
	@rm -f parser/grammar/*.interp parser/grammar/*.tokens
	@gofmt -w parser/grammar
	@echo "Regenerated parser/grammar from $(GRAMMAR)"

# Fails when the committed parser does not match the grammar, so a grammar edit
# cannot be merged without its regenerated parser.
generate-check: $(ANTLR_JAR)
	@rm -rf build/generate-check
	@mkdir -p build/generate-check
	@java -jar $(ANTLR_JAR) -Dlanguage=Go -package grammar -visitor -no-listener \
		-o build/generate-check $(GRAMMAR)
	@rm -f build/generate-check/grammar/*.interp build/generate-check/grammar/*.tokens
	@gofmt -w build/generate-check/grammar
	@diff -q build/generate-check/grammar parser/grammar >/dev/null 2>&1 \
		|| { rm -rf build/generate-check; echo "parser/grammar is out of date with $(GRAMMAR); run 'make generate'"; exit 1; }
	@rm -rf build/generate-check
	@echo "Generated parser is up to date with $(GRAMMAR)"

test:
	go test ./...
	cd conformance && go test ./...

test-race:
	go test -race ./...

# Benchmarks are run here rather than in CI: a shared runner's timings vary by
# more than most changes worth making, so a number from one would be noise
# wearing a green check.
#
# BENCH selects benchmarks, COUNT samples each -- benchstat needs six for a
# confidence interval -- and BENCHTIME sets how long each sample runs.
BENCH     ?= .
COUNT     ?= 6
BENCHTIME ?= 0.5s

bench:
	go test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) -count=$(COUNT) .

# Measures this working tree against BASE, both now and on this machine, which
# is the only comparison that means anything. A benchmark present on one side
# only is reported without a comparison.
#
#   make bench-compare                      # against main
#   make bench-compare BASE=v1.7.0          # against a release
#   make bench-compare BENCH=ScaleDeepNav   # one benchmark
BASE ?= main

bench-compare:
	@COUNT=$(COUNT) BENCHTIME=$(BENCHTIME) scripts/bench-compare.sh $(BASE) '$(BENCH)' 

# Run through go run so that this matches CI exactly without anyone having to
# install a particular build. Keep in sync with GOLANGCI_VERSION in
# .github/workflows/ci.yml.
GOLANGCI_VERSION := v2.12.2
GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

lint:
	$(GOLANGCI) run ./...
	cd conformance && $(GOLANGCI) run ./...

# The harness lives in its own module so that the engine's go.mod stays free of
# the FHIR model packages.
conformance:
	@cd conformance && go test -run TestOfficialSuite -v . 2>&1 | grep -E "official suite|skipped [0-9]"

conformance-update:
	@cd conformance && go test -run TestOfficialSuite -update-known-failures .

clean:
	rm -rf build
