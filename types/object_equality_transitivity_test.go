package types

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Plan 024: = between objects is transitive. A shortcut answers only where the
// reader reads the text as written, so every answer of equalJSON is the one the
// reader gives. The expected values below come from the plan's text and tables,
// never from running the engine.

// equalJSONReader is equalJSON with every shortcut off, at every depth: each
// pair read child by child, the reference the shortcuts must agree with.
func equalJSONReader(a, b []byte) bool {
	previous := shortcuts
	shortcuts = false
	defer func() { shortcuts = previous }()
	return equalJSON(a, b)
}

// jsonMember is one member of a generated object, written as its key text and
// its value.
type jsonMember struct {
	key   string
	value *jsonNode
}

// jsonNode is a generated value: a scalar written as text, an object or an
// array.
type jsonNode struct {
	scalar  string
	members []jsonMember // an object, when isObject
	items   []*jsonNode  // an array, when isArray
	object  bool
	array   bool
}

// generatorNumbers are the writings of numbers the generator uses. Every one
// decodes (TestGeneratorNumbersDecode): the numbers decodeDecimal refuses are
// out of the plan's scope.
var generatorNumbers = []string{
	"1", "1.0", "1e0", "10e-1", "1.00e-0", "-0", "0", "0.0e0", "1E2", "100", "1.5e1", "15", "0.5", "5e-1",
	"-1", "-1.0", "12345678901234567890.123456789", "1e-9", "0.000000001",
}

// generatorStrings are the writings of strings: the same text escaped or not.
var generatorStrings = []string{
	`"a"`, `"b"`, `"a b"`, `"a"`, `"é"`, `"é"`, `"a\"b"`, `"x\\y"`, `"1"`, `""`, `"2020-01-01"`,
}

// generatorKeys are the keys of generated members. Escaped keys read as the
// key they spell.
var generatorKeys = []string{"a", "b", "c", "_a", "_b", "\\u0061", "\\u005fa", "n", "x"}

// objectGenerator writes objects by hand, key by key, so that a key is written
// twice where a generator through a map could not.
type objectGenerator struct {
	rng     *rand.Rand
	numbers map[string]bool // every number written, for the check that they decode
}

func newObjectGenerator(seed int64) *objectGenerator {
	return &objectGenerator{rng: rand.New(rand.NewSource(seed)), numbers: map[string]bool{}}
}

func (g *objectGenerator) number() *jsonNode {
	n := generatorNumbers[g.rng.Intn(len(generatorNumbers))]
	g.numbers[n] = true
	return &jsonNode{scalar: n}
}

// scalar draws a number, a string, a Boolean or a null.
func (g *objectGenerator) scalar() *jsonNode {
	switch g.rng.Intn(8) {
	case 0, 1, 2:
		return g.number()
	case 3, 4:
		return &jsonNode{scalar: generatorStrings[g.rng.Intn(len(generatorStrings))]}
	case 5:
		return &jsonNode{scalar: []string{"true", "false"}[g.rng.Intn(2)]}
	case 6:
		return &jsonNode{scalar: "null"}
	}
	return &jsonNode{scalar: generatorStrings[g.rng.Intn(3)]}
}

// value draws a value that may hold more, to the depth left.
func (g *objectGenerator) value(depth int) *jsonNode {
	if depth == 0 {
		return g.scalar()
	}
	switch g.rng.Intn(10) {
	case 0, 1, 2:
		return g.object(depth - 1)
	case 3, 4:
		node := &jsonNode{array: true}
		for range g.rng.Intn(4) {
			node.items = append(node.items, g.value(depth-1))
		}
		return node
	case 5:
		// A nested array, whose numbers are written in several ways
		inner := &jsonNode{array: true}
		for range 1 + g.rng.Intn(2) {
			inner.items = append(inner.items, g.number())
		}
		return &jsonNode{array: true, items: []*jsonNode{inner}}
	case 6:
		// An element with content, or one without
		if g.rng.Intn(2) == 0 {
			return &jsonNode{object: true, members: []jsonMember{{"resourceType", &jsonNode{scalar: generatorStrings[g.rng.Intn(2)]}}}}
		}
		return &jsonNode{object: true}
	}
	return g.scalar()
}

// object draws an object of up to four members. A key already written is
// written again, in place, one time in six.
func (g *objectGenerator) object(depth int) *jsonNode {
	node := &jsonNode{object: true}
	for range g.rng.Intn(5) {
		key := generatorKeys[g.rng.Intn(len(generatorKeys))]
		if len(node.members) > 0 && g.rng.Intn(6) == 0 {
			key = node.members[g.rng.Intn(len(node.members))].key
		}
		node.members = append(node.members, jsonMember{key, g.value(depth)})
	}
	return node
}

