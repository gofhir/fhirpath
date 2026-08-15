package fhirpath

import (
	"fmt"
	"testing"
)

// Compilation descends into a function's arguments, so anything it does twice
// per call is paid once per level of nesting — squared, then cubed. This is
// where that shows: select(select(select(...))) nine deep.
func BenchmarkCompileNestedArgs(b *testing.B) {
	for _, depth := range []int{1, 5, 9} {
		arg := "given"
		for i := 0; i < depth; i++ {
			arg = "select(" + arg + ")"
		}
		expr := "Patient.name." + arg

		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Compile(expr)
			}
		})
	}
}
