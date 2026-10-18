package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// A battery is a list of expressions to compare across engines, outside any
// suite: one case a line, "resource<TAB>expression<TAB>version", where the
// resource is a JSON file beside the battery and the version is 4.0.1 (r4) or
// 5.0.0 (r5). Blank lines and lines starting with # are skipped.
//
// It answers the question #167 asks of each function: where this engine, HL7's
// Java FHIRPath engine — the one the validator runs — and fhirpath.js agree,
// and where one of them stands alone. A battery states no expected result; the
// specification text, read case by case, does.

// batteryCase is a case of a battery, with what the Java engine needs to run
// it: the resource's path and the FHIR version.
type batteryCase struct {
	testCase
	file    string
	version string
}

// loadBattery reads a battery and the resources it names. A battery that is
// one of this project's spec-case files, conformance/testdata/spec-cases/
// <area>.xml, is read as those cases with the R4 and the R5 model, as
// -corpus spec reads them; the Java engine reads each input from a file
// written into dir.
func loadBattery(path, dir string) ([]batteryCase, error) {
	if strings.HasSuffix(path, ".xml") {
		return loadSpecBattery(path, dir)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	base := filepath.Dir(path)
	var cases []batteryCase
	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: want resource, expression and version separated by tabs", path, n)
		}
		model, version, ok := fhirVersion(fields[2])
		if !ok {
			return nil, fmt.Errorf("%s:%d: version %q is not 4.0.1, r4, 5.0.0 or r5", path, n, fields[2])
		}
		file := filepath.Join(base, fields[0])
		resource, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		cases = append(cases, batteryCase{
			testCase: testCase{
				ID:         fmt.Sprintf("%s:%d", filepath.Base(path), n),
				Expression: fields[1],
				Resource:   json.RawMessage(resource),
				Model:      model,
			},
			file:    file,
			version: version,
		})
	}
	return cases, scanner.Err()
}

// loadSpecBattery reads a spec-case file as a battery, each case once with the
// R4 model and inputs and once with the R5 ones. A case with no input is
// evaluated by this engine and fhirpath.js against {}, as the spec cases are;
// the Java engine needs a resource to start from, and is given an empty
// Parameters. A case with no input that reads its context, %context or a
// path, reads that Parameters in the Java engine, and a difference there is
// the input's, not the engine's.
func loadSpecBattery(path, dir string) ([]batteryCase, error) {
	written := map[string]string{}
	var cases []batteryCase
	for _, official := range corpora() {
		c := official
		c.file = path
		loaded, err := c.load(true)
		if err != nil {
			return nil, err
		}
		_, version, _ := fhirVersion(c.fhir)
		for _, tc := range loaded {
			if tc.skip != "" {
				continue
			}
			resource := string(tc.Resource)
			if resource == "{}" {
				resource = `{"resourceType":"Parameters"}`
			}
			file, ok := written[resource]
			if !ok {
				file = filepath.Join(dir, fmt.Sprintf("input-%d.json", len(written)))
				if err := os.WriteFile(file, []byte(resource), 0o600); err != nil {
					return nil, err
				}
				written[resource] = file
			}
			tc.ID = c.fhir + " " + tc.ID
			cases = append(cases, batteryCase{testCase: tc, file: file, version: version})
		}
	}
	return cases, nil
}

// fhirVersion reads a battery's version as the model this engine and
// fhirpath.js take and the version the Java engine does.
func fhirVersion(v string) (model, version string, ok bool) {
	switch v {
	case "4.0.1", "r4", "R4":
		return "r4", "4.0.1", true
	case "5.0.0", "r5", "R5":
		return "r5", "5.0.0", true
	}
	return "", "", false
}

// The engines a battery runs, variables so that tests can count what runs.
var (
	evaluateJS   = evaluateWithFHIRPathJS
	evaluateJava = evaluateWithJava
	jsVersion    = fhirpathJSVersion
	lookJava     = func() error { _, err := exec.LookPath("java"); return err }
)

// evaluateWithJava answers each case with HL7's Java engine, through the
// validator's command line, jobs at a time: a run takes seconds, nearly all of
// it loading packages.
func evaluateWithJava(cases []batteryCase, jar string, jobs int) (map[string]answer, error) {
	answers := make(map[string]answer, len(cases))
	var (
		mu       sync.Mutex
		firstErr error
	)
	work := make(chan batteryCase)
	var wg sync.WaitGroup
	for range max(jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				a, err := evaluateOneWithJava(c, jar)
				if err != nil {
					// A run that writes no result under load, a JVM short of
					// memory beside the others, is tried once more before it
					// stops the battery
					a, err = evaluateOneWithJava(c, jar)
				}
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				answers[c.ID] = a
				mu.Unlock()
			}
		}()
	}
	for _, c := range cases {
		work <- c
	}
	close(work)
	wg.Wait()
	return answers, firstErr
}

