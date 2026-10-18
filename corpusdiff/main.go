// Command corpusdiff evaluates a corpus of expressions over the official FHIR
// examples, and compares two such evaluations.
//
// The official suite asks each expression for its expected answer, and difftest
// asks fhirpath.js. Neither walks real resources deep enough to notice when an
// answer changes three levels down in a Claim: 1.9.2 made four choice elements
// answer empty across twenty R4 examples, and both stayed green. What caught it
// was evaluating what a validator evaluates, over what a validator is given, on
// two revisions of the engine, and reading every answer that changed.
//
// The corpus is built from the version's own packages: every element path of
// every resource's snapshot as an expression, the same beneath a where() over a
// sibling the examples hold alongside each field of a backbone (navigation inside a where() is
// where a lost path shows), every root constraint as written and every element constraint as
// P.all(X), which evaluates it on each instance. ele-1 is left out; it is on
// every element and says little. Each is evaluated with the version's model, on
// the raw resource and on a Document.
//
// Usage:
//
//	corpusdiff fetch   -fhir r4|r4b|r5 -cache DIR
//	corpusdiff eval    -fhir r4|r4b|r5 -cache DIR -out FILE
//	corpusdiff compare [-v] BASE HEAD
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// versions lists the FHIR versions the corpus can be built for.
var versions = map[string]struct {
	core, examples string
	model          func() fhirpath.Model
}{
	"r4":  {"hl7.fhir.r4.core/4.0.1", "hl7.fhir.r4.examples/4.0.1", func() fhirpath.Model { return r4.FHIRPathModel() }},
	"r4b": {"hl7.fhir.r4b.core/4.3.0", "hl7.fhir.r4b.examples/4.3.0", func() fhirpath.Model { return r4b.FHIRPathModel() }},
	"r5":  {"hl7.fhir.r5.core/5.0.0", "hl7.fhir.r5.examples/5.0.0", func() fhirpath.Model { return r5.FHIRPathModel() }},
}

// packagePins is the SHA-256 of each package's tarball as packages2.fhir.org
// serves it. A download that hashes otherwise is refused, so every
// comparison measures the same bytes (#221). packages.fhir.org is not the
// same source: its R4 4.0.1 packages are re-packed, with three more
// StructureDefinitions.
var packagePins = map[string]string{
	"hl7.fhir.r4.core/4.0.1":      "b090bf929e1f665cf2c91583720849695bc38d2892a7c5037c56cb00817fb091",
	"hl7.fhir.r4.examples/4.0.1":  "02aa10ea545301b1e14277123dd43a2a3a9297e6345de677801866df8afe7270",
	"hl7.fhir.r4b.core/4.3.0":     "56ade6affa774053e8e514d480944568705d93d9252e264888ee224907fe40e7",
	"hl7.fhir.r4b.examples/4.3.0": "a64db561a943eab454d85349cfb5a088cbfc57c3d90706db36ae3df64269afa5",
	"hl7.fhir.r5.core/5.0.0":      "74b27cd1bfce9e80eaceac431edf230b0945a443564fbf5512f82e5fa50a80d4",
	"hl7.fhir.r5.examples/5.0.0":  "129eb96eaeab3b14b4c3a89a2e3593a0a79ebddb56ef46e15fac6928a0a5976e",
}

// errNotPinned is what a download whose bytes are not the pinned ones fails
// with: unlike a network failure, a reason for a test to fail, not to skip.
var errNotPinned = errors.New("not the pinned bytes")

// registry is where the packages are fetched from, and maxPackage the most a
// download may be: ten times the largest of them.
const (
	registry   = "https://packages2.fhir.org/packages/"
	maxPackage = 200 << 20
)

// maxResource leaves out the few examples above it — Bundles of tens of
// megabytes — whose constraints run on their entries, which the corpus has on
// their own anyway, and which would otherwise take most of the time.
const maxResource = 3 << 20

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "fetch":
		err = fetchCommand(os.Args[2:])
	case "eval":
		err = evalCommand(os.Args[2:])
	case "compare":
		err = compareCommand(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "corpusdiff:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: corpusdiff fetch -fhir r4|r4b|r5 -cache DIR")
	fmt.Fprintln(os.Stderr, "       corpusdiff eval -fhir r4|r4b|r5 -cache DIR -out FILE")
	fmt.Fprintln(os.Stderr, "       corpusdiff compare [-v] BASE HEAD")
	os.Exit(2)
}

