package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// makeBasic builds a Basic whose size is its n extensions. It has code and no
// subject, so the two compare a field that is there with one that is not.
func makeBasic(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Basic","id":"b","code":{"text":"c"},"extension":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"url":"http://example.org/%d","valueString":"v%d"}`, i, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

// A field that is absent is what constraints ask about all the time —
// contained, modifierExtension, text — and before it is answered empty it is
// tried as each type a choice element could take. That should cost about what
// reading a field that is there does, not a multiple of it that grows with the
// resource.
func BenchmarkAbsentField(b *testing.B) {
	model := &testModel{
		typeOf: map[string]string{
			"Basic.code":    "CodeableConcept",
			"Basic.subject": "Reference",
		},
		// The choice types of Observation.value, lent to Basic so that an absent
		// choice element is measured on the same resource.
		choiceTypes: map[string][]string{"Basic.value": {
			"Quantity", "CodeableConcept", "string", "boolean", "integer", "Range",
			"Ratio", "SampledData", "time", "dateTime", "Period",
		}},
	}

	for _, n := range []int{1000, 16000} {
		data := makeBasic(n)
		for _, expr := range []string{"code.exists()", "subject.exists()", "value.exists()"} {
			compiled := fhirpath.MustCompile(expr)

			b.Run(fmt.Sprintf("extensions=%d/%s", n, expr), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = compiled.Evaluate(data)
				}
			})

			b.Run(fmt.Sprintf("extensions=%d/%s/model", n, expr), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = compiled.EvaluateWithOptions(data, fhirpath.WithModel(model))
				}
			})
		}

		// A Document keeps what it read, so asking again should stay free.
		doc := fhirpath.MustNewDocument(data)
		absent := fhirpath.MustCompile("subject.exists()")
		b.Run(fmt.Sprintf("extensions=%d/subject.exists()/document", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = doc.EvaluateCompiled(absent)
			}
		})
	}
}
