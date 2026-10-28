package conformance

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/models/r4/v2"
)

// Every function and every operator, given a decimal beyond the bound as its
// input or as an argument, answers without writing the value out (#232). The
// value is 1e5000000: held, it is a coefficient and an exponent; rescaled, five
// million digits, which an operation that writes it out allocates by the
// megabyte. A call that allocates more than sweepAllocBound fails, whatever it
// answers; one that answers wrongly fails too.
//
// Each case is checked against its class:
//   - same: answers as it does with 1.5 in the value's place, item for item by
//     Go type and FHIR type, or errs as it does (what does not read the value);
//   - or the answer, written as typedItems renders it.

const sweepAllocBound = 1 << 20

var sweepBig = `{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":1e5000000}]}`
var sweepControl = `{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":1.5}]}`

const same = "same"

// sweepCase is an expression, V standing for the decimal, and its class.
type sweepCase struct{ expr, want string }

const (
	sweepTrue  = "[Boolean true]"
	sweepFalse = "[Boolean false]"
	sweepEmpty = "[]"
)

// sweepFunctions names, for each function, the calls that put the decimal in
// it: as its input, as an argument, inside its criteria.
var sweepFunctions = map[string][]sweepCase{
	"abs":                {{"V.abs()", sweepEmpty}},
	"aggregate":          {{"V.aggregate($total + $this, 0)", sweepEmpty}, {"(V | 1).aggregate(iif($total.empty(), $this, $total), {}).exists()", sweepTrue}},
	"all":                {{"V.all($this > 0)", sweepTrue}},
	"allFalse":           {{"V.allFalse()", same}},
	"allTrue":            {{"V.allTrue()", same}},
	"anyFalse":           {{"V.anyFalse()", same}},
	"anyTrue":            {{"V.anyTrue()", same}},
	"as":                 {{"V.as(FHIR.decimal).exists()", same}},
	"avg":                {{"V.avg()", sweepEmpty}, {"(V | 1).avg()", sweepEmpty}},
	"ceiling":            {{"V.ceiling()", sweepEmpty}},
	"checkModifiers":     {{"V.checkModifiers('x')", same}},
	"children":           {{"V.children().count()", same}},
	"coalesce":           {{"coalesce(V, 1) > 1", sweepTrue}},
	"combine":            {{"V.combine(V).count()", "[Integer 2]"}},
	"comparable":         {{"V.comparable(1 'g')", same}},
	"conformsTo":         {{"V.conformsTo('http://hl7.org/fhir/StructureDefinition/decimal')", same}},
	"contains":           {{"V.contains('1')", same}},
	"convertsToBoolean":  {{"V.convertsToBoolean()", same}},
	"convertsToDate":     {{"V.convertsToDate()", same}},
	"convertsToDateTime": {{"V.convertsToDateTime()", same}},
	"convertsToDecimal":  {{"V.convertsToDecimal()", sweepTrue}, {"V.toString().convertsToDecimal()", sweepFalse}},
	"convertsToInteger":  {{"V.convertsToInteger()", sweepFalse}},
	"convertsToLong":     {{"V.convertsToLong()", sweepFalse}},
	"convertsToQuantity": {{"V.convertsToQuantity()", sweepFalse}},
	"convertsToString":   {{"V.convertsToString()", sweepTrue}},
	"convertsToTime":     {{"V.convertsToTime()", same}},
	"count":              {{"V.count()", same}},
	"dateOf":             {{"V.dateOf()", same}},
	"day":                {{"V.day()", same}},
	"dayOf":              {{"V.dayOf()", same}},
	"decode":             {{"V.decode('hex')", same}},
	"defineVariable":     {{"V.defineVariable('x').select(%x > 0)", sweepTrue}},
	"descendants":        {{"V.descendants().count()", same}},
	"difference":         {{"V.difference(@2020)", same}},
	"distinct":           {{"V.combine(V).distinct().count()", "[Integer 1]"}},
	"duration":           {{"V.duration(@2020)", same}},
	"elementDefinition":  {{"V.elementDefinition().exists()", same}},
	"empty":              {{"V.empty()", same}},
	"encode":             {{"V.encode('hex')", same}},
	"endsWith":           {{"V.endsWith('0')", same}},
	"escape":             {{"V.escape('json')", same}},
	"exclude":            {{"(V | 1).exclude(V).count()", "[Integer 1]"}},
	"exists":             {{"V.exists()", same}, {"V.exists($this > 0)", sweepTrue}},
	"exp":                {{"V.exp()", sweepEmpty}},
	"extension":          {{"V.extension('x').exists()", same}},
	"first":              {{"(V | 1).first() > 1", sweepTrue}},
	"floor":              {{"V.floor()", sweepEmpty}},
	"getExtensionValue":  {{"V.getExtensionValue('x').exists()", same}},
	"getReferenceKey":    {{"V.getReferenceKey()", same}},
	"getValue":           {{"V.getValue().toString()", "[String 1e5000000]"}},
	"hasExtension":       {{"V.hasExtension('x')", same}},
	"hasValue":           {{"V.hasValue()", same}},
	"highBoundary":       {{"V.highBoundary()", sweepEmpty}, {"V.highBoundary(2)", sweepEmpty}, {"(1.5).highBoundary(V)", same}},
	"hour":               {{"V.hour()", same}},
	"hourOf":             {{"V.hourOf()", same}},
	"htmlChecks":         {{"V.htmlChecks()", same}},
	"iif":                {{"iif(V > 0, 'pos', 'neg')", "[String pos]"}, {"iif(true, V, 1) > 1", sweepTrue}},
	"indexOf":            {{"V.indexOf('1')", same}},
	"intersect":          {{"(V | 1).intersect(V).count()", "[Integer 1]"}},
	"is":                 {{"V.is(FHIR.decimal)", same}},
	"isDistinct":         {{"V.combine(V).isDistinct()", sweepFalse}},
	"join":               {{"V.join(',')", same}},
	"last":               {{"(1 | V).last() > 1", sweepTrue}},
	"lastIndexOf":        {{"V.lastIndexOf('1')", same}},
	"length":             {{"V.length()", same}},
	"ln":                 {{"V.ln()", sweepEmpty}},
	"log":                {{"V.log(10)", sweepEmpty}, {"(10.0).log(V)", sweepEmpty}},
	"lowBoundary":        {{"V.lowBoundary()", sweepEmpty}, {"V.lowBoundary(2)", sweepEmpty}, {"(1.5).lowBoundary(V)", same}},
	"lower":              {{"V.lower()", same}},
	"matches":            {{"V.matches('1')", same}},
	"matchesFull":        {{"V.matchesFull('1')", same}},
	"max":                {{"(V | 2).max() > 2", sweepTrue}},
	"memberOf":           {{"V.memberOf('http://example.org/vs')", same}},
	"millisecond":        {{"V.millisecond()", same}},
	"millisecondOf":      {{"V.millisecondOf()", same}},
	"min":                {{"(V | 2).min()", "[Integer 2]"}},
	"minute":             {{"V.minute()", same}},
	"minuteOf":           {{"V.minuteOf()", same}},
	"month":              {{"V.month()", same}},
	"monthOf":            {{"V.monthOf()", same}},
	"not":                {{"V.not()", same}},
	"now":                {{"now().exists() and V > 0", sweepTrue}},
	"ofType":             {{"V.ofType(FHIR.decimal).count()", same}},
	"power":              {{"V.power(2)", sweepEmpty}, {"(2.0).power(V)", sweepEmpty}, {"V.power(0)", "[Decimal 1]"}},
	"precision":          {{"V.precision()", "[Integer 0]"}},
	"repeat":             {{"V.repeat($this).count()", same}},
	"repeatAll":          {{"V.repeatAll(children()).count()", same}},
	"replace":            {{"V.replace('1', '2')", same}},
	"replaceMatches":     {{"V.replaceMatches('1', '2')", same}},
	"resolve":            {{"V.resolve()", same}},
	"round":              {{"V.round()", sweepEmpty}, {"V.round(2)", sweepEmpty}, {"(1.5).round(V)", same}},
	"second":             {{"V.second()", same}},
	"secondOf":           {{"V.secondOf()", same}},
	"select":             {{"V.select($this > 0)", sweepTrue}},
	"single":             {{"V.single() > 0", sweepTrue}},
	"skip":               {{"(V | 1).skip(1).count()", "[Integer 1]"}, {"(1 | 2).skip(V)", same}},
	"slice":              {{"V.slice('x', 'y')", same}},
	"sort":               {{"(V | 3 | 2).sort().first()", "[Integer 2]"}, {"(V | 3 | 2).sort(-$this).first() > 3", same}},
	"split":              {{"V.split('e')", same}},
	"sqrt":               {{"V.sqrt()", sweepEmpty}},
	"startsWith":         {{"V.startsWith('1')", same}},
	"subsetOf":           {{"V.subsetOf(V | 1)", sweepTrue}},
	"substring":          {{"V.substring(0, 1)", same}, {"'abc'.substring(V)", same}},
	"subsumedBy":         {{"V.subsumedBy(V)", same}},
	"subsumes":           {{"V.subsumes(V)", same}},
	"sum":                {{"V.sum()", sweepEmpty}, {"(V | 1).sum()", sweepEmpty}},
	"supersetOf":         {{"(V | 1).supersetOf(V)", sweepTrue}},
	"tail":               {{"(V | 1).tail().count()", "[Integer 1]"}},
	"take":               {{"(V | 1).take(1) > 1", sweepTrue}, {"(1 | 2).take(V)", same}},
	"timeOf":             {{"V.timeOf()", same}},
	"timeOfDay":          {{"timeOfDay().exists() and V > 0", sweepTrue}},
	"timezoneOffsetOf":   {{"V.timezoneOffsetOf()", same}},
	"toBoolean":          {{"V.toBoolean()", sweepEmpty}},
	"toChars":            {{"V.toChars()", same}},
	"toDate":             {{"V.toDate()", same}},
	"toDateTime":         {{"V.toDateTime()", same}},
	"toDecimal":          {{"V.toDecimal() = V", sweepTrue}, {"V.toString().toDecimal()", sweepEmpty}},
	"toInteger":          {{"V.toInteger()", sweepEmpty}},
	"toLong":             {{"V.toLong()", sweepEmpty}},
	"toQuantity":         {{"V.toQuantity()", sweepEmpty}, {"V.toQuantity('g')", sweepEmpty}},
	"toString":           {{"V.toString()", "[String 1e5000000]"}},
	"toTime":             {{"V.toTime()", same}},
	"today":              {{"today().exists() and V > 0", sweepTrue}},
	"trace":              {{"V.trace('v') > 0", sweepTrue}},
	"trim":               {{"V.trim()", same}},
	"truncate":           {{"V.truncate()", sweepEmpty}},
	"type":               {{"V.type().name", same}},
	"unescape":           {{"V.unescape('json')", same}},
	"union":              {{"V.union(V).count()", "[Integer 1]"}},
	"upper":              {{"V.upper()", same}},
	"where":              {{"(V | 1).where($this > 1).count()", "[Integer 1]"}},
	"year":               {{"V.year()", same}},
	"yearOf":             {{"V.yearOf()", same}},
}

