package conformance

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// A known failure stays on the list only with a written reason. The baselines
// say which cases fail; known-failure-reasons.txt says why, and points at the
// section of CONFORMANCE.md that argues it, so that a gap nobody has explained
// cannot sit next to a suite defect that has been and look the same.
//
// Only the model baselines are checked. A case that fails without a model and
// passes with one is explained by the difference between the two runs.

const conformanceDoc = "../CONFORMANCE.md"

var suiteFailureKinds = map[string]bool{
	"suite-defect":   true,
	"deliberate":     true,
	"needs-external": true,
	"gap":            true,
}

var specFailureKinds = map[string]bool{
	"deliberate":     true,
	"needs-external": true,
	"gap":            true,
}

type failureReason struct {
	kind, anchor, text string
}

func TestKnownFailuresAreExplained(t *testing.T) {
	anchors := headingAnchors(t, conformanceDoc)

	for _, suite := range []struct{ name, baseline, reasons string }{
		{"R4", knownFailuresModelFile, suiteDir + "/known-failure-reasons.txt"},
		{"R5", knownFailuresModelFileR5, suiteDirR5 + "/known-failure-reasons.txt"},
	} {
		t.Run(suite.name, func(t *testing.T) {
			checkFailureReasons(t, anchors, suiteFailureKinds, suite.reasons, suite.baseline)
		})
	}

	// A spec case is this project's own, so a failing one is never the
	// suite's fault: a wrong case is corrected, not listed. Both versions
	// share one reasons file, since a case is named alike in both.
	for _, area := range specAreas(t) {
		t.Run("spec cases "+area, func(t *testing.T) {
			checkFailureReasons(t, anchors, specFailureKinds, specReasonsFile(area),
				specBaselineFile(area, "r4"), specBaselineFile(area, "r5"))
		})
	}
}

// checkFailureReasons checks that the cases failing in any of the baselines
// and the cases a reasons file explains are the same cases.
func checkFailureReasons(t *testing.T, anchors, kinds map[string]bool, reasonsPath string, baselines ...string) {
	t.Helper()
	known := map[string]bool{}
	for _, b := range baselines {
		for id := range loadKnownFailures(t, b) {
			known[id] = true
		}
	}
	reasons := loadFailureReasons(t, reasonsPath)

	for _, id := range sortedKeys(known) {
		if _, ok := reasons[id]; !ok {
			t.Errorf("%s fails and %s does not say why", id, reasonsPath)
		}
	}
	for _, id := range sortedKeys(reasons) {
		r := reasons[id]
		if !known[id] {
			t.Errorf("%s is explained in %s but fails in none of %s; remove its line", id, reasonsPath, strings.Join(baselines, ", "))
		}
		if !kinds[r.kind] {
			t.Errorf("%s: kind %q is not one of %s", id, r.kind, strings.Join(sortedKeys(kinds), ", "))
		}
		// A pending gap may not have a section yet; anything else is a
		// position taken, and is argued in CONFORMANCE.md.
		if (r.kind != "gap" || r.anchor != "-") && !anchors[r.anchor] {
			t.Errorf("%s: %s has no heading with anchor #%s", id, conformanceDoc, r.anchor)
		}
		if r.text == "" {
			t.Errorf("%s: no reason given", id)
		}
	}
}

func loadFailureReasons(t *testing.T, path string) map[string]failureReason {
	t.Helper()
	reasons := map[string]failureReason{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return reasons // nothing fails, or the baseline will say so
	}
	if err != nil {
		t.Fatalf("read failure reasons: %v", err)
	}
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			t.Errorf("%s:%d: want \"group/test kind anchor reason\", got %q", path, n+1, line)
			continue
		}
		id := fields[0]
		if _, dup := reasons[id]; dup {
			t.Errorf("%s:%d: %s is explained twice", path, n+1, id)
		}
		reasons[id] = failureReason{kind: fields[1], anchor: fields[2], text: strings.Join(fields[3:], " ")}
	}
	return reasons
}

// headingAnchors returns the anchors GitHub gives the headings of a Markdown
// file: lower case, punctuation dropped, spaces turned into hyphens, and a
// numeric suffix on a repeated heading.
func headingAnchors(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	anchors := map[string]bool{}
	seen := map[string]int{}
	inFence := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		var b strings.Builder
		for _, r := range strings.ToLower(heading) {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
				b.WriteRune(r)
			case r == ' ':
				b.WriteRune('-')
			}
		}
		anchor := b.String()
		if n := seen[anchor]; n > 0 {
			anchors[fmt.Sprintf("%s-%d", anchor, n)] = true
		} else {
			anchors[anchor] = true
		}
		seen[anchor]++
	}
	return anchors
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
