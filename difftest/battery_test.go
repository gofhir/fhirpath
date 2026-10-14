package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Java answer is compared as the line the engine writes, so an item that
// holds a comma does not split into two, and an empty line is an empty
// collection or a single empty string.
func TestSameThreeWayReadsAJavaLineAsWritten(t *testing.T) {
	java := func(line string) answer {
		return answer{Results: splitJavaLine(line), javaLine: line, isJava: true}
	}
	here := func(items ...string) answer { return answer{Results: items} }

	for _, tc := range []struct {
		name  string
		a, b  answer
		alike bool
	}{
		{"an item with a comma", here("a,b", "c"), java("a,b,c"), true},
		{"a value with spaces at its ends", here(" f "), java(" f "), true},
		{"spaces are content", here("f"), java(" f "), false},
		{"an empty line against none", here(), java(""), true},
		{"an empty line against an empty string", here(""), java(""), true},
		{"an empty line against a value", here("x"), java(""), false},
		{"objects alike", here(`{"family":"F"}`), java("name=HumanName[1 children]"), true},
		{"decimals alike", here("1.50"), java("1.5"), true},
		{"an error against an answer", answer{Error: "x"}, java("1"), false},
		{"in either order", java("a,b"), here("a", "b"), true},
	} {
		if got := sameThreeWay(tc.a, tc.b); got != tc.alike {
			t.Errorf("%s: sameThreeWay = %v, want %v", tc.name, got, tc.alike)
		}
	}
}

// A battery run with a jar that does not exist is refused, rather than every
// case read as an error of the Java engine.
func TestABatteryWithAMissingJarIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.json"), []byte(`{"resourceType":"Patient"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	battery := filepath.Join(dir, "b.tsv")
	if err := os.WriteFile(battery, []byte("p.json\tPatient.exists()\t4.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runBattery(battery, filepath.Join(dir, "missing.jar"), 1, false); err == nil {
		t.Error("runBattery with a missing jar: no error")
	}
}

// A spec-case file reads as a battery of its cases with each version's inputs,
// and a case with no input gives the Java engine a resource to start from.
func TestASpecCaseFileReadsAsABattery(t *testing.T) {
	dir := t.TempDir()
	area := filepath.Join(dir, "area.xml")
	if err := os.WriteFile(area, []byte(`<tests name="area">
  <group name="REQ-AREA-001">
    <test name="REQ-AREA-001-a"><expression>1.exists()</expression><output type="boolean">true</output></test>
  </group>
</tests>`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases, err := loadBattery(area, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want the case once per version", len(cases))
	}
	versions := map[string]bool{}
	for _, c := range cases {
		versions[c.version] = true
		if string(c.Resource) != "{}" {
			t.Errorf("%s: this engine and fhirpath.js read %s, want {}", c.ID, c.Resource)
		}
		data, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"resourceType"`) {
			t.Errorf("%s: the Java engine reads %s, want a resource", c.ID, data)
		}
	}
	if !versions["4.0.1"] || !versions["5.0.0"] {
		t.Errorf("versions %v, want 4.0.1 and 5.0.0", versions)
	}
}
