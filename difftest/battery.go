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

// loadBattery reads a battery and the resources it names.
func loadBattery(path string) ([]batteryCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dir := filepath.Dir(path)
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
		file := filepath.Join(dir, fields[0])
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

// evaluateWithJava answers each case with HL7's Java engine, through the
// validator's command line, jobs at a time: a run takes seconds, nearly all of
// it loading packages.
func evaluateWithJava(cases []batteryCase, jar string, jobs int) map[string]answer {
	answers := make(map[string]answer, len(cases))
	var mu sync.Mutex
	work := make(chan batteryCase)
	var wg sync.WaitGroup
	for range max(jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				a := evaluateOneWithJava(c, jar)
				mu.Lock()
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
	return answers
}

// ansiCodes are the terminal colors the validator writes its log in.
var ansiCodes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// evaluateOneWithJava runs one case. The validator writes its log, then
// "...evaluating <expression>", then the result as one line, the items joined
// by commas, or an exception, then "Done.".
func evaluateOneWithJava(c batteryCase, jar string) answer {
	cmd := exec.Command("java", "-Xmx3g", "-jar", jar,
		"-fhirpath", c.Expression, c.file, "-version", c.version, "-tx", "n/a")
	out, _ := cmd.CombinedOutput()

	var lines []string
	evaluating := false
	for _, line := range strings.Split(ansiCodes.ReplaceAllString(string(out), ""), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "...evaluating"):
			evaluating = true
		case strings.HasPrefix(line, "Done."):
			evaluating = false
		case evaluating:
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return answer{ID: c.ID, Error: "no answer read from the validator: " + firstLine(string(out))}
	}
	result := lines[0]
	if javaException.MatchString(result) {
		return answer{ID: c.ID, Error: result}
	}
	if result == "" {
		return answer{ID: c.ID, Results: []string{}}
	}
	// The items are joined by commas, so an item that holds one splits: the
	// report reads such a case as differing, which a reader then checks
	return answer{ID: c.ID, Results: strings.Split(result, ",")}
}

// javaException is how an error starts on the line the validator writes the
// result on: a Java exception's class, or its own "Error evaluating FHIRPath".
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

// sameThreeWay compares two answers as threeWayValue renders their values.
func sameThreeWay(a, b answer) bool {
	if a.failed() || b.failed() {
		return a.failed() && b.failed()
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

// runBattery compares a battery across the engines and prints where they part
// ways. Without a jar it compares this engine and fhirpath.js only.
func runBattery(path, jar string, jobs int, verbose bool) error {
	cases, err := loadBattery(path)
	if err != nil {
		return err
	}
	jsVersion, err := fhirpathJSVersion()
	if err != nil {
		return err
	}

	plain := make([]testCase, len(cases))
	for i, c := range cases {
		plain[i] = c.testCase
	}
	js, err := evaluateWithFHIRPathJS(plain)
	if err != nil {
		return err
	}
	var java map[string]answer
	if jar != "" {
		fmt.Printf("%s: %d cases against fhirpath.js %s and %s, %d at a time\n", path, len(cases), jsVersion, filepath.Base(jar), jobs)
		java = evaluateWithJava(cases, jar, jobs)
	} else {
		fmt.Printf("%s: %d cases against fhirpath.js %s; no -java jar, so not against HL7's Java engine\n", path, len(cases), jsVersion)
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
			groups[order[4]] = append(groups[order[4]], threeLine(label, c.Expression, ours, theirs, answer{}))
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
		groups[group] = append(groups[group], threeLine(label, c.Expression, ours, theirs, hl7))
	}

	fmt.Printf("\n%d agree", agree)
	for _, g := range order {
		if n := len(groups[g]); n > 0 {
			fmt.Printf("; %d %s", n, g)
		}
	}
	fmt.Println()
	for _, g := range order {
		section("battery", g, groups[g], verbose)
	}
	return nil
}

// threeLine renders one case for the battery report.
func threeLine(label, expression string, ours, js, java answer) string {
	s := fmt.Sprintf("  %-28s %s\n      here: %s\n      fhirpath.js: %s",
		label, cut(expression, 90), cut(ours.text(), 70), cut(js.text(), 70))
	if java.ID != "" {
		s += "\n      Java: " + cut(java.text(), 70)
	}
	return s
}
