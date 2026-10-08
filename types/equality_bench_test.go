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

// largeResources writes n resources of about size bytes that differ early, at
// their id, as resources in a Bundle do, half of them a little longer.
func largeResources(n, size int) Collection {
	body := strings.Repeat(`{"system":"http://loinc.org","code":"1234-5","display":"A coded value"},`, size/72)
	items := make(Collection, n)
	for i := range items {
		status := ""
		if i%2 == 0 {
			status = `"status":"final",`
		}
		items[i] = NewObjectValue(fmt.Appendf(nil, `{"resourceType":"Observation","id":"%04d",%s"code":{"coding":[%s]}}`,
			i, status, strings.TrimSuffix(body, ",")))
	}
	return items
}

func BenchmarkDistinctResourcesDifferingEarly(b *testing.B) {
	items := largeResources(100, 50_000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = items.Distinct()
	}
}

func BenchmarkExcludeEightResourcesDifferingEarly(b *testing.B) {
	items := largeResources(500, 2_000)
	some := append(Collection{}, items[492:]...)
	for i := range some {
		some[i] = NewObjectValue(append([]byte{}, some[i].(*ObjectValue).data...))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = some.Exclude(items[:480])
	}
}

// sharedPrefixObjects writes n objects that share a prefix of about size
// bytes and differ only in a trailing note of a different length each, as
// entries of a transaction with no ids do.
func sharedPrefixObjects(n, size int) Collection {
	body := strings.Repeat(`{"system":"http://loinc.org","code":"1234-5","display":"A coded value"},`, size/72)
	items := make(Collection, n)
	for i := range items {
		items[i] = NewObjectValue(fmt.Appendf(nil, `{"resourceType":"Observation","code":{"coding":[%s]},"note":[{"text":"%s"}]}`,
			strings.TrimSuffix(body, ","), strings.Repeat("n", i+1)))
	}
	return items
}

func benchmarkDistinctSharedPrefix(b *testing.B, size int) {
	items := sharedPrefixObjects(100, size)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = items.Distinct()
	}
}

func BenchmarkDistinctSharedPrefix1K(b *testing.B)  { benchmarkDistinctSharedPrefix(b, 1_000) }
func BenchmarkDistinctSharedPrefix10K(b *testing.B) { benchmarkDistinctSharedPrefix(b, 10_000) }