// sweepOperators names, for each operator token of the grammar's expression
// rule, the expressions that put the decimal beside it.
var sweepOperators = map[string][]sweepCase{
	".": {{"V.exists()", same}},
	"[": {{"(V | 1)[0] > 1", sweepTrue}},
	"]": {{"(1 | V)[1] > 1", sweepTrue}},
	// Against a quantity in unit '1' the decimal compares exactly, as against
	// a number, from either side; in another unit it is empty, as
	// TestADecimalBeyondTheBoundAgainstAQuantityInAnotherUnitIsUnknown has it
	"=":        {{"V = V", sweepTrue}, {"V = 1", sweepFalse}, {"V = 1 '1'", sweepFalse}, {"1 '1' = V", sweepFalse}},
	"!=":       {{"V != V", sweepFalse}, {"1 != V", sweepTrue}},
	"~":        {{"V ~ V", sweepTrue}, {"V ~ 1", sweepFalse}, {"V ~ 1 '1'", sweepFalse}, {"1 '1' ~ V", sweepFalse}},
	"!~":       {{"V !~ V", sweepFalse}, {"1 !~ V", sweepTrue}},
	"<":        {{"V < V", sweepFalse}, {"1 < V", sweepTrue}, {"V < 1 'g'", same}},
	"<=":       {{"V <= V", sweepTrue}, {"V <= 1", sweepFalse}},
	">":        {{"V > 1", sweepTrue}, {"1.5 > V", sweepFalse}},
	">=":       {{"V >= V", sweepTrue}, {"1 >= V", sweepFalse}},
	"+":        {{"V + 1", sweepEmpty}, {"1 + V", sweepEmpty}, {"+V > 1", sweepTrue}, {"V + 1 'g'", same}},
	"-":        {{"V - 1", sweepEmpty}, {"1 - V", sweepEmpty}, {"V - V", sweepEmpty}, {"-V", sweepEmpty}},
	"*":        {{"V * 2", sweepEmpty}, {"2 * V", sweepEmpty}, {"V * 0", sweepEmpty}, {"V * 1 'g'", sweepEmpty}},
	"/":        {{"V / 2", sweepEmpty}, {"2 / V", sweepEmpty}, {"V / V", sweepEmpty}, {"1 'g' / V", sweepEmpty}},
	"div":      {{"V div 2", sweepEmpty}, {"2 div V", sweepEmpty}},
	"mod":      {{"V mod 2", sweepEmpty}, {"2 mod V", sweepEmpty}},
	"&":        {{"V & ''", same}},
	"|":        {{"(V | V).count()", "[Integer 1]"}, {"(V | 1).count()", "[Integer 2]"}},
	"in":       {{"V in (1 | V)", sweepTrue}},
	"contains": {{"(1 | V) contains V", sweepTrue}},
	"and":      {{"(V > 0) and true", sweepTrue}},
	"or":       {{"(V < 0) or false", sweepFalse}},
	"xor":      {{"(V > 0) xor false", sweepTrue}},
	"implies":  {{"(V > 0) implies true", sweepTrue}},
	"is":       {{"V is FHIR.decimal", same}},
	"as":       {{"(V as FHIR.decimal).exists()", same}},
	"=>":       {{"V.exists()", same}},
}

