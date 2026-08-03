---
title: "String Functions"
linkTitle: "String Functions"
weight: 1
description: >
  Functions for manipulating and inspecting string values in FHIRPath expressions.
---

String functions operate on `String` values and provide common text manipulation capabilities. When invoked on an empty collection, all string functions return an empty collection. If the input is not a string type, the functions generally return an empty collection rather than raising an error.

---

## startsWith

Returns `true` if the input string starts with the given prefix.

**Signature:**

```text
startsWith(prefix : String) : Boolean
```

**Parameters:**

| Name       | Type     | Description              |
|------------|----------|--------------------------|
| `prefix`   | `String` | The prefix to check for  |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('Smi')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('Jon')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('')")
// Any string starts with empty string -> true
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty.
- An empty prefix (`''`) always returns `true` for any non-empty string.
- The comparison is case-sensitive.

---

## endsWith

Returns `true` if the input string ends with the given suffix.

**Signature:**

```text
endsWith(suffix : String) : Boolean
```

**Parameters:**

| Name       | Type     | Description              |
|------------|----------|--------------------------|
| `suffix`   | `String` | The suffix to check for  |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.endsWith('ith')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.endsWith('.pdf')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(resource, "Patient.id.endsWith('')")
// Any string ends with empty string -> true
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty.
- An empty suffix (`''`) always returns `true`.
- The comparison is case-sensitive.

---

## contains

Returns `true` if the input string contains the given substring.

**Signature:**

```text
contains(substring : String) : Boolean
```

**Parameters:**

| Name          | Type     | Description                  |
|---------------|----------|------------------------------|
| `substring`   | `String` | The substring to search for  |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('mit')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('xyz')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('')")
// Any string contains the empty string -> true
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty.
- An empty substring (`''`) always returns `true`.
- The search is case-sensitive.

---

## replace

Replaces all occurrences of a pattern string with a substitution string.

**Signature:**

```text
replace(pattern : String, substitution : String) : String
```

**Parameters:**

| Name             | Type     | Description                        |
|------------------|----------|------------------------------------|
| `pattern`        | `String` | The literal string to search for   |
| `substitution`   | `String` | The replacement string             |

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.replace('Smith', 'Jones')")
// "Smith" -> "Jones"

result, _ := fhirpath.Evaluate(resource, "'hello-world'.replace('-', '_')")
// "hello-world" -> "hello_world"

result, _ := fhirpath.Evaluate(resource, "'aaa'.replace('a', 'bb')")
// "aaa" -> "bbbbbb"
```

**Edge Cases / Notes:**

- Replaces **all** occurrences, not just the first.
- This is a literal string replacement, not a regex replacement. Use `replaceMatches` for regex.
- Returns empty collection if the input is empty.

---

## matches

Returns `true` if the input string matches the given regular expression.

**Signature:**

```text
matches(regex : String [, flags : String]) : Boolean
```

**Parameters:**

| Name      | Type     | Description                    |
|-----------|----------|--------------------------------|
| `regex`   | `String` | A regular expression pattern   |
| `flags`   | `String` | Optional. `i` for a case-insensitive search, `m` to make `^` and `$` match the start and end of each line |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'ABC'.matches('[A-Z]{3}')")
// true

result, _ := fhirpath.Evaluate(resource, "'abc123'.matches('[a-z]+\\d+')")
// true

result, _ := fhirpath.Evaluate(resource, "'hello'.matches('^[0-9]+$')")
// false

result, _ := fhirpath.Evaluate(resource, "'ABC'.matches('abc', 'i')")
// true
```

**The match is partial.** The pattern is looked for *anywhere* in the value, not
tested against the whole of it. This is what the specification calls for — "the
start/end of line markers `^`, `$` can be used to match the entire string" only
means something if the pattern is not anchored already — and it is the most
common source of surprise in this function:

```go
"'N8000123123'.matches('N[0-9]{8}')"      // true — an 8-digit sequence is in there
"'N8000123123'.matches('^N[0-9]{8}$')"    // false — anchored by hand
"'  X  '.matches('[A-Z][A-Za-z0-9_]*')"   // true — the X alone satisfies it
```

