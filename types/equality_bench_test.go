package types

import (
	"fmt"
	"strings"
	"testing"
)

// Objects of about 2KB that share all but their end: the pairs in, contains
// and = meet when a resource is looked for among others like it, and those a
// comparison of bytes rejected at once when their lengths differ.
func largeObjects(n int, sameLength bool) Collection {
	pad := strings.Repeat(`{"system":"http://loinc.org","code":"1234-5","display":"A coded value"},`, 25)
	items := make(Collection, n)
	for i := range items {
		tail := fmt.Sprintf(`"id":"%04d"`, i)
		if !sameLength && i%2 == 0 {
			tail += `,"status":"final"`
		}
		items[i] = NewObjectValue([]byte(`{"coding":[` + strings.TrimSuffix(pad, ",") + `],` + tail + `}`))
	}
	return items
}

func benchmarkContains(b *testing.B, sameLength bool) {
	items := largeObjects(200, sameLength)
	probe := NewObjectValue([]byte(strings.Replace(string(items[0].(*ObjectValue).data), `"0000"`, `"9999"`, 1)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if items.Contains(probe) {
			b.Fatal("found")
		}
	}
}

func BenchmarkContainsLargeSameLength(b *testing.B)      { benchmarkContains(b, true) }
func BenchmarkContainsLargeDifferentLength(b *testing.B) { benchmarkContains(b, false) }

func BenchmarkExcludeOneFromMany(b *testing.B) {
	items := largeObjects(500, false)
	one := items[:1]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = one.Exclude(items)
	}
}

func BenchmarkDistinctIntegers(b *testing.B) {
	items := make(Collection, 100)
	for i := range items {
		items[i] = NewInteger(int64(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = items.Distinct()
	}
}

func BenchmarkDistinctLargeObjects(b *testing.B) {
	items := largeObjects(200, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = items.Distinct()
	}
}