// sweepKinds renders a result for the class same: each item's Go type and
// FHIR type, not its value.
func sweepKinds(result fhirpath.Collection, err error) string {
	if err != nil {
		return "error"
	}
	kinds := make([]string, len(result))
	for i, v := range result {
		kinds[i] = fmt.Sprintf("%T %s", v, v.Type())
	}
	return fmt.Sprint(kinds)
}

// sweepEvaluate evaluates expr over doc and reports the bytes it allocated.
func sweepEvaluate(expr, doc string) (result fhirpath.Collection, allocated uint64, err error) {
	compiled, err := fhirpath.Compile(expr)
	if err != nil {
		return nil, 0, err
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	result, err = compiled.EvaluateWithOptions([]byte(doc), fhirpath.WithModel(r4.FHIRPathModel()))
	runtime.ReadMemStats(&after)
	return result, after.TotalAlloc - before.TotalAlloc, err
}

// sweepPlaceholder is V standing alone, not inside a name such as getValue.
var sweepPlaceholder = regexp.MustCompile(`\bV\b`)

func sweep(t *testing.T, name string, cases []sweepCase) {
	t.Helper()
	for _, c := range cases {
		expr := sweepPlaceholder.ReplaceAllString(c.expr, "Patient.extension.value")
		// The control first: what a call sets up once, the UCUM tables among
		// it, is then not counted against the decimal
		control, _, controlErr := sweepEvaluate(expr, sweepControl)
		start := time.Now()
		result, allocated, err := sweepEvaluate(expr, sweepBig)
		took := time.Since(start)
		if allocated > sweepAllocBound {
			t.Errorf("%s: %s allocated %d bytes in %v with a decimal beyond the bound: it wrote the value out", name, c.expr, allocated, took)
		}
		if c.want == same {
			if got, want := sweepKinds(result, err), sweepKinds(control, controlErr); got != want {
				t.Errorf("%s: %s = %s (%v), want %s as with 1.5 (%v)", name, c.expr, got, err, want, controlErr)
			}
			continue
		}
		got := "error"
		if err == nil {
			got = typedItems(result)
		}
		if got != c.want {
			t.Errorf("%s: %s = %s (%v), want %s", name, c.expr, got, err, c.want)
		}
	}
}

func TestEveryFunctionAnswersWithoutWritingADecimalBeyondTheBoundOut(t *testing.T) {
	names := funcs.List()
	slices.Sort(names)
	for _, name := range names {
		cases, ok := sweepFunctions[name]
		if !ok {
			t.Errorf("function %s has no case in sweepFunctions: add one", name)
			continue
		}
		sweep(t, name, cases)
	}
	for name := range sweepFunctions {
		if !slices.Contains(names, name) {
			t.Errorf("sweepFunctions names %s, which is no registered function", name)
		}
	}
}

// grammarOperators reads the tokens of the grammar's expression rule.
func grammarOperators(t *testing.T) []string {
	t.Helper()
	grammar, err := os.ReadFile("../grammar/fhirpath.g4")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`(?ms)^expression\n(.*?)\n\s*;`).FindSubmatch(grammar)
	if rule == nil {
		t.Fatal("no expression rule in the grammar")
	}
	var tokens []string
	for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllSubmatch(rule[1], -1) {
		if token := string(m[1]); !slices.Contains(tokens, token) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func TestEveryOperatorAnswersWithoutWritingADecimalBeyondTheBoundOut(t *testing.T) {
	tokens := grammarOperators(t)
	for _, token := range tokens {
		cases, ok := sweepOperators[token]
		if !ok {
			t.Errorf("operator %s has no case in sweepOperators: add one", token)
			continue
		}
		sweep(t, token, cases)
	}
	for token := range sweepOperators {
		if !slices.Contains(tokens, token) {
			t.Errorf("sweepOperators names %s, which is no token of the grammar's expression rule", token)
		}
	}
}