// ansiCodes are the terminal colors the validator writes its log in.
var ansiCodes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// javaArgs is the validator's command line for a case. An expression that
// starts with a minus, -5 = -5, would be read as an option; a leading space,
// which FHIRPath reads as whitespace, keeps it the expression.
func javaArgs(c batteryCase, jar string) []string {
	expression := c.Expression
	if strings.HasPrefix(expression, "-") {
		expression = " " + expression
	}
	return []string{"-Xmx3g", "-jar", jar,
		"-fhirpath", expression, c.file, "-version", c.version, "-tx", "n/a"}
}

// evaluateOneWithJava runs one case. The validator writes its log, then
// "...evaluating <expression>", then the result as one line, the items joined
// by commas, or an exception, then "Done.". A run that writes no result line —
// no java, a jar that is not the validator, too little memory — is no answer
// of the Java engine, and is reported as the error it is rather than counted
// as one: two refusals compare alike, and it would pass for agreement.
func evaluateOneWithJava(c batteryCase, jar string) (answer, error) {
	cmd := exec.Command("java", javaArgs(c, jar)...)
	out, runErr := cmd.CombinedOutput()

	lines := strings.Split(ansiCodes.ReplaceAllString(string(out), ""), "\n")
	for k, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "...evaluating") {
			continue
		}
		if k+1 == len(lines) {
			break
		}
		// The line after, as written: a value may begin or end with a space
		result := strings.TrimRight(lines[k+1], "\r")
		if javaException.MatchString(result) {
			return answer{ID: c.ID, Error: result, isJava: true}, nil
		}
		return answer{ID: c.ID, Results: splitJavaLine(result), javaLine: result, isJava: true}, nil
	}
	if runErr == nil {
		runErr = fmt.Errorf("no result line")
	}
	return answer{ID: c.ID, Error: "no answer"}, fmt.Errorf("%s: the validator gave no answer (%v): %s", c.ID, runErr, firstLine(string(out)))
}

// splitJavaLine reads a Java result line as items, for the report. An empty
// line is an empty collection or a single empty string alike.
func splitJavaLine(line string) []string {
	if line == "" {
		return []string{}
	}
	return strings.Split(line, ",")
}

// javaException is how an error starts on the line the validator writes the
// result on: a Java exception's class, or its own "Error evaluating FHIRPath".
// A string result that starts so, 'java.lang.Error', reads as one too.
var javaException = regexp.MustCompile(`^((org\.hl7|java\.|ca\.uhn|com\.|org\.)[\w.$]*(Exception|Error)\b|Error evaluating FHIRPath)`)

// javaObject is how the validator writes an object: name=Type[...] or
// Type[n children].
var javaObject = regexp.MustCompile(`^(\w+=)?\w+\[`)

// threeWayValue renders a value for comparing the three engines: an object is
// "<object>" whichever way an engine writes it, since the Java engine names it
// by its type and the others write its JSON. Everything else is compared as
// the suite comparison does.
func threeWayValue(s string) string {
	switch {
	case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
		return "<object>"
	case javaObject.MatchString(s) && strings.HasSuffix(s, "]"):
		return "<object>"
	}
	return trimZeros(normalize(s))
}

// sameThreeWay compares two answers as threeWayValue renders their values. A
// Java answer is compared as its line: the other's items, rendered, joined by
// commas, so that an item holding a comma reads alike on both sides. That
// cannot tell the single string 'a,b' from the two items 'a' and 'b', which
// it takes for alike. An empty Java line is an empty collection or a single
// empty string.
func sameThreeWay(a, b answer) bool {
	if a.failed() || b.failed() {
		return a.failed() && b.failed()
	}
	if b.isJava && !a.isJava {
		a, b = b, a
	}
	if a.isJava && !b.isJava {
		if a.javaLine == "" {
			return len(b.Results) == 0 || len(b.Results) == 1 && b.Results[0] == ""
		}
		return renderedLine(splitJavaLine(a.javaLine)) == renderedLine(b.Results)
	}
	if len(a.Results) != len(b.Results) {
		return false
	}
	for i := range a.Results {
		if threeWayValue(a.Results[i]) != threeWayValue(b.Results[i]) {
			return false
		}
	}
	return true
}

// renderedLine renders items as threeWayValue does and joins them by commas,
// as the Java engine writes them.
func renderedLine(items []string) string {
	rendered := make([]string, len(items))
	for i, item := range items {
		rendered[i] = threeWayValue(item)
	}
	return strings.Join(rendered, ",")
}