func fetchCommand(args []string) error {
	flags := flag.NewFlagSet("fetch", flag.ExitOnError)
	fhirVersion := flags.String("fhir", "r4", "FHIR version: r4, r4b or r5")
	cache := flags.String("cache", "build/corpusdiff/cache", "where the packages are kept")
	if err := flags.Parse(args); err != nil {
		return err
	}

	version, ok := versions[*fhirVersion]
	if !ok {
		usage()
	}
	for _, pkg := range []string{version.core, version.examples} {
		if _, err := fetch(*cache, pkg); err != nil {
			return err
		}
	}
	return nil
}

func evalCommand(args []string) error {
	flags := flag.NewFlagSet("eval", flag.ExitOnError)
	fhirVersion := flags.String("fhir", "r4", "FHIR version: r4, r4b or r5")
	cache := flags.String("cache", "build/corpusdiff/cache", "where the packages are kept")
	out := flags.String("out", "", "file to write the answers to")
	if err := flags.Parse(args); err != nil {
		return err
	}

	version, ok := versions[*fhirVersion]
	if !ok || *out == "" {
		usage()
	}

	coreDir, err := fetch(*cache, version.core)
	if err != nil {
		return err
	}
	examplesDir, err := fetch(*cache, version.examples)
	if err != nil {
		return err
	}

	corpus, err := buildCorpus(coreDir, examplesDir)
	if err != nil {
		return err
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)

	// A file cut short would read as evaluations one side does not have, so
	// every write is checked, the last ones included.
	n, err := evaluate(corpus, examplesDir, version.model(), w)
	if err == nil {
		err = w.Flush()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	fmt.Fprintf(os.Stderr, "%s: %d evaluations\n", *fhirVersion, n)
	return err
}

// fetch returns the directory of a pinned package's files, downloading it
// into the cache on first use, or when what the cache holds is not the
// pinned bytes.
func fetch(cache, pkg string) (string, error) {
	pin, ok := packagePins[pkg]
	if !ok {
		return "", fmt.Errorf("fetching %s: no pinned SHA-256 for it", pkg)
	}
	return fetchFrom(registry, cache, pkg, pin, maxPackage)
}

// fetchFrom is fetch from base, with the pin and size limit given. The
// download goes to a temporary file, hashed as it is written; bytes whose
// SHA-256 is not the pin are refused before anything is extracted. The
// files are extracted into a temporary directory with the digest beside
// them, which is renamed into place only then, under a name the digest is
// part of, so a package directory always holds its files and its digest, and
// a cached one is used only when that digest is the pin. Temporary names are
// unique, and of two processes fetching the same package at once the one
// that installs it second finds it in place; a refused fetch leaves the cache
// as it was, and nothing in it is removed.
func fetchFrom(base, cache, pkg, pin string, limit int64) (string, error) {
	dir := filepath.Join(cache, packageDirName(pkg, pin))
	if installed(dir, pin) {
		return filepath.Join(dir, "package"), nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr, "fetching", pkg)
	tarball, err := download(base+pkg, cache, pin, limit)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", pkg, err)
	}
	defer os.Remove(tarball)

	tmp, err := os.MkdirTemp(cache, ".fetch-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	f, err := os.Open(tarball)
	if err != nil {
		return "", err
	}
	extracted, err := extract(f, tmp)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", pkg, err)
	}
	if extracted == 0 {
		return "", fmt.Errorf("fetching %s: no JSON under package/ in what was downloaded", pkg)
	}
	if werr := os.WriteFile(filepath.Join(tmp, "sha256"), []byte(pin+"\n"), 0o600); werr != nil {
		return "", werr
	}

	// The directory is named after the pinned bytes, so whatever already sits
	// at that name, put there by another process fetching the same package at
	// once, is this very package. A directory there that is not — one edited
	// by hand — is not removed, since another process may be reading what it
	// holds; it is reported, to be removed by whoever damaged it.
	if err := os.Rename(tmp, dir); err != nil && !installed(dir, pin) {
		return "", fmt.Errorf("fetching %s: %s is not the pinned package; remove it: %w", pkg, dir, err)
	}
	return filepath.Join(dir, "package"), nil
}

