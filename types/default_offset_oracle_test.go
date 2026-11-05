package types

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The reference oracle for a default offset coarser than the hour: the CQL
// engines Java 5.4.0 and cql-execution 3.3.2 evaluated probe256.cql at
// 2026-01-01T00:00:00 at four offsets (testdata/cql-default-offset, README.md
// for the table), and both answer every comparison alike. The expected values
// come from their raw output, not from this engine's rule.
//
// The durations (D11, D12, D20, D21) and the same-day test (D13) are not
// comparisons, and the two engines split on the durations, so they are not here.

type oracleOperand struct {
	written string
	// placed marks a CQL literal without an offset, which takes the evaluation
	// request's offset; one written with Z or an offset keeps its own.
	placed bool
}

type oracleCase struct {
	define      string
	left, right oracleOperand
	op          string // "<", ">" or "="
}

func at(written string) oracleOperand  { return oracleOperand{written, true} }
func own(written string) oracleOperand { return oracleOperand{written, false} }

var oracleCases = []oracleCase{
	{"D0", at("2012-01-01"), own("2012-01-01T05:00:00.000Z"), "<"},
	{"D1", at("2012-01-01"), own("2012-01-01T01:00:00.000Z"), "="},
	{"D2", at("2012-01-01"), own("2012-01-02T02:00:00.000Z"), "="},
	{"D3", at("2012-01-01"), own("2011-12-31T23:00:00.000Z"), ">"},
	{"D4", at("2012-01"), own("2012-02-01T02:00:00.000Z"), "="},
	{"D5", at("2012"), own("2013-01-01T02:00:00.000Z"), "="},
	{"D6", at("2012-01-01"), at("2012-01-01"), "="},
	{"D7", at("2012-01-01"), at("2012-01-01T10:00:00.000"), "="},
	// D8 is the FHIRPath-reachable row at the hour with written offsets, which
	// this engine answers as FP-19 (start instants) and the references as null.
	// It is checked apart, in TestOracleHourPrecisionAcrossOffsetsIsFP19.
	{"D9", own("2020-01-01T10:00+05:30"), own("2020-01-01T04:45Z"), "<"},
	{"D10", own("2020-01-01T10:00+05:30"), own("2020-01-01T05:15Z"), "="},
	// ToDateTime(@2012-01-01) is the day as a DateTime, placed.
	{"D14", at("2012-01-01"), own("2012-01-01T05:00:00.000Z"), "<"},
	{"D15", at("2016-12-31"), own("2017-01-01T02:59:59.999Z"), "="},
	{"D16", at("2012"), own("2013-01-01T00:00:00.000Z"), "<"},
	{"D17", at("2012-12-31"), own("2013-01-01T10:00:00.000Z"), "<"},
	// DateTime(2012, 1, 1, 0, 0, 0, 0, 14.0)
	{"D18", at("2012-01-01"), own("2012-01-01T00:00:00.000+14:00"), "="},
	// DateTime(2012, 1, 1, 23, 0, 0, 0, -14.0)
	{"D19", at("2012-01-01"), own("2012-01-01T23:00:00.000-14:00"), "="},
}

var oracleRuns = []struct {
	suffix string
	offset time.Duration
}{
	{"m0300", -3 * time.Hour},
	{"p1400", 14 * time.Hour},
	{"m1400", -14 * time.Hour},
	{"p0530", 5*time.Hour + 30*time.Minute},
}

// oracleAnswer reads one define from an engine's output: "true", "false" or
// "unknown" for a null.
func oracleAnswer(t *testing.T, file, define string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "cql-default-offset", file))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	cell, ok := out.Results[define]
	if !ok {
		t.Fatalf("%s: no result for %s", file, define)
	}
	if string(cell) == "null" {
		return "unknown"
	}
	var boolean struct {
		Type  string `json:"type"`
		Value bool   `json:"value"`
	}
	if err := json.Unmarshal(cell, &boolean); err != nil || boolean.Type != "Boolean" {
		t.Fatalf("%s %s: not a Boolean: %s", file, define, cell)
	}
	if boolean.Value {
		return "true"
	}
	return "false"
}

func (o oracleOperand) build(t *testing.T, evaluation time.Duration) DateTime {
	t.Helper()

	value := mustDateTime(t, o.written)
	if o.placed {
		value = value.WithDefaultOffset(evaluation)
	}
	return value
}

// goAnswer evaluates the operator the way a CQL engine on this one would.
func goAnswer(t *testing.T, op string, left, right DateTime) string {
	t.Helper()

	if op == "=" {
		equal, err := EqualTemporal(left, right)
		if err != nil {
			if IsUnknownTemporalComparison(err) {
				return "unknown"
			}
			t.Fatal(err)
		}
		return boolWord(equal)
	}

	cmp, err := left.Compare(right)
	if err != nil {
		if IsUnknownTemporalComparison(err) {
			return "unknown"
		}
		t.Fatal(err)
	}
	if op == "<" {
		return boolWord(cmp < 0)
	}
	return boolWord(cmp > 0)
}

func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func mirrorOp(op string) string {
	switch op {
	case "<":
		return ">"
	case ">":
		return "<"
	}
	return op
}

func TestOracleCQLComparisonsAtFourEvaluationOffsets(t *testing.T) {
	for _, run := range oracleRuns {
		for _, tt := range oracleCases {
			t.Run(run.suffix+"/"+tt.define, func(t *testing.T) {
				java := oracleAnswer(t, "java"+run.suffix+".json", tt.define)
				js := oracleAnswer(t, "js"+run.suffix+".json", tt.define)
				if java != js {
					t.Fatalf("the references disagree: Java %s, cql-execution %s", java, js)
				}

				left, right := tt.left.build(t, run.offset), tt.right.build(t, run.offset)
				if got := goAnswer(t, tt.op, left, right); got != java {
					t.Errorf("%s %s %s: got %s, want %s", tt.left.written, tt.op, tt.right.written, got, java)
				}
				// The mirrored question has the mirrored answer.
				if got := goAnswer(t, mirrorOp(tt.op), right, left); got != java {
					t.Errorf("reversed, %s %s %s: got %s, want %s", tt.right.written, mirrorOp(tt.op), tt.left.written, got, java)
				}
			})
		}
	}
}

// D8, @2020-01-01T10+05:30 < @2020-01-01T04:45Z, involves no default and no
// value coarser than the hour: it is the row where the references answer null
// (the first value is the hour 10 at +05:30, 04:30Z to 05:30Z, and 04:45Z falls
// within it) and this engine, which compares values at different offsets as the
// instants they start at, answers true. That divergence is FP-19 and is out of
// this increment's scope, so this pins what the engine says today and that the
// references still say null; it must change together with FP-19.
func TestOracleHourPrecisionAcrossOffsetsIsFP19(t *testing.T) {
	for _, run := range oracleRuns {
		t.Run(run.suffix, func(t *testing.T) {
			for _, file := range []string{"java" + run.suffix + ".json", "js" + run.suffix + ".json"} {
				if got := oracleAnswer(t, file, "D8"); got != "unknown" {
					t.Fatalf("%s: the reference now answers %s", file, got)
				}
			}
			left, right := own("2020-01-01T10+05:30").build(t, run.offset), own("2020-01-01T04:45Z").build(t, run.offset)
			if got := goAnswer(t, "<", left, right); got != "true" {
				t.Errorf("got %s; FP-19 says this engine answers true until it is settled", got)
			}
		})
	}
}