// clone copies a node.
func (n *jsonNode) clone() *jsonNode {
	c := *n
	c.members = make([]jsonMember, len(n.members))
	for i, m := range n.members {
		c.members[i] = jsonMember{m.key, m.value.clone()}
	}
	c.items = make([]*jsonNode, len(n.items))
	for i, item := range n.items {
		c.items[i] = item.clone()
	}
	return &c
}

// nodes lists the objects and arrays under a node, itself included.
func (n *jsonNode) containers(into []*jsonNode) []*jsonNode {
	if !n.object && !n.array {
		return into
	}
	into = append(into, n)
	for _, m := range n.members {
		into = m.value.containers(into)
	}
	for _, item := range n.items {
		into = item.containers(into)
	}
	return into
}

// mutate changes one thing in a copy of a node: a value written another way, a
// member dropped, added, written twice, in another order, or a scalar changed.
func (g *objectGenerator) mutate(n *jsonNode) *jsonNode {
	c := n.clone()
	all := c.containers(nil)
	if len(all) == 0 {
		return c
	}
	target := all[g.rng.Intn(len(all))]
	switch {
	case target.object && len(target.members) > 0:
		i := g.rng.Intn(len(target.members))
		switch g.rng.Intn(6) {
		case 0:
			target.members = append(target.members[:i], target.members[i+1:]...)
		case 1:
			target.members[i].value = g.value(1)
		case 2:
			// The key written again after the member, with another value
			dup := jsonMember{target.members[i].key, g.value(1)}
			target.members = append(target.members[:i+1], append([]jsonMember{dup}, target.members[i+1:]...)...)
		case 3:
			j := g.rng.Intn(len(target.members))
			target.members[i], target.members[j] = target.members[j], target.members[i]
		case 4:
			target.members[i].key = generatorKeys[g.rng.Intn(len(generatorKeys))]
		default:
			target.members = append(target.members, jsonMember{generatorKeys[g.rng.Intn(len(generatorKeys))], g.value(1)})
		}
	case target.object:
		target.members = append(target.members, jsonMember{generatorKeys[g.rng.Intn(len(generatorKeys))], g.value(1)})
	case len(target.items) > 0:
		i := g.rng.Intn(len(target.items))
		if g.rng.Intn(2) == 0 {
			target.items[i] = g.value(1)
		} else {
			target.items = append(target.items[:i], target.items[i+1:]...)
		}
	default:
		target.items = append(target.items, g.value(1))
	}
	return c
}

// write writes a node as text. Members of an object are written in the order
// held, or in the opposite one when reversed is asked of that object; a node
// is laid out with spaces when spaced is true.
func (n *jsonNode) write(b *strings.Builder, reversed, spaced bool) {
	sep := ","
	colon := ":"
	if spaced {
		sep, colon = ", ", " : "
	}
	switch {
	case n.object:
		b.WriteByte('{')
		for k := range n.members {
			i := k
			if reversed {
				i = len(n.members) - 1 - k
			}
			if k > 0 {
				b.WriteString(sep)
			}
			b.WriteByte('"')
			b.WriteString(n.members[i].key)
			b.WriteByte('"')
			b.WriteString(colon)
			n.members[i].value.write(b, reversed, spaced)
		}
		b.WriteByte('}')
	case n.array:
		b.WriteByte('[')
		for i, item := range n.items {
			if i > 0 {
				b.WriteString(sep)
			}
			item.write(b, reversed, spaced)
		}
		b.WriteByte(']')
	default:
		b.WriteString(n.scalar)
	}
}

// text writes a node's text, at the top level only a reordering of its members:
// the order of the members of every object below stays as drawn unless the node
// is reversed throughout.
func (n *jsonNode) text(reversed, spaced bool) string {
	var b strings.Builder
	n.write(&b, reversed, spaced)
	return b.String()
}

// reorderedTop writes the members of the top object in the opposite order,
// leaving the objects below as they are.
func (n *jsonNode) reorderedTop(spaced bool) string {
	top := *n
	top.members = make([]jsonMember, len(n.members))
	for i, m := range n.members {
		top.members[len(n.members)-1-i] = m
	}
	return top.text(false, spaced)
}

// TestGeneratorNumbersDecode: a number the generator writes is one decodeDecimal
// reads. Those it refuses for their exponent are out of the plan's scope, and
// a pair holding one would test nothing the plan claims.
func TestGeneratorNumbersDecode(t *testing.T) {
	g := newObjectGenerator(1)
	for _, n := range generatorNumbers {
		if _, err := decodeDecimal(n); err != nil {
			t.Errorf("generator number %s does not decode: %v", n, err)
		}
	}
	// Every number written over a sweep of objects is drawn from that list
	for range 2000 {
		g.mutate(g.object(3))
	}
	if len(g.numbers) == 0 {
		t.Fatal("the generator wrote no number")
	}
	for n := range g.numbers {
		if _, err := decodeDecimal(n); err != nil {
			t.Errorf("written number %s does not decode: %v", n, err)
		}
	}
}

