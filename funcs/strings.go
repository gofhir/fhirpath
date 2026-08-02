package funcs

import (
	"strings"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	// Register string functions
	Register(FuncDef{
		Name:    "startsWith",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnStartsWith,
	})

	Register(FuncDef{
		Name:    "endsWith",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnEndsWith,
	})

	Register(FuncDef{
		Name:    "contains",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnContains,
	})

	Register(FuncDef{
		Name:    "replace",
		MinArgs: 2,
		MaxArgs: 2,
		Fn:      fnReplace,
	})

	Register(FuncDef{
		Name:    "matches",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnMatches,
	})

	Register(FuncDef{
		Name:    "lastIndexOf",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnLastIndexOf,
	})

	Register(FuncDef{
		Name:    "matchesFull",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnMatchesFull,
	})

	Register(FuncDef{
		Name:    "replaceMatches",
		MinArgs: 2,
		MaxArgs: 2,
		Fn:      fnReplaceMatches,
	})

	Register(FuncDef{
		Name:    "indexOf",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnIndexOf,
	})

	Register(FuncDef{
		Name:    "substring",
		MinArgs: 1,
		MaxArgs: 2,
		Fn:      fnSubstring,
	})

	Register(FuncDef{
		Name:    "lower",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnLower,
	})

	Register(FuncDef{
		Name:    "upper",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnUpper,
	})

	Register(FuncDef{
		Name:    "toChars",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnToChars,
	})

	Register(FuncDef{
		Name:    "split",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnSplit,
	})

	Register(FuncDef{
		Name:    "join",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnJoin,
	})

	Register(FuncDef{
		Name:    "trim",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnTrim,
	})

	Register(FuncDef{
		Name:    "length",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnLength,
	})
}

// fnStartsWith returns true if the string starts with the given prefix.
func fnStartsWith(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	prefix, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewBoolean(strings.HasPrefix(str, prefix))}, nil
}

// fnEndsWith returns true if the string ends with the given suffix.
func fnEndsWith(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	suffix, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewBoolean(strings.HasSuffix(str, suffix))}, nil
}

// fnContains returns true if the string contains the given substring.
func fnContains(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	substr, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewBoolean(strings.Contains(str, substr))}, nil
}

// fnReplace replaces all occurrences of pattern with substitution.
func fnReplace(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	pattern, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	substitution, ok := toStringArg(args[1])
	if !ok {
		return types.Collection{}, nil
	}

	result := strings.ReplaceAll(str, pattern, substitution)
	return types.Collection{types.NewString(result)}, nil
}

// fnMatches returns true if the string matches the regex pattern.
// Uses cached regex compilation with ReDoS protection.
func fnMatches(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	pattern, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	// Use regex cache with timeout protection
	matched, err := DefaultRegexCache.MatchWithTimeout(ctx.Context(), pattern, str)
	if err != nil {
		return nil, err
	}

	return types.Collection{types.NewBoolean(matched)}, nil
}

// fnMatchesFull returns true when the pattern matches the entire input string,
// as opposed to matches(), which succeeds on any substring match.
//
// The pattern is anchored to the whole input, so a pattern that only describes
// part of it fails even if it is present: 'a/Library/b'.matchesFull('Library')
// is false, while 'a/Library/b'.matchesFull('.*Library.*') is true.
func fnMatchesFull(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	pattern, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	// \A and \z anchor to the whole text regardless of any line anchors inside
	// the pattern, so an already-anchored pattern keeps its own meaning.
	matched, err := DefaultRegexCache.MatchWithTimeout(ctx.Context(), `\A(?:`+pattern+`)\z`, str)
	if err != nil {
		return nil, err
	}

	return types.Collection{types.NewBoolean(matched)}, nil
}

// fnReplaceMatches replaces regex matches with substitution.
// Uses cached regex compilation with ReDoS protection.
func fnReplaceMatches(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	pattern, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	substitution, ok := toStringArg(args[1])
	if !ok {
		return types.Collection{}, nil
	}

	// Use regex cache with timeout protection
	result, err := DefaultRegexCache.ReplaceWithTimeout(ctx.Context(), pattern, str, substitution)
	if err != nil {
		return nil, err
	}

	return types.Collection{types.NewString(result)}, nil
}

// fnIndexOf returns the index of the first occurrence of substring.
func fnIndexOf(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	substr, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	idx := strings.Index(str, substr)
	return types.Collection{types.NewInteger(int64(idx))}, nil
}