// packageDirName is the cache directory a package's pinned bytes are kept
// in: its name and version, and the start of its digest, so that bytes pinned
// otherwise never share it.
func packageDirName(pkg, pin string) string {
	return strings.ReplaceAll(pkg, "/", "#") + "@" + pin[:16]
}

// installed says whether dir holds the package whose bytes are pin: its
// files, and the digest recorded when they were verified.
func installed(dir, pin string) bool {
	marker, err := os.ReadFile(filepath.Join(dir, "sha256"))
	if err != nil || strings.TrimSpace(string(marker)) != pin {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "package", "package.json"))
	return err == nil
}

// download writes what url answers to a temporary file in dir and returns
// its name, refusing an answer that is not 200, one longer than limit, and
// one whose SHA-256 is not pin; a refused download leaves no file.
func download(url, dir, pin string, limit int64) (name string, err error) {
	// A stalled download fails rather than hangs; the largest package is tens
	// of megabytes.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", resp.Status)
	}

	f, err := os.CreateTemp(dir, ".download-")
	if err != nil {
		return "", err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", err
	}
	if n > limit {
		return "", fmt.Errorf("larger than the %d bytes a package may be", limit)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != pin {
		return "", fmt.Errorf("%w: SHA-256 %s is not the pinned %s", errNotPinned, got, pin)
	}
	return f.Name(), f.Close()
}

// extract writes a package tarball's top-level JSON files under dir and says
// how many there were. Nothing else is needed, and a name that climbs out of
// the directory, or is not a regular file, is not written anywhere.
func extract(r io.Reader, dir string) (int, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return 0, err
	}

	extracted := 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return extracted, nil
		}
		if err != nil {
			return extracted, err
		}
		name := filepath.Clean(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || filepath.Dir(name) != "package" || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := writeFile(filepath.Join(dir, name), tr); err != nil {
			return extracted, err
		}
		extracted++
	}
}

func writeFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type structureDefinition struct {
	Kind       string `json:"kind"`
	Type       string `json:"type"`
	Derivation string `json:"derivation"`
	Snapshot   struct {
		Element []struct {
			Path       string `json:"path"`
			Constraint []struct {
				Key        string `json:"key"`
				Expression string `json:"expression"`
			} `json:"constraint"`
		} `json:"element"`
	} `json:"snapshot"`
}

// buildCorpus returns, for each resource type, the expressions to evaluate on
// its examples, sorted so that two runs write their answers in the same order.
func buildCorpus(coreDir, examplesDir string) (map[string][]string, error) {
	files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
	if err != nil {
		return nil, err
	}
	together, err := countFields(examplesDir)
	if err != nil {
		return nil, err
	}

	corpus := map[string][]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var sd structureDefinition
		if json.Unmarshal(data, &sd) != nil || sd.Kind != "resource" || sd.Derivation != "specialization" {
			continue
		}
		corpus[sd.Type] = expressionsFor(&sd, together)
	}
	return corpus, nil
}

// expressionsFor lists a resource's corpus: its element paths, the same beneath
// a where() over a sibling, and its constraints.
func expressionsFor(sd *structureDefinition, together map[string]int) []string {
	exprs := map[string]bool{}
	children := map[string][]string{}
	for _, e := range sd.Snapshot.Element {
		path := strings.TrimSuffix(e.Path, "[x]")
		exprs[path] = true
		if i := strings.LastIndex(path, "."); i > 0 {
			children[path[:i]] = append(children[path[:i]], path[i+1:])
		}

		for _, c := range e.Constraint {
			if c.Expression == "" || c.Key == "ele-1" {
				continue
			}
			if e.Path == sd.Type {
				exprs[c.Expression] = true
			} else {
				exprs[fmt.Sprintf("%s.all(%s)", path, c.Expression)] = true
			}
		}
	}

	for parent, names := range children {
		if !strings.Contains(parent, ".") {
			continue
		}
		for _, name := range names {
			if filter := filterSibling(parent, name, names, together); filter != "" {
				exprs[fmt.Sprintf("%s.where(%s.exists()).%s", parent, filter, name)] = true
			}
		}
	}

	list := make([]string, 0, len(exprs))
	for e := range exprs {
		list = append(list, e)
	}
	sort.Strings(list)
	return list
}