// generatedPairs draws pairs of objects: an object against a mutation of itself
// or against another, each written as drawn, in the opposite order of the top
// members, in the opposite order throughout, or laid out with spaces.
func generatedPairs(count int, seed int64) [][2]string {
	g := newObjectGenerator(seed)
	pairs := make([][2]string, 0, count)
	for len(pairs) < count {
		a := g.object(3)
		b := a
		switch g.rng.Intn(4) {
		case 0:
			b = g.object(3)
		default:
			b = g.mutate(a)
			if g.rng.Intn(3) == 0 {
				b = g.mutate(b)
			}
		}
		left := a.text(false, g.rng.Intn(4) == 0)
		var right string
		switch g.rng.Intn(4) {
		case 0:
			right = b.reorderedTop(false)
		case 1:
			right = b.text(true, false)
		case 2:
			right = b.text(false, true)
		default:
			right = b.text(false, false)
		}
		pairs = append(pairs, [2]string{left, right})
	}
	return pairs
}

// TestEqualJSONFastPathsAgreeWithTheReader (criterion 1): over generated pairs
// of objects, to depth 3, with every writing of a value, keys escaped and
// written twice in place, nulls, nested arrays and elements with content, the
// answer of equalJSON is the reader's with the shortcuts off at every depth.
func TestEqualJSONFastPathsAgreeWithTheReader(t *testing.T) {
	const want = 70000
	pairs := generatedPairs(want+2000, 24)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	var equal, disagree int
	for _, p := range pairs {
		a, b := []byte(p[0]), []byte(p[1])
		got, ref := equalJSON(a, b), equalJSONReader(a, b)
		if ref {
			equal++
		}
		if got != ref {
			disagree++
			if disagree <= 10 {
				t.Errorf("equalJSON(%s, %s) = %v, the reader says %v", p[0], p[1], got, ref)
			}
		}
		// And the other way round, which the reader answers alike
		if back := equalJSON(b, a); back != got {
			disagree++
			if disagree <= 10 {
				t.Errorf("equalJSON(%s, %s) = %v but the reverse is %v", p[0], p[1], got, back)
			}
		}
	}
	runtime.ReadMemStats(&after)
	t.Logf("%d pairs, %d equal, %d disagreements, %v, %d KiB allocated", len(pairs), equal, disagree,
		time.Since(start).Round(time.Millisecond), (after.TotalAlloc-before.TotalAlloc)/1024)
	if len(pairs) < want {
		t.Errorf("%d pairs, want at least %d", len(pairs), want)
	}
	if equal < 3000 {
		t.Errorf("only %d equal pairs: the sweep tests too little", equal)
	}
}

// transitivityWritings makes writings of a few objects, each written several
// ways that the reader reads alike. The objects hold the shapes of plan 024 §1.
func transitivityWritings(children []string) []string {
	var bases []string
	keys := []string{"a", "_a", "b"}
	for _, k := range keys {
		for _, v := range children {
			bases = append(bases, writeMembers([]string{k}, []string{v}))
		}
	}
	// Two members: the same key twice, in place, and two keys
	for _, k := range keys {
		for _, v := range children[:min(len(children), 8)] {
			for _, w := range children[:min(len(children), 8)] {
				bases = append(bases, `{"`+k+`":`+v+`,"`+k+`":`+w+`}`)
			}
		}
	}
	for i, k := range keys {
		for _, k2 := range keys[i+1:] {
			for _, v := range children[:min(len(children), 8)] {
				for _, w := range children[:min(len(children), 8)] {
					bases = append(bases, writeMembers([]string{k, k2}, []string{v, w}))
				}
			}
		}
	}
	rng := rand.New(rand.NewSource(24))
	rng.Shuffle(len(bases), func(i, j int) { bases[i], bases[j] = bases[j], bases[i] })
	bases = bases[:min(len(bases), 150)]

	out := make([]string, 0, 3*len(bases)+22)
	for _, base := range bases {
		// As written, laid out, and nested in an object: the shapes one level down
		out = append(out, base, strings.ReplaceAll(strings.ReplaceAll(base, `":`, `" : `), `,"`, `, "`), `{"n":`+base+`}`)
	}
	// The shapes of §1, whose writings vary: numbers in a nested array, a key
	// written twice, and an element with content beside a value
	out = append(out,
		`{"a":[[1]],"b":0}`, `{"a":[[1.0]],"b":0}`, `{"b":0,"a":[[1.0]]}`, `{"b":0,"a":[[1e0]]}`,
		`{"x":"a","x":"b"}`, `{"x":"a","x":"c"}`, `{"x":"a"}`, `{"x":"a","x":null}`,
		`{"v":"a","_v":{"resourceType":"X"}}`, `{"v":"a","_v":{"resourceType":"Y"}}`, `{"v":"a"}`, `{"_v":{"resourceType":"X"},"v":"a"}`,
		`{"n":{"x":"a","x":"b"}}`, `{"n":{"x":"a","x":"c"}}`, `{"n":{"x":"a"}}`,
		`{"n":{"v":"a","_v":{"resourceType":"X"}}}`, `{"n":{"v":"a","_v":{"resourceType":"Y"}}}`, `{"n":{"v":"a"}}`,
		`{"n":{"a":[[1]],"b":0}}`, `{"n":{"b":0,"a":[[1.0]]}}`, `{"n":[{"a":[[1]],"b":0}]}`, `{"n":[{"b":0,"a":[[1.0]]}]}`,
	)
	return out
}