// fnSubstring returns a substring starting at the given index.
func fnSubstring(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	// "If the input or start is empty, the result is empty" — an absent start is
	// not a malformed call, so it is answered rather than refused
	if col, isCollection := args[0].(types.Collection); isCollection && col.Empty() {
		return types.Collection{}, nil
	}

	start, err := toInteger(args[0])
	if err != nil {
		return nil, err
	}

	if start < 0 || int(start) >= len(str) {
		return types.Collection{}, nil
	}

	// Optional length parameter
	if len(args) > 1 {
		length, err := toInteger(args[1])
		if err != nil {
			return nil, err
		}
		end := int(start + length)
		if end > len(str) {
			end = len(str)
		}
		return types.Collection{types.NewString(str[start:end])}, nil
	}

	return types.Collection{types.NewString(str[start:])}, nil
}

// fnLower converts string to lowercase.
func fnLower(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewString(strings.ToLower(str))}, nil
}

// fnUpper converts string to uppercase.
func fnUpper(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewString(strings.ToUpper(str))}, nil
}

// fnToChars converts string to a collection of single characters.
func fnToChars(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	result := types.Collection{}
	for _, ch := range str {
		result = append(result, types.NewString(string(ch)))
	}

	return result, nil
}

// fnSplit splits a string by the given separator.
func fnSplit(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	separator, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	parts := strings.Split(str, separator)
	result := types.Collection{}
	for _, part := range parts {
		result = append(result, types.NewString(part))
	}

	return result, nil
}

// fnJoin joins a collection of strings with an optional separator.
func fnJoin(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewString("")}, nil
	}

	separator := ""
	if len(args) > 0 {
		if sep, ok := toStringArg(args[0]); ok {
			separator = sep
		}
	}

	parts := make([]string, 0, len(input))
	for _, item := range input {
		if s, ok := item.(types.String); ok {
			parts = append(parts, s.Value())
		} else {
			parts = append(parts, item.String())
		}
	}

	return types.Collection{types.NewString(strings.Join(parts, separator))}, nil
}

// fnTrim removes leading and trailing whitespace.
func fnTrim(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewString(strings.TrimSpace(str))}, nil
}

// fnLength returns the length of the string.
func fnLength(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewInteger(int64(len(str)))}, nil
}

// Helper functions

// toString reads the String a function's input must be, under the rule the
// specification states for passing a collection where a single item is
// expected:
//
//	IF the collection contains a single node AND the node's value can be
//	implicitly converted to the expected input type THEN the collection
//	evaluates to the value of that single node
//	ELSE IF the collection is empty THEN ... an empty collection
//	ELSE the evaluation will end and signal an error
//
// So a collection of two strings is an error rather than a guess at which one
// was meant, and a node that is not a String — a whole Identifier, say — is an
// error rather than its JSON rendered as text. Nothing converts implicitly to
// String: an Integer is not one.
//
// present is false with no error when the input is empty, which callers turn
// into an empty result.
func toString(col types.Collection) (value string, present bool, err error) {
	if col.Empty() {
		return "", false, nil
	}
	if len(col) > 1 {
		return "", false, eval.NewEvalError(eval.ErrSingletonExpected,
			"expected a single String, got %d items", len(col))
	}

	s, ok := col[0].(types.String)
	if !ok {
		return "", false, eval.NewEvalError(eval.ErrType,
			"expected a String, got %s", col[0].Type())
	}
	return s.Value(), true, nil
}

// toStringArg extracts a string from an argument.
func toStringArg(arg interface{}) (string, bool) {
	switch v := arg.(type) {
	case types.Collection:
		value, ok, err := toString(v)
		if err != nil {
			return "", false
		}
		return value, ok
	case types.String:
		return v.Value(), true
	case string:
		return v, true
	default:
		return "", false
	}
}

// fnLastIndexOf returns the 0-based index of the last occurrence of a substring,
// or -1 when it does not occur.
//
// Per the FHIRPath 3.0.0 specification, the index counts characters — Unicode
// scalar values, not bytes — and an empty substring returns the length of the
// input.
func fnLastIndexOf(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	str, ok, err := toString(input)
	if err != nil {
		return nil, err
	}
	if !ok {
		return types.Collection{}, nil
	}

	substring, ok := toStringArg(args[0])
	if !ok {
		return types.Collection{}, nil
	}

	runes := []rune(str)
	if substring == "" {
		return types.Collection{types.NewInteger(int64(len(runes)))}, nil
	}

	byteIndex := strings.LastIndex(str, substring)
	if byteIndex < 0 {
		return types.Collection{types.NewInteger(-1)}, nil
	}

	// Convert the byte offset to a character offset
	return types.Collection{types.NewInteger(int64(len([]rune(str[:byteIndex]))))}, nil
}