Use [`matchesFull`](#matchesfull) when the pattern has to cover the whole value,
or anchor the pattern yourself with `^` and `$`.

This matters when reading FHIR's published invariants, most of which are
unanchored: `eld-19`, `eld-20` and the `[A-Z]([A-Za-z0-9_]){0,254}` constraint
shared by many canonical resources all admit a value that merely *contains*
something acceptable. That is a property of those constraints rather than of this
engine — HL7's Java validator anchors because `String.matches` in Java does, not
because the specification says to.

**Edge Cases / Notes:**

- Uses Go's `regexp` package (RE2) for pattern matching.
- The regex is compiled with caching and matching runs under a timeout.
- Returns empty collection if the input is empty.
- A `flags` value containing anything other than `i` or `m` is an error, as the specification requires.
- `.` matches a newline in every mode, which is the 'single line' mode the specification fixes.

---

## matchesFull

Returns `true` if the regular expression matches the **entire** input string.

**Signature:**

```text
matchesFull(regex : String [, flags : String]) : Boolean
```

**Parameters:**

| Name      | Type     | Description                    |
|-----------|----------|--------------------------------|
| `regex`   | `String` | A regular expression pattern   |
| `flags`   | `String` | Optional. `i` for a case-insensitive search, `m` to make `^` and `$` match the start and end of each line |

**Return Type:** `Boolean`

The specification defines it as the form where the start/end markers always
surround the pattern, so it answers the question `matches` does not: does this
value, all of it, have the shape the pattern describes.

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'ABC'.matchesFull('[A-Z]{3}')")
// true

result, _ := fhirpath.Evaluate(resource, "'ABCD'.matchesFull('[A-Z]{3}')")
// false — matches would be true

result, _ := fhirpath.Evaluate(resource, "'N8000123123'.matchesFull('N[0-9]{8}')")
// false — the string has 10 digits, not 8

result, _ := fhirpath.Evaluate(resource, "'N8000123123'.matchesFull('N[0-9]{10}')")
// true
```

Side by side with `matches`, on the same input and pattern:

| Expression | `matches` | `matchesFull` |
|---|---|---|
| `'a/Library/b'` with `'Library'` | true | false |
| `'a/Library/b'` with `'.*Library.*'` | true | true |
| `'  X  '` with `'[A-Z][A-Za-z0-9_]*'` | true | false |

**Edge Cases / Notes:**

- Anchoring uses `\A` and `\z`, which bound the whole text regardless of any `^` or `$` inside the pattern — an already-anchored pattern keeps its own meaning.
- Returns empty collection if the input or the regex is empty.
- Defined in FHIRPath 3.0.0 and marked Standard for Trial Use.

---

## replaceMatches

Replaces all occurrences of a regex pattern with the substitution string.

**Signature:**

```text
replaceMatches(regex : String, substitution : String [, flags : String]) : String
```

**Parameters:**

| Name             | Type     | Description                    |
|------------------|----------|--------------------------------|
| `regex`          | `String` | A regular expression pattern   |
| `substitution`   | `String` | The replacement string         |

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello   world'.replaceMatches('\\\\s+', ' ')")
// "hello   world" -> "hello world"

result, _ := fhirpath.Evaluate(resource, "'abc123def'.replaceMatches('[0-9]+', 'NUM')")
// "abc123def" -> "abcNUMdef"

result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.replaceMatches('(\\\\d{4})-(\\\\d{2})-(\\\\d{2})', '$2/$3/$1')")
// "2024-01-15" -> "01/15/2024"
```

**Edge Cases / Notes:**

- Uses Go's `regexp` package with cached compilation and ReDoS timeout protection.
- Substitution supports backreferences (`$1`, `$2`, etc.).
- Returns empty collection if the input is empty.

---

## indexOf

Returns the zero-based index of the first occurrence of the given substring, or `-1` if not found.

**Signature:**

```text
indexOf(substring : String) : Integer
```

**Parameters:**

| Name          | Type     | Description                  |
|---------------|----------|------------------------------|
| `substring`   | `String` | The substring to search for  |

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello world'.indexOf('world')")
// 6

result, _ := fhirpath.Evaluate(resource, "'hello world'.indexOf('xyz')")
// -1

result, _ := fhirpath.Evaluate(resource, "'abcabc'.indexOf('bc')")
// 1 (first occurrence)
```

**Edge Cases / Notes:**

- Returns `-1` when the substring is not found.
- Returns `0` when searching for an empty string.
- Returns empty collection if the input is empty.
- The search is case-sensitive.

---

## substring

Returns a substring starting at the given zero-based index, optionally limited to a specified length.

**Signature:**

```text
substring(start : Integer [, length : Integer]) : String
```

**Parameters:**

| Name       | Type      | Description                                           |
|------------|-----------|-------------------------------------------------------|
| `start`    | `Integer` | Zero-based start index                                |
| `length`   | `Integer` | (Optional) Maximum number of characters to return     |

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello world'.substring(6)")
// "world"

result, _ := fhirpath.Evaluate(resource, "'hello world'.substring(0, 5)")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'abc'.substring(1, 10)")
// "bc" (length exceeds string, returns to end)
```

**Edge Cases / Notes:**

- If `start` is negative or greater than or equal to the string length, returns an empty collection.
- If `length` would extend beyond the end of the string, returns characters up to the end.
- Returns empty collection if the input is empty.

---

## lower

Returns the input string converted to lowercase.

**Signature:**

```text
lower() : String
```

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'Hello World'.lower()")
// "hello world"

result, _ := fhirpath.Evaluate(resource, "'ABC123'.lower()")
// "abc123"

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.lower()")
// "Smith" -> "smith"
```

**Edge Cases / Notes:**

- Non-alphabetic characters are left unchanged.
- Returns empty collection if the input is empty.

---

## upper

Returns the input string converted to uppercase.

**Signature:**

```text
upper() : String
```

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'Hello World'.upper()")
// "HELLO WORLD"

result, _ := fhirpath.Evaluate(resource, "'abc123'.upper()")
// "ABC123"

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.upper()")
// "Smith" -> "SMITH"
```

**Edge Cases / Notes:**

- Non-alphabetic characters are left unchanged.
- Returns empty collection if the input is empty.

---

## length

Returns the number of characters in the input string.

**Signature:**

```text
length() : Integer
```

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello'.length()")
// 5

result, _ := fhirpath.Evaluate(resource, "''.length()")
// 0

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.length()")
// "Smith" -> 5
```

**Edge Cases / Notes:**

- Returns the byte length of the string (Go's `len`), not the rune count. For ASCII strings these are identical.
- Returns empty collection if the input is empty.

---

## toChars

Converts a string into a collection of individual characters, where each character is a separate `String` element.

**Signature:**

```text
toChars() : Collection
```

**Return Type:** `Collection` of `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'abc'.toChars()")
// { "a", "b", "c" }

result, _ := fhirpath.Evaluate(resource, "'Hi'.toChars().count()")
// 2

result, _ := fhirpath.Evaluate(resource, "''.toChars()")
// { } (empty collection)
```

**Edge Cases / Notes:**

- Iterates over Unicode runes, so multi-byte characters are handled correctly.
- An empty string returns an empty collection.
- Returns empty collection if the input is empty.

---

## trim

Removes leading and trailing whitespace from the input string.

**Signature:**

```text
trim() : String
```

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'  hello  '.trim()")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'\\thello\\n'.trim()")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'nospace'.trim()")
// "nospace" (unchanged)
```

**Edge Cases / Notes:**

- Uses Go's `strings.TrimSpace`, which removes all Unicode whitespace characters (spaces, tabs, newlines, etc.).
- Does not remove whitespace from the middle of the string.
- Returns empty collection if the input is empty.

---

## split

Splits the input string by the given separator and returns a collection of substrings.

**Signature:**

```text
split(separator : String) : Collection
```

**Parameters:**

| Name          | Type     | Description                |
|---------------|----------|----------------------------|
| `separator`   | `String` | The delimiter to split on  |

**Return Type:** `Collection` of `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',')")
// { "a", "b", "c" }

result, _ := fhirpath.Evaluate(resource, "'hello world'.split(' ')")
// { "hello", "world" }

result, _ := fhirpath.Evaluate(resource, "'no-delimiter'.split(',')")
// { "no-delimiter" } (single element)
```

**Edge Cases / Notes:**

- If the separator is not found, returns a collection with the entire string as a single element.
- An empty separator splits into individual characters (Go behavior of `strings.Split`).
- Returns empty collection if the input is empty.

---

## join

Joins a collection of strings into a single string, optionally separated by a given separator.

**Signature:**

```text
join([separator : String]) : String
```

**Parameters:**

| Name          | Type     | Description                                                                       |
|---------------|----------|-----------------------------------------------------------------------------------|
| `separator`   | `String` | (Optional) The string to place between elements. Defaults to empty string (`''`)  |

**Return Type:** `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().given.join(', ')")
// Given names "John", "James" -> "John, James"

result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',').join('-')")
// "a-b-c"

result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',').join()")
// "abc" (no separator)
```

**Edge Cases / Notes:**

- If the input collection is empty, returns an empty string (`""`), not an empty collection.
- Non-string elements in the collection are converted to their string representation using `.String()`.
- If no separator argument is provided, elements are concatenated directly.

---

## encode / decode

{{< callout type="warning" >}}
**Not Implemented:** The `encode(encoding)` and `decode(encoding)` functions for base64 encoding/decoding are defined in the FHIRPath specification but are **not yet implemented** in this library. Calling these functions will result in an error.
{{< /callout >}}

**Signature:**

```text
encode(encoding : String) : String
decode(encoding : String) : String
```

**Description:**

- `encode('base64')` -- Encodes the input string to base64.
- `decode('base64')` -- Decodes a base64-encoded string.

These functions are planned for a future release.