// transitiveChildren keeps the child values whose own comparisons are symmetric
// and transitive, by = and by ~: an object takes its children's senses, which
// quantities and calendar units give a sense of their own, and which cases of
// their own fix (plan 023). It leaves those out, and says which.
func transitiveChildren(t *testing.T) []string {
	t.Helper()
	candidates := []string{
		`"x"`, `{}`, `{"id":"e"}`, `[{}]`, `null`, `[{"id":"e"},null]`, `["x"]`, `[null,"x"]`, `[]`,
		`{"resourceType":"X"}`, `1`, `1.0`, `[[1]]`, `[[1.0]]`, `[1,[2]]`, `[[null]]`,
		`{"value":1,"code":"1"}`, `{"value":1.5,"code":"Cel"}`, `{"value":274.6,"code":"K"}`, `{"value":1,"unit":"year"}`, `{"value":12,"unit":"month"}`,
	}
	excluded := map[string]bool{
		`{"value":1,"code":"1"}`: true, `{"value":1.5,"code":"Cel"}`: true, `{"value":274.6,"code":"K"}`: true,
		`{"value":1,"unit":"year"}`: true, `{"value":12,"unit":"month"}`: true,
	}
	var kept []string
	for _, v := range candidates {
		if excluded[v] {
			t.Logf("child %s left out: a quantity or a calendar unit, whose comparison has a sense of its own", v)
			continue
		}
		ok := true
		for _, w := range candidates {
			if excluded[w] {
				continue
			}
			a, b := object(`{"k":`+v+`}`), object(`{"k":`+w+`}`)
			if a.Equal(b) != b.Equal(a) {
				ok = false
				t.Logf("child %s left out: = against %s is not symmetric", v, w)
			}
			for _, u := range candidates {
				if excluded[u] {
					continue
				}
				c := object(`{"k":` + u + `}`)
				if a.Equal(b) && b.Equal(c) && !a.Equal(c) {
					ok = false
					t.Logf("child %s left out: = is not transitive over %s and %s", v, w, u)
				}
			}
		}
		if ok {
			kept = append(kept, v)
		}
	}
	t.Logf("%d of %d child values kept", len(kept), len(candidates))
	return kept
}

// TestObjectEqualityIsTransitive (criterion 2): over triples drawn from the
// writings, a = b and b = c imply a = c.
func TestObjectEqualityIsTransitive(t *testing.T) {
	writings := transitivityWritings(transitiveChildren(t))
	n := len(writings)
	objects := make([]*ObjectValue, n)
	for i, w := range writings {
		objects[i] = object(w)
	}
	eq := make([][]bool, n)
	for i := range eq {
		eq[i] = make([]bool, n)
	}
	for i := range n {
		for j := range n {
			eq[i][j] = objects[i].Equal(objects[j])
		}
	}
	var chains, violations, symmetric int
	for i := range n {
		for j := range n {
			if eq[i][j] != eq[j][i] {
				symmetric++
				if symmetric <= 10 {
					t.Errorf("%s = %s is %v, the reverse %v", writings[i], writings[j], eq[i][j], eq[j][i])
				}
			}
			if !eq[i][j] {
				continue
			}
			for k := range n {
				if !eq[j][k] {
					continue
				}
				if i != j && j != k && i != k && writings[i] != writings[k] {
					chains++
				}
				if !eq[i][k] {
					violations++
					if violations <= 10 {
						t.Errorf("%s = %s and %s = %s, but %s = %s is false", writings[i], writings[j], writings[j], writings[k], writings[i], writings[k])
					}
				}
			}
		}
	}
	t.Logf("%d writings, %d triples a=b, b=c over three different texts, %d intransitive, %d asymmetric pairs", n, chains, violations, symmetric)
	if violations > 0 || symmetric > 0 {
		t.Errorf("%d intransitive triples, %d asymmetric pairs", violations, symmetric)
	}
	if chains < 5000 {
		t.Errorf("only %d chains: the sweep tests too little", chains)
	}
}