// filterSibling picks the child a where() filters on before reading target:
// the sibling the examples hold most often alongside target, in the same
// instance, or "" when none is ever held with it. A filter that keeps only
// instances without target reads as empty whatever the engine does, and a
// regression beneath it would read as empty on both sides — every backbone
// starts with id, and the first child after it is often optional. The examples
// are counted from their JSON rather than through the engine, so that both
// revisions evaluate the same corpus.
func filterSibling(parent, target string, names []string, together map[string]int) string {
	best, bestCount := "", 0
	for _, name := range names {
		switch name {
		case target, "id", "extension", "modifierExtension":
			continue
		}
		if n := together[parent+"|"+name+"|"+target]; n > bestCount {
			best, bestCount = name, n
		}
	}
	return best
}

// countFields counts, over the examples, how many instances of each element
// path hold two fields together: "Claim.diagnosis|sequence|diagnosis" for every
// diagnosis entry with both. A variant of a choice element counts for its base
// name, valueQuantity for value.
func countFields(examplesDir string) (map[string]int, error) {
	files, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
	if err != nil {
		return nil, err
	}

	together := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if len(data) > maxResource {
			continue
		}
		var resource map[string]any
		if json.Unmarshal(data, &resource) != nil {
			continue
		}
		if resourceType, ok := resource["resourceType"].(string); ok {
			countObject(resourceType, resource, together)
		}
	}
	return together, nil
}

func countObject(path string, object map[string]any, together map[string]int) {
	names := make([]string, 0, len(object))
	for key, value := range object {
		name := strings.TrimPrefix(key, "_")
		if i := strings.IndexFunc(name, func(r rune) bool { return 'A' <= r && r <= 'Z' }); i > 0 {
			name = name[:i]
		}
		names = append(names, name)

		items, isArray := value.([]any)
		if !isArray {
			items = []any{value}
		}
		for _, item := range items {
			if child, ok := item.(map[string]any); ok {
				countObject(path+"."+name, child, together)
			}
		}
	}

	for _, a := range names {
		for _, b := range names {
			if a != b {
				together[path+"|"+a+"|"+b]++
			}
		}
	}
}

// evaluate writes one line per example and expression: the file, the
// expression, and the answer on the raw resource and on a Document. An answer
// that is an error is written as its kind rather than its message, so that
// rewording a message does not read as a changed answer.
func evaluate(corpus map[string][]string, examplesDir string, model fhirpath.Model, w io.Writer) (int, error) {
	files, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
	if err != nil {
		return 0, err
	}
	sort.Strings(files)

	compiled := map[string]*fhirpath.Expression{}
	n := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return n, err
		}
		if len(data) > maxResource {
			continue
		}
		var head struct {
			ResourceType string `json:"resourceType"`
		}
		if json.Unmarshal(data, &head) != nil {
			continue
		}
		doc, err := fhirpath.NewDocument(data)
		if err != nil {
			continue
		}

		for _, text := range corpus[head.ResourceType] {
			expr, seen := compiled[text]
			if !seen {
				var err error
				if expr, err = fhirpath.Compile(text); err != nil {
					expr = nil
				}
				compiled[text] = expr
			}

			// An expression that does not compile is recorded as such, so that
			// one revision compiling what the other cannot is a difference like
			// any other.
			line := "compile-error\tcompile-error"
			if expr != nil {
				raw, rawErr := expr.EvaluateWithOptions(data, fhirpath.WithModel(model))
				kept, keptErr := doc.EvaluateWithOptions(expr, fhirpath.WithModel(model))
				line = answer(raw, rawErr) + "\t" + answer(kept, keptErr)
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", filepath.Base(file), text, line); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func answer(result fhirpath.Collection, err error) string {
	if err != nil {
		var evalErr *eval.EvalError
		if errors.As(err, &evalErr) {
			return "error:" + evalErr.Type.String()
		}
		return "error"
	}
	// Quoted, so that a value holding a newline or a tab stays on its line.
	return strconv.Quote(result.String())
}
