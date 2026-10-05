package fhirpath_test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A FHIR extension context of type fhirpath selects, from the root of the
// resource, the elements an extension may appear on, and a validator checks
// it by asking whether the element holding the extension is among the
// results. That takes knowing where each result sits: two contacts with the
// same content are equal, and their element path is the same, but they are
// not the same element.
func TestAResultKnowsWhereItSitsInTheInput(t *testing.T) {
	patient := `{"resourceType":"Patient",` +
		`"extension":[{"url":"r","valueCode":"root"}],` +
		`"contact":[{"gender":"male"},{"gender":"male","name":{"family":"F"}}],` +
		`"name":[{"use":"official","family":"F"},{"use":"usual","given":["Peter",null,"James"],` +
		`"_given":[null,{"extension":[{"url":"u","valueCode":"absent"}]},{"id":"g2"}]}],` +
		`"_birthDate":{"id":"b1"},` +
		`"deceasedBoolean":false,"_deceasedBoolean":{"id":"d1"},` +
		`"contained":[{"resourceType":"Organization","id":"o1"}],` +
		`"managingOrganization":{"reference":"#o1"}}`
	bundle := `{"resourceType":"Bundle","entry":[` +
		`{"fullUrl":"urn:p","resource":` + patient + `},` +
		`{"resource":{"resourceType":"Observation","subject":{"reference":"urn:p"}}}]}`

	tests := []struct {
		input, expr string
		want        []string
	}{
		// The root is named by its resourceType: an extension on the root,
		// under a context that selects the root, is a valid case.
		{patient, "Patient", []string{"Patient"}},
		{patient, "%resource", []string{"Patient"}},
		{patient, "Patient.extension", []string{"Patient.extension[0]"}},
		{patient, "Patient.contact.last()", []string{"Patient.contact[1]"}},
		{patient, "Patient.contact", []string{"Patient.contact[0]", "Patient.contact[1]"}},
		{patient, "Patient.contact.where(gender = 'male')", []string{"Patient.contact[0]", "Patient.contact[1]"}},
		{patient, "Patient.contact.name", []string{"Patient.contact[1].name"}},
		{patient, "Patient.name.where(use = 'official')", []string{"Patient.name[0]"}},
		{patient, "Patient.managingOrganization", []string{"Patient.managingOrganization"}},
		// A primitive's position is its element's, which is named without
		// the underscore FHIR writes it under, and an index counts the nulls
		// of the JSON array.
		{patient, "Patient.name.given", []string{"", "Patient.name[1].given[1]", "Patient.name[1].given[2]"}},
		{patient, "Patient.birthDate", []string{"Patient.birthDate"}},
		{patient, "Patient.name.given.extension", []string{"Patient.name[1].given[1].extension[0]"}},
		// A choice element sits under the key that spells its type.
		{patient, "Patient.deceased", []string{"Patient.deceasedBoolean"}},
		{patient, "Patient.children().where(use = 'official')", []string{"Patient.name[0]"}},
		{patient, "Patient.descendants().where(url = 'u')", []string{"Patient.name[1].given[1].extension[0]"}},
		{patient, "Patient.managingOrganization.resolve()", []string{"Patient.contained[0]"}},
		{bundle, "Bundle.entry.resource.ofType(Patient).contact.first()", []string{"Bundle.entry[0].resource.contact[0]"}},
		{bundle, "Bundle.entry.resource.ofType(Patient).name.given[2]", []string{"Bundle.entry[0].resource.name[1].given[2]"}},
		{bundle, "Bundle.entry[1].resource.subject.resolve()", []string{"Bundle.entry[0].resource"}},
		{bundle, "Bundle.entry[1].resource.subject.resolve().contact[1]", []string{"Bundle.entry[0].resource.contact[1]"}},
		// An object a function made was not read from the input.
		{patient, "Patient.type()", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)

			// A root a caller reads once and shares.
			col, err := types.JSONToCollection([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			result, err := compiled.EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil {
				t.Fatal(err)
			}
			checkLocations(t, "", result, tt.want)

			// A root the evaluation reads for itself, privately, and the
			// same unchecked.
			result, err = compiled.Evaluate([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			checkLocations(t, "from Evaluate ", result, tt.want)

			result, err = compiled.EvaluateWithContext(eval.NewContextForValidJSON([]byte(tt.input)))
			if err != nil {
				t.Fatal(err)
			}
			checkLocations(t, "from valid JSON ", result, tt.want)

			// A Document, whose objects cache what they read.
			doc := fhirpath.MustNewDocument([]byte(tt.input))
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateCompiled(compiled)
				if err != nil {
					t.Fatal(err)
				}
				checkLocations(t, "on a Document ", result, tt.want)
			}
		})
	}
}

// A resource a Resolver hands back was not read from the input, so it has no
// location in it — not that of a root, which it would share with the
// resource being evaluated.
func TestAResolvedResourceHasNoLocation(t *testing.T) {
	ctx := eval.NewContext([]byte(`{"resourceType":"Patient","generalPractitioner":[{"reference":"Patient/2"}]}`))
	ctx.SetResolver(staticResolver(`{"resourceType":"Patient","id":"2","contact":[{}]}`))

	for expr, want := range map[string][]string{
		"Patient.generalPractitioner.resolve()":         {""},
		"Patient.generalPractitioner.resolve().contact": {""},
		"Patient.generalPractitioner":                   {"Patient.generalPractitioner[0]"},
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkLocations(t, expr+": ", result, want)
	}
}

type staticResolver string

func (r staticResolver) Resolve(context.Context, string) ([]byte, error) {
	return []byte(r), nil
}

// An element read as the root has no resourceType to be named by, so its
// fields are located from nothing; the items of an array read as the root are
// no input's root.
func TestARootWithoutAResourceType(t *testing.T) {
	col, err := types.JSONToCollectionWithType([]byte(`{"family":"F","given":["A","B"],"_given":[null,{"id":"x"}]}`), "HumanName")
	if err != nil {
		t.Fatal(err)
	}
	result, err := fhirpath.MustCompile("given").EvaluateWithContext(eval.NewContextForRoot(col))
	if err != nil {
		t.Fatal(err)
	}
	checkLocations(t, "", result, []string{"", "given[1]"})

	col, err = types.JSONToCollection([]byte(`[{"resourceType":"Patient","contact":[{}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	result, err = fhirpath.MustCompile("contact").EvaluateWithContext(eval.NewContextForRoot(col))
	if err != nil {
		t.Fatal(err)
	}
	checkLocations(t, "array root: ", result, []string{""})
}

// Location only reads, so the results of evaluations against one shared root
// can be located from several goroutines at once: run with -race.
func TestLocationIsReadFromSeveralGoroutines(t *testing.T) {
	col, err := types.JSONToCollection([]byte(`{"resourceType":"Patient","contact":[{"gender":"male"},{"gender":"male"}],` +
		`"name":[{"given":["A","B"],"_given":[null,{"id":"x"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	compiled := fhirpath.MustCompile("Patient.contact | Patient.name.given.where($this = 'B')")
	shared, err := compiled.EvaluateWithContext(eval.NewContextForRoot(col))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := compiled.EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil {
				t.Error(err)
				return
			}
			checkLocations(t, "", result, []string{"Patient.contact[0]", "Patient.name[0].given[1]"})
			checkLocations(t, "shared: ", shared, []string{"Patient.contact[0]", "Patient.name[0].given[1]"})
		}()
	}
	wg.Wait()
}

func checkLocations(t *testing.T, where string, result types.Collection, want []string) {
	t.Helper()
	if len(result) != len(want) {
		t.Fatalf("%sgot %d results (%s), want %d", where, len(result), result, len(want))
	}
	for i, value := range result {
		got := ""
		if element, ok := types.ElementOf(value); ok {
			got = element.Location()
		}
		if got != want[i] {
			t.Errorf("%sresult %d is at %q, want %q", where, i, got, want[i])
		}
	}
}

// Location reads each ancestor's JSON to spell the path, so it costs about
// what reading the path again does: this is a result three levels into an
// entry of a Bundle of a hundred.
func BenchmarkLocation(b *testing.B) {
	entry := `{"resource":{"resourceType":"Patient","name":[{"family":"F","given":["A","B"]}],` +
		`"contact":[{"gender":"male"},{"gender":"male","name":{"family":"F","given":["A"]}}]}}`
	bundle := `{"resourceType":"Bundle","entry":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			bundle += ","
		}
		bundle += entry
	}
	bundle += `]}`

	col, err := types.JSONToCollection([]byte(bundle))
	if err != nil {
		b.Fatal(err)
	}
	result, err := fhirpath.MustCompile("Bundle.entry[50].resource.contact[1].name").EvaluateWithContext(eval.NewContextForRoot(col))
	if err != nil || len(result) != 1 {
		b.Fatal(err, result)
	}
	obj := result[0].(*types.ObjectValue)
	if got := obj.Location(); got != "Bundle.entry[50].resource.contact[1].name" {
		b.Fatal(got)
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = obj.Location()
	}
}

// Locating every item of an array costs the depth of each path, not the size
// of the array: per entry, the time is the same for a Bundle of 1000 entries
// as for one of 8000.
func BenchmarkLocationOfEveryEntry(b *testing.B) {
	for _, n := range []int{1000, 8000} {
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			var bundle strings.Builder
			bundle.WriteString(`{"resourceType":"Bundle","entry":[`)
			for i := 0; i < n; i++ {
				if i > 0 {
					bundle.WriteByte(',')
				}
				fmt.Fprintf(&bundle, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d"}}`, i, i)
			}
			bundle.WriteString(`]}`)

			col, err := types.JSONToCollection([]byte(bundle.String()))
			if err != nil {
				b.Fatal(err)
			}
			entries, err := fhirpath.MustCompile("Bundle.entry").EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil || len(entries) != n {
				b.Fatal(err, len(entries))
			}
			if got := entries[n-1].(*types.ObjectValue).Location(); got != fmt.Sprintf("Bundle.entry[%d]", n-1) {
				b.Fatal(got)
			}

			b.ReportAllocs()
			for b.Loop() {
				for _, entry := range entries {
					_ = entry.(*types.ObjectValue).Location()
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/entry")
		})
	}
}

// JSON does not order keys, and a producer may write resourceType after the
// entries: Go's encoding/json sorts a map's keys. The root is named the same
// wherever its resourceType is, and without one by nothing, and locating the
// first entries from several goroutines at once, which is when the name is
// read and kept, races on nothing: run with -race.
func TestARootIsNamedWhereverItsResourceTypeIs(t *testing.T) {
	entries := `"entry":[{"resource":{"resourceType":"Patient","id":"a"}},{"resource":{"resourceType":"Patient","id":"b"}}]`
	for _, tt := range []struct {
		name, json, want string
	}{
		{"first", `{"resourceType":"Bundle",` + entries + `}`, "Bundle.entry[1]"},
		{"last", `{` + entries + `,"resourceType":"Bundle"}`, "Bundle.entry[1]"},
		{"absent", `{` + entries + `}`, "entry[1]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			read := map[string]func([]byte) (types.Collection, error){
				"JSONToCollection": types.JSONToCollection,
				"ReadRoot":         types.ReadRoot,
				"ReadRootWithType": func(data []byte) (types.Collection, error) {
					return types.ReadRootWithType(data, "Bundle")
				},
			}
			for how, readRoot := range read {
				col, err := readRoot([]byte(tt.json))
				if err != nil {
					t.Fatal(err)
				}
				result, err := fhirpath.MustCompile("entry").EvaluateWithContext(eval.NewContextForRoot(col))
				if err != nil || len(result) != 2 {
					t.Fatal(err, result)
				}

				var wg sync.WaitGroup
				for range 8 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if got := result[1].(*types.ObjectValue).Location(); got != tt.want {
							t.Errorf("%s: entry is at %q, want %q", how, got, tt.want)
						}
					}()
				}
				wg.Wait()
				if got := col[0].(*types.ObjectValue).Location(); got != strings.TrimSuffix(strings.TrimSuffix(tt.want, "entry[1]"), ".") {
					t.Errorf("%s: root is at %q", how, got)
				}
			}
		})
	}
}