// transitivityCase is a pair of objects and what = and ~ answer, where the plan
// states it.
type transitivityCase struct {
	name, a, b string
	eq         bool
	equiv      *bool // nil where the plan states no answer for ~
	// noReverse keeps the order of the members, where it carries a meaning:
	// of a key written twice, the first that is not null counts.
	noReverse bool
}

func yes() *bool { v := true; return &v }
func no() *bool  { v := false; return &v }

// transitivityCases takes its expected values from the plan's §2 and exit
// criterion 3.
func transitivityCases() []transitivityCase {
	return []transitivityCase{
		// A nested array's numbers compare as decimals, in either key order
		{"nested array, one order", `{"a":[[1]],"b":0}`, `{"a":[[1.0]],"b":0}`, true, nil, false},
		{"nested array, keys reordered", `{"a":[[1]],"b":0}`, `{"b":0,"a":[[1.0]]}`, true, nil, false},
		{"nested array, both written 1.0", `{"a":[[1.0]],"b":0}`, `{"b":0,"a":[[1.0]]}`, true, nil, false},
		{"nested array, alone", `{"a":[[1]]}`, `{"a":[[1.0]]}`, true, yes(), false},
		{"nested array is not flattened", `{"a":[[1]]}`, `{"a":[1]}`, false, nil, false},
		{"nested array keeps positions", `{"a":[[1],[2]]}`, `{"a":[[2],[1]]}`, false, nil, false},
		{"nested arrays of another depth", `{"a":[[[1]]]}`, `{"a":[[1]]}`, false, nil, false},
		{"a Quantity inside a nested array", `{"a":[[{"value":1,"code":"1"}]]}`, `{"a":[[{"value":1.0,"code":"1"}]]}`, true, yes(), false},
		{"a Quantity inside a nested array, another value", `{"a":[[{"value":1,"code":"1"}]]}`, `{"a":[[{"value":2,"code":"1"}]]}`, false, no(), false},
		{"a null in a nested array is compared as written", `{"a":[[null]]}`, `{"a":[[]]}`, false, nil, false},
		{"nulls in nested arrays", `{"a":[[null,1]]}`, `{"a":[[null,1.0]]}`, true, nil, false},
		// Of a key written twice, the first that is not null counts
		{"a key twice, the second differs", `{"a":"x","a":"y"}`, `{"a":"x","a":"z"}`, true, nil, true},
		{"a key twice against once", `{"a":"x","a":"y"}`, `{"a":"x"}`, true, nil, true},
		{"a key twice, the first differs", `{"a":"x","a":"z"}`, `{"a":"y","a":"z"}`, false, nil, true},
		{"a key twice after a null, the second differs", `{"a":null,"a":"x"}`, `{"a":null,"a":"y"}`, false, nil, true},
		{"a key twice after a null, the same", `{"a":null,"a":"x"}`, `{"a":null,"a":"x"}`, true, nil, true},
		{"a key twice after a null against once", `{"a":null,"a":"x"}`, `{"a":"x"}`, true, nil, true},
		{"a key twice, a difference before it", `{"n":1,"a":"x","a":"y"}`, `{"n":2,"a":"x","a":"y"}`, false, nil, true},
		// An element that holds no value adds nothing beside one
		{"an element with content beside a value", `{"a":"x","_a":{"resourceType":"X"}}`, `{"a":"x","_a":{"resourceType":"Y"}}`, true, nil, false},
		{"an element with an id beside a value", `{"a":"x","_a":{"id":"1"}}`, `{"a":"x","_a":{"id":"2"}}`, false, nil, false},
		// Plain text, which the shortcuts decide
		{"a number written another way", `{"n":1,"m":"x"}`, `{"n":1.0,"m":"x"}`, true, nil, false},
		{"a number that differs", `{"n":1,"m":"x"}`, `{"n":2.0,"m":"x"}`, false, nil, false},
		{"a string that differs", `{"n":1,"m":"x"}`, `{"n":1,"m":"y"}`, false, nil, false},
		{"layout", `{"n":1,"m":"x"}`, "{ \"n\" : 1 ,\n \"m\" : \"x\" }", true, nil, false},
		// Keys and members that read alike
		{"a key escaped", `{"\u0061":"x"}`, `{"a":"x"}`, true, nil, false},
		{"a key escaped, an underscore", `{"\u005fa":{"id":"1"}}`, `{"_a":{"id":"1"}}`, true, nil, false},
		{"an escaped underscore beside a value", `{"a":"x","\u005fa":{"resourceType":"X"}}`, `{"a":"x","_a":{"resourceType":"Y"}}`, true, nil, false},
		{"an object against nothing", `{"a":{}}`, `{}`, false, nil, false},
		{"an array against nothing", `{"a":[]}`, `{}`, true, nil, false},
		{"a null against nothing", `{"a":null}`, `{}`, true, nil, false},
		{"an element array longer than its value array", `{"a":["x"],"_a":[{"id":"1"},{"id":"2"}]}`, `{"a":["x"],"_a":[{"id":"1"}]}`, false, nil, false},
		{"an element array longer, against itself", `{"a":["x"],"_a":[{"id":"1"},{"id":"2"}]}`, `{"_a":[{"id":"1"},{"id":"2"}],"a":["x"]}`, true, nil, false},
		{"a null inside an array", `{"a":[null,"x"]}`, `{"a":["x"]}`, true, nil, false},
		{"a null after the item", `{"a":["x",null]}`, `{"a":["x"]}`, true, nil, false},
	}
}

