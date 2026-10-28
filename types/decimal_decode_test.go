package types

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"
)

// What decodeDecimal reads is what shopspring/decimal v1.4.0's NewFromString
// read, measured on that version (#235): the same coefficient and exponent,
// whatever the decoder bound later versions put on the exponent. The grammar
// is that library's, not decided here (#238).
func TestDecodeDecimalReadsAsVersion140Did(t *testing.T) {
	for _, tt := range []struct {
		written, coefficient string
		exponent             int32
	}{
		{"0", "0", 0},
		{"-0", "0", 0},
		{"+1", "1", 0},
		{"1", "1", 0},
		{"-1", "-1", 0},
		{"007.50", "750", -2},
		{"5.", "5", 0},
		{".5", "5", -1},
		{"-.5", "-5", -1},
		{"1.e5", "1", 5},
		{"1e5", "1", 5},
		{"1E5", "1", 5},
		{"1e+5", "1", 5},
		{"1e-5", "1", -5},
		{"-1.50E-3", "-150", -5},
		{"-0.50E+3", "-50", 1},
		{"1e+0005", "1", 5},
		{"-999999999999999999", "-999999999999999999", 0},
		{"9999999999999999999", "9999999999999999999", 0},
		{"123456789012345678901234567890.5", "1234567890123456789012345678905", -1},
		{"1.5e2147483647", "15", 2147483646},
		{"0.5e-2147483647", "5", -2147483648},
		{"1e0000000000000000000005", "1", 5},
		// Past MaxDecodeExponent, which v1.5.0 and later refuse
		{"1e10000", "1", 10000},
		{"1e10001", "1", 10001},
		{"1e-10001", "1", -10001},
		{"1e20000", "1", 20000},
		{"1e999999999", "1", 999999999},
	} {
		d, err := decodeDecimal(tt.written)
		if err != nil {
			t.Errorf("%q: %v", tt.written, err)
			continue
		}
		if d.Coefficient().String() != tt.coefficient || d.Exponent() != tt.exponent {
			t.Errorf("%q read as %se%d, want %se%d", tt.written, d.Coefficient(), d.Exponent(), tt.coefficient, tt.exponent)
		}
	}
}

// What v1.4.0 refused stays refused, and so does a sign after the point,
// which v1.4.0 read (".-5" as -0.05), and a Unicode minus, which v1.6.0 reads
// as "-": FHIR's decimal and FHIRPath's String form are ASCII.
func TestDecodeDecimalRefuses(t *testing.T) {
	for _, written := range []string{
		"", ".", "e5", "1e", "1e+", "1e-", " 1", "1 ", "-", "+", "--1", "+-1",
		"1_000", "0x10", "1/2", "NaN", "Inf", "1..2", "1.2.3", "1e5e5", "1e1.5",
		"1e2147483648", "0.05e-2147483647", "1e99999999999999999999",
		".-5", ".+5",
		"−1", "1e−2", "1.5e−2", "−.5",
		"١",
	} {
		if d, err := decodeDecimal(written); err == nil {
			t.Errorf("%q read as %s", written, d)
		}
	}
}

// The public API refuses the Unicode minus too, and a decimal past
// MaxDecodeExponent is beyond the engine's own bound, not a decoding error.
func TestThePublicAPIDecodesAsTheEngineDoes(t *testing.T) {
	for _, written := range []string{"−1", "1e−2"} {
		if _, err := NewDecimal(written); err == nil {
			t.Errorf("NewDecimal(%q) read", written)
		}
		if _, err := ParseDecimal(written); err == nil {
			t.Errorf("ParseDecimal(%q) read", written)
		}
		if _, err := ReadDecimal(written); err == nil {
			t.Errorf("ReadDecimal(%q) read", written)
		}
	}
	for _, written := range []string{"1e20000", "1e-20000", "1e999999999"} {
		if _, err := ParseDecimal(written); !errors.Is(err, ErrDecimalRange) {
			t.Errorf("ParseDecimal(%q): %v, want ErrDecimalRange", written, err)
		}
		if _, err := NewDecimal(written); !errors.Is(err, ErrDecimalRange) {
			t.Errorf("NewDecimal(%q): %v, want ErrDecimalRange", written, err)
		}
		d, err := ReadDecimal(written)
		if err != nil || InDecimalRange(d.Value()) || d.String() != written {
			t.Errorf("ReadDecimal(%q) = %s, %v: want it held beyond the bound", written, d, err)
		}
	}
	// Zeros past the bound, 15000 of them, are read at the bound (#232)
	d, err := ReadDecimal("1." + strings.Repeat("0", 15000))
	if err != nil || d.Value().Exponent() != -DecimalDigits || !d.Value().Equal(NewDecimalFromInt(1).Value()) {
		t.Errorf("1 with 15000 zeros read as %v, %v", d.Value().Exponent(), err)
	}
}

// A short text with a huge exponent is told beyond the bound at once:
// nothing is written out.
func TestAHugeExponentDecodesAtOnce(t *testing.T) {
	start := time.Now()
	for _, written := range []string{"1e2000000000", "1e-2000000000", "9.99e2147483647"} {
		d, err := decodeDecimal(written)
		if err != nil || InDecimalRange(d) {
			t.Errorf("%q: %v, in range %v", written, err, err == nil && InDecimalRange(d))
		}
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Errorf("three huge exponents took %v", elapsed)
	}
}

// acceptedSpelling is the grammar decodeDecimal reads, written apart from it
// as the oracle of FuzzDecodeDecimal: a sign, digits with a point that has
// digits on one side at least, and an exponent.
var acceptedSpelling = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// For any input, decodeDecimal reads exactly the spellings of the grammar,
// and reads each as its value: big.Rat, which reads more than the grammar
// (0x10, 1/2), checks the value only. An exponent written or held past int32
// is refused, as the decimal type holds an int32 (FP-6).
func FuzzDecodeDecimal(f *testing.F) {
	for _, seed := range []string{"0", "-1.50E-3", ".5", "5.", "1e20000", "0.5e-2147483647", ".-5", "1e−2", "1_000", "0x10", "1/2", "+.5e+5"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, written string) {
		d, err := decodeDecimal(written)
		if !acceptedSpelling.MatchString(written) {
			if err == nil {
				t.Fatalf("%q, outside the grammar, read as %s", written, d)
			}
			return
		}
		if err != nil {
			// Only an exponent past int32 may refuse a spelling of the grammar
			if !strings.ContainsAny(written, "eE") && len(written) < 1<<20 {
				t.Fatalf("%q refused: %v", written, err)
			}
			return
		}
		// Check the value where big.Rat writes it out cheaply
		if exponent := d.Exponent(); exponent > 2000 || exponent < -2000 {
			return
		}
		want, ok := new(big.Rat).SetString(written)
		if !ok {
			t.Fatalf("%q: big.Rat refuses it", written)
		}
		if d.Rat().Cmp(want) != 0 {
			t.Fatalf("%q read as %s, want %s", written, d, want.FloatString(20))
		}
	})
}