// Locating every entry of a Bundle costs the same per entry however large the
// Bundle is and wherever its resourceType is written: per entry, a Bundle of
// 8000 entries takes about what one of 500 does, where reading past the
// entries for each one took sixteen times as long. The ratio is of the best of
// several passes on the same machine, so it holds on a slow or busy one, and
// a margin of three keeps noise from failing it while still catching a cost
// that grows with the Bundle.
func TestLocatingEveryEntryCostsTheSamePerEntryAtAnySize(t *testing.T) {
	perEntry := func(n int, last bool) time.Duration {
		var bundle strings.Builder
		bundle.WriteString("{")
		if !last {
			bundle.WriteString(`"resourceType":"Bundle",`)
		}
		bundle.WriteString(`"entry":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				bundle.WriteByte(',')
			}
			fmt.Fprintf(&bundle, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d"}}`, i, i)
		}
		bundle.WriteString("]")
		if last {
			bundle.WriteString(`,"resourceType":"Bundle"`)
		}
		bundle.WriteString("}")

		col, err := types.JSONToCollection([]byte(bundle.String()))
		if err != nil {
			t.Fatal(err)
		}
		entries, err := fhirpath.MustCompile("Bundle.entry").EvaluateWithContext(eval.NewContextForRoot(col))
		if err != nil || len(entries) != n {
			t.Fatal(err, len(entries))
		}

		best := time.Duration(math.MaxInt64)
		for range 15 {
			start := time.Now()
			for _, entry := range entries {
				_ = entry.(*types.ObjectValue).Location()
			}
			elapsed := time.Since(start)
			best = min(best, elapsed)
			if elapsed > 2*time.Second {
				break // already far past linear; no need to wait out the rest
			}
		}
		return best / time.Duration(n)
	}

	for _, last := range []bool{false, true} {
		small, large := perEntry(500, last), perEntry(8000, last)
		ratio := float64(large) / float64(max(small, 1))
		t.Logf("resourceType last=%v: %v per entry at 500, %v at 8000 (%.1fx)", last, small, large, ratio)
		if ratio > 3 {
			t.Errorf("resourceType last=%v: an entry of a Bundle of 8000 takes %.1fx what one of 500 does, want about the same", last, ratio)
		}
	}
}