// wrapped puts a pair one level down: in an object, and in an array of objects.
func wrapped(a, b string) [][2]string {
	return [][2]string{{a, b}, {`{"n":` + a + `}`, `{"n":` + b + `}`}, {`{"n":[` + a + `]}`, `{"n":[` + b + `]}`}, {`{"m":0,"n":` + a + `}`, `{"n":` + b + `,"m":0}`}}
}

func TestObjectEqualityOfTransitivityCases(t *testing.T) {
	for _, tc := range transitivityCases() {
		t.Run(tc.name, func(t *testing.T) {
			forms := wrapped(tc.a, tc.b)
			if !tc.noReverse {
				forms = append(forms, [2]string{reversedMembers(tc.a), tc.b}, [2]string{tc.a, reversedMembers(tc.b)})
			}
			for _, f := range forms {
				l, r := object(f[0]), object(f[1])
				if got := l.Equal(r); got != tc.eq {
					t.Errorf("%s = %s is %v, want %v", f[0], f[1], got, tc.eq)
				}
				if got := r.Equal(l); got != tc.eq {
					t.Errorf("%s = %s is %v, want %v", f[1], f[0], got, tc.eq)
				}
				// The reader says the same, with every shortcut off
				if got := equalJSONReader([]byte(f[0]), []byte(f[1])); got != tc.eq {
					t.Errorf("the reader: %s = %s is %v, want %v", f[0], f[1], got, tc.eq)
				}
				if tc.equiv != nil {
					if got := l.Equivalent(r); got != *tc.equiv {
						t.Errorf("%s ~ %s is %v, want %v", f[0], f[1], got, *tc.equiv)
					}
					if got := r.Equivalent(l); got != *tc.equiv {
						t.Errorf("%s ~ %s is %v, want %v", f[1], f[0], got, *tc.equiv)
					}
				}
			}
		})
	}
}

// TestNestedArrayEquivalence: [[1]] ~ [[1.0]] is true (plan 024 §2, 1), as
// fhirpath.js answers.
func TestNestedArrayEquivalence(t *testing.T) {
	a, b := object(`{"a":[[1]]}`), object(`{"a":[[1.0]]}`)
	if !a.Equivalent(b) || !b.Equivalent(a) {
		t.Errorf("{a:[[1]]} ~ {a:[[1.0]]} is %v and the reverse %v, want true", a.Equivalent(b), b.Equivalent(a))
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("{a:[[1]]} = {a:[[1.0]]} is %v and the reverse %v, want true", a.Equal(b), b.Equal(a))
	}
}

// TestNullBesideAKeyTwice (§6): un null suelto no obliga a leer por hijo, solo
// lo hace junto a una clave escrita dos veces. Los valores esperados son los de
// la lectura con el interruptor apagado, que es la referencia: el atajo negativo
// deja sin decidir un null frente a otro valor, y el positivo solo habla de
// textos iguales.
func TestNullBesideAKeyTwice(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{`{"a":[null]}`, `{"a":[]}`, true},
		{`{"a":null,"b":1}`, `{"a":2,"b":1}`, false},
		{`{"a":null,"b":1}`, `{"b":1}`, true},
		{`{"x":1,"a":null,"b":1}`, `{"x":1,"a":2,"b":1}`, false},
	} {
		a, b := []byte(tc.a), []byte(tc.b)
		if got := equalJSONReader(a, b); got != tc.want {
			t.Errorf("la lectura de %s = %s da %v, se esperaba %v", tc.a, tc.b, got, tc.want)
		}
		if got := equalJSON(a, b); got != tc.want {
			t.Errorf("%s = %s da %v, se esperaba %v", tc.a, tc.b, got, tc.want)
		}
		if got := equalJSON(b, a); got != tc.want {
			t.Errorf("%s = %s da %v, se esperaba %v", tc.b, tc.a, got, tc.want)
		}
	}
}