// runBattery compares a battery across the engines and prints where they part
// ways. Without a jar it compares this engine and fhirpath.js only.
func runBattery(path, jar, unpinned string, jobs int, verbose bool) error {
	dir, err := os.MkdirTemp("", "battery-")
	if err != nil {
		return err
	}
	// Removed on return, and on an interrupt, which skips defers: the jar's
	// snapshot in it is 188 MB.
	defer onInterrupt(func() { os.RemoveAll(dir) })()
	cases, err := loadBattery(path, dir)
	if err != nil {
		return err
	}

	// The jar is checked, and copied to where every JVM runs it from, before
	// anything is evaluated: a jar that is not the pinned validator stops the
	// battery at once.
	var snap jarSnapshot
	if jar != "" {
		if javaErr := lookJava(); javaErr != nil {
			return fmt.Errorf("running HL7's Java engine: %w", javaErr)
		}
		if snap, err = prepareJar(jar, pinnedValidator, unpinned, dir); err != nil {
			return err
		}
	}

	version, err := jsVersion()
	if err != nil {
		return err
	}
	plain := make([]testCase, len(cases))
	for i, c := range cases {
		plain[i] = c.testCase
	}
	js, err := evaluateJS(plain)
	if err != nil {
		return err
	}
	mark := ""
	if snap.marker != "" {
		mark = snap.marker + " "
	}
	var java map[string]answer
	if jar != "" {
		fmt.Printf("%s%s: %d cases against fhirpath.js %s and %s, %d at a time\n", mark, path, len(cases), version, snap.describe, jobs)
		fmt.Printf("%sthis engine at %s; JVM %s\n", mark, engineRevision(), jvmVersion())
		if java, err = evaluateJava(cases, snap.path, jobs); err != nil {
			return err
		}
	} else {
		fmt.Printf("%s: %d cases against fhirpath.js %s; NOT against HL7's Java engine (no jar: make validator-jar, or -java)\n", path, len(cases), version)
		fmt.Printf("this engine at %s\n", engineRevision())
	}
	groups := map[string][]string{}
	order := []string{
		"this engine alone differs (Java = fhirpath.js)",
		"fhirpath.js alone differs (this engine = Java)",
		"Java alone differs (this engine = fhirpath.js)",
		"the three differ",
		"this engine and fhirpath.js differ",
	}
	agree := 0
	for _, c := range cases {
		ours := evaluateHere(c.testCase)
		theirs := js[c.ID]
		label := fmt.Sprintf("%s [%s]", c.ID, c.version)
		if java == nil {
			if sameThreeWay(ours, theirs) {
				agree++
				continue
			}
			groups[order[4]] = append(groups[order[4]], threeLine(label, c.Expression, ours, theirs, answer{}, ""))
			continue
		}
		hl7 := java[c.ID]
		hereJava, hereJS, javaJS := sameThreeWay(ours, hl7), sameThreeWay(ours, theirs), sameThreeWay(hl7, theirs)
		var group string
		switch {
		case hereJava && hereJS:
			agree++
			continue
		case javaJS:
			group = order[0]
		case hereJava:
			group = order[1]
		case hereJS:
			group = order[2]
		default:
			group = order[3]
		}
		groups[group] = append(groups[group], threeLine(label, c.Expression, ours, theirs, hl7, snap.marker))
	}

	fmt.Printf("\n%s%d agree", mark, agree)
	for _, g := range order {
		if n := len(groups[g]); n > 0 {
			fmt.Printf("; %d %s", n, g)
		}
	}
	fmt.Println()
	for _, g := range order {
		batterySection(mark+g, groups[g], verbose)
	}
	return nil
}

// batterySection prints a group in the battery's own order, which sorting the
// lines as text would not keep: line 102 before line 11.
func batterySection(title string, lines []string, verbose bool) {
	if len(lines) == 0 {
		return
	}
	fmt.Printf("\nbattery: %s (%d)\n", title, len(lines))
	shown := lines
	if !verbose && len(lines) > 12 {
		shown = lines[:12]
	}
	for _, line := range shown {
		fmt.Println(line)
	}
	if len(shown) < len(lines) {
		fmt.Printf("  … %d more; pass -v for all\n", len(lines)-len(shown))
	}
}

// threeLine renders one case for the battery report; an unpinned jar's
// answer carries its marker.
func threeLine(label, expression string, ours, js, java answer, marker string) string {
	s := fmt.Sprintf("  %-28s %s\n      here: %s\n      fhirpath.js: %s",
		label, cut(expression, 90), cut(ours.text(), 70), cut(js.text(), 70))
	if java.ID != "" {
		name := "Java"
		if marker != "" {
			name += " " + marker
		}
		s += "\n      " + name + ": " + cut(java.text(), 70)
	}
	return s
}
