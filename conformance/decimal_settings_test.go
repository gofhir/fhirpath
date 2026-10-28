package conformance

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// decimalSettingsEnv, in a re-executed test binary, changes every
// process-wide setting of shopspring/decimal before any test runs.
const decimalSettingsEnv = "FHIRPATH_TEST_DECIMAL_SETTINGS"

func TestMain(m *testing.M) {
	if os.Getenv(decimalSettingsEnv) != "" {
		decimal.TrimTrailingZeros = false
		decimal.UseScientificNotation = true
		decimal.DivisionPrecision = 4
		decimal.PowPrecisionNegativeExponent = 2
		decimal.ExpMaxIterations = 1
		decimal.MarshalJSONWithoutQuotes = true
		decimal.MaxDecodeExponent = 1
		decimal.Zero = decimal.New(1, 0)
	}
	os.Exit(m.Run())
}

// The suites pass and fail the same cases, each against its baseline, with
// every setting of the decimal library changed (#237): a program that
// embeds the engine and sets one changes no answer.
func TestTheSuitesAnswerAlikeUnderTheDecimalLibrarysSettings(t *testing.T) {
	if os.Getenv(decimalSettingsEnv) != "" {
		return
	}
	if *updateKnownFailures {
		t.Skip("the baselines are being rewritten; this run compares with them")
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), decimalSettingsEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("with the decimal library's settings changed: %v\n%s", err, out)
	}
	// Both suites ran, and no case was skipped
	for _, suite := range []string{"TestOfficialSuite", "TestSpecCases"} {
		if !strings.Contains(string(out), "--- PASS: "+suite+" ") {
			t.Errorf("with the decimal library's settings changed, %s did not run", suite)
		}
	}
	if strings.Contains(string(out), "--- SKIP") {
		t.Errorf("with the decimal library's settings changed, a test was skipped:\n%s", out)
	}
}