// TestSwitchReachesEveryShortcut (§2, 3): with the shortcuts off, none is
// entered at any depth. The pairs are plain, so that the gate would let their
// shortcuts decide, and each reaches one: a plain object nested in a plain
// object, an array of objects, and the path of writtenAlike.
func TestSwitchReachesEveryShortcut(t *testing.T) {
	pairs := [][2]string{
		{`{"n":{"m":{"k":1,"j":"x"}}}`, `{"n":{"m":{"k":1.0,"j":"x"}}}`},
		{`{"n":{"m":{"k":1,"j":"x"}}}`, `{"n":{"m":{"k":1,"j":"y"}}}`},
		{`{"n":[{"k":1},{"k":2}]}`, `{"n":[{"k":1},{"k":3}]}`},
		{`{"n":[{"k":1},{"k":2}]}`, `{"n":[{"k":1},{"k":2}]}`},
		{`{"n":{"k":1},"m":[1,2]}`, `{"m":[1,2],"n":{"k":1}}`},
		{`{"a":[1,2],"b":{"k":"x"},"c":"s"}`, `{"a":[1,2],"b":{"k":"x"},"c":"s"}`},
		{`{"a":[[1]],"b":[{"k":1}]}`, `{"a":[[1]],"b":[{"k":1.0}]}`},
	}
	// The counter is live: a shortcut entered with the switch off is counted
	shortcuts = false
	shortcutEntries = 0
	sameJSON([]byte(`{"k":1}`), []byte(`{"k":1}`))
	live := shortcutEntries
	orderedEqualFromDifference([]byte(`{"k":1}`), []byte(`{"k":2}`))
	live2 := shortcutEntries
	shortcutEntries = 0
	for _, p := range pairs {
		a, b := []byte(p[0]), []byte(p[1])
		equalJSON(a, b)
		equalJSON(b, a)
		// The path of writtenAlike: the children of a plain object, one by one
		l, lok := parseJSONObject(a)
		r, rok := parseJSONObject(b)
		if lok && rok {
			for _, c := range l.children {
				if other, found := r.find(c.key, 0); found {
					writtenAlike(c.data, c.dataType, other.data, other.dataType)
					equalChildren(c, other)
				}
			}
		}
	}
	entered := shortcutEntries
	shortcuts = true
	if live != 1 || live2 != 2 {
		t.Fatalf("the counter reads %d and %d after one entry to each shortcut, want 1 and 2", live, live2)
	}
	if entered != 0 {
		t.Errorf("%d shortcut entries with the switch off, want 0", entered)
	}
}

// plainJSON informa lo que el escáner de la puerta encuentra en un texto: si
// P1 a P3 se cumplen en todos los niveles y los corchetes cierran. El escáner
// no comprueba la sintaxis de JSON, solo P1 a P3, así que no dice si el texto
// es JSON. No reserva memoria en el caso común.
func plainJSON(data []byte) bool {
	var keyBuf [plainStackKeys]keyRange
	var markBuf [plainStackMarks]int32
	s, stop, ok := plainScan{keyBuf[:0], markBuf[:0]}.scan(data, 0, len(data))
	return ok && stop == len(data) && len(s.marks) == 0
}

// TestTextThatIsNotJSONIsOutsideTheClaim fija las respuestas de hoy para un
// texto que no es JSON: una clave con un escape inválido (`\q`). Los atajos
// positivos no se filtran, así que el atajo lo da por igual a sí mismo y a
// `{"\q":1.0}`; el lector (jsonparser) rechaza la clave y responde falso. Este
// caso queda fuera de la garantía, que cubre solo texto JSON (RFC 8259) que
// jsonparser lee: un texto que no es JSON no tiene lectura definida, igual que
// los decimales fuera de alcance.
func TestTextThatIsNotJSONIsOutsideTheClaim(t *testing.T) {
	for _, tc := range []struct{ name, a, b string }{
		{"the same text", `{"\q":1}`, `{"\q":1}`},
		{"a number written another way", `{"\q":1}`, `{"\q":1.0}`},
	} {
		if !equalJSON([]byte(tc.a), []byte(tc.b)) {
			t.Errorf("%s: the shortcuts answer false for %s and %s, want true", tc.name, tc.a, tc.b)
		}
		if equalJSONReader([]byte(tc.a), []byte(tc.b)) {
			t.Errorf("%s: the reader answers true for %s and %s, want false", tc.name, tc.a, tc.b)
		}
	}
}

// TestALoneSurrogateKeyIsOutsideTheClaim fija las respuestas de hoy para una
// clave con un escape de sustituto sin par (`\uDEAD`). RFC 8259 §8.2 lo permite,
// pero jsonparser v1.1.1 lo rechaza en una clave (JP-3 en
// docs/upstream-findings.md): el lector responde falso. Los atajos no
// decodifican las claves y lo dan por igual a sí mismo y a `{"\uDEAD":1.0}`.
// Es un defecto de la librería y no se rodea aquí: se documenta y se fija.
// Ningún nombre de elemento FHIR tiene una clave así. El mismo escape dentro
// de un valor se lee bien, y los dos caminos coinciden.
func TestALoneSurrogateKeyIsOutsideTheClaim(t *testing.T) {
	old := shortcuts
	defer func() { shortcuts = old }()

	for _, tc := range []struct{ name, a, b string }{
		{"the same text", `{"\uDEAD":1}`, `{"\uDEAD":1}`},
		{"a number written another way", `{"\uDEAD":1}`, `{"\uDEAD":1.0}`},
	} {
		if !equalJSON([]byte(tc.a), []byte(tc.b)) {
			t.Errorf("%s: the shortcuts answer false for %s and %s, want true", tc.name, tc.a, tc.b)
		}
		if equalJSONReader([]byte(tc.a), []byte(tc.b)) {
			t.Errorf("%s: the reader answers true for %s and %s, want false", tc.name, tc.a, tc.b)
		}
	}

	v := []byte(`{"a":"\uDEAD"}`)
	shortcuts = true
	on := equalJSON(v, v)
	shortcuts = false
	off := equalJSON(v, v)
	if on != off {
		t.Errorf("a lone surrogate in a value: shortcuts %v, reader %v, want the same", on, off)
	}
}

// TestPlainJSON (§2, 2): what the check reports plain and what it does not,
// each of P1 to P3 at the root and nested.
func TestPlainJSON(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		plain      bool
	}{
		{"an object", `{"a":1,"b":[1,2,{"c":"x"}]}`, true},
		{"layout", "{ \"a\" : 1 ,\n \"b\" : [ ] }", true},
		{"the same key in sibling objects", `{"a":{"k":1},"b":{"k":2},"c":[{"k":1},{"k":2}]}`, true},
		{"the same key in a parent and a child", `{"k":{"k":{"k":1}}}`, true},
		{"a string with an escaped quote and a colon", `{"a":"x\":y","b":"n:\\"}`, true},
		{"a string value that is called null", `{"a":"null","b":"a_b"}`, true},
		{"a key with an underscore inside", `{"a_b":1}`, true},
		{"an array of numbers", `[1,2.5,-3e2]`, true},
		{"a key starting with an underscore", `{"_a":{}}`, false},
		{"a key that is only an underscore", `{"_":1}`, false},
		{"a nested key starting with an underscore", `{"a":[{"b":{"_c":1}}]}`, false},
		{"a key with an escape", `{"\u0061":1}`, false},
		{"a nested key with an escape", `{"a":{"b\n":1}}`, false},
		{"a key written twice", `{"a":1,"a":2}`, false},
		{"a key written twice, nested two deep", `{"x":{"y":{"a":1,"b":2,"a":3}}}`, false},
		{"a key written twice with others between", `{"a":1,"b":{"a":1},"c":2,"a":3}`, false},
		{"a null", `{"a":null}`, false},
		{"a null nested in an array", `{"a":[1,[null]]}`, false},
		{"unbalanced", `{"a":{"b":1}`, false},
		{"closed too soon", `{"a":1}}`, false},
		{"an unterminated string", `{"a":"x`, false},
		{"a key outside an object", `"a":1`, false},
	} {
		if got := plainJSON([]byte(tc.text)); got != tc.plain {
			t.Errorf("%s: plainJSON(%s) = %v, want %v", tc.name, tc.text, got, tc.plain)
		}
	}
	// An object of more keys than the check compares in pairs is not plain
	var many strings.Builder
	many.WriteByte('{')
	for i := range maxPlainKeys + 1 {
		if i > 0 {
			many.WriteByte(',')
		}
		fmt.Fprintf(&many, `"k%d":1`, i)
	}
	many.WriteByte('}')
	if plainJSON([]byte(many.String())) {
		t.Errorf("an object of %d keys is plain", maxPlainKeys+1)
	}
	// It allocates nothing on the texts of the benchmarks
	text := []byte(longPatient("one"))
	if !plainJSON(text) {
		t.Fatal("the long Patient is not plain")
	}
	if allocs := testing.AllocsPerRun(100, func() { plainJSON(text) }); allocs != 0 {
		t.Errorf("plainJSON allocates %v times on a Patient", allocs)
	}
}
