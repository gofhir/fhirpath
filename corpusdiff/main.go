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
// sibling for each backbone (navigation inside a where() is where a lost path
// shows), every root constraint as written and every element constraint as
// P.all(X), which evaluates it on each instance. ele-1 is left out; it is on
// every element and says little. Each is evaluated with the version's model, on
// the raw resource and on a Document.
//
// Usage:
//
//	corpusdiff fetch   -fhir r4|r5 -cache DIR
//	corpusdiff eval    -fhir r4|r5 -cache DIR -out FILE
//	corpusdiff compare [-v] BASE HEAD
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
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

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r5"
)

// versions lists the FHIR versions the corpus can be built for.
var versions = map[string]struct {
	core, examples string
	model          func() fhirpath.Model
}{
	"r4": {"hl7.fhir.r4.core/4.0.1", "hl7.fhir.r4.examples/4.0.1", func() fhirpath.Model { return r4.FHIRPathModel() }},
	"r5": {"hl7.fhir.r5.core/5.0.0", "hl7.fhir.r5.examples/5.0.0", func() fhirpath.Model { return r5.FHIRPathModel() }},
}

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
	fmt.Fprintln(os.Stderr, "usage: corpusdiff fetch -fhir r4|r5 -cache DIR")
	fmt.Fprintln(os.Stderr, "       corpusdiff eval -fhir r4|r5 -cache DIR -out FILE")
	fmt.Fprintln(os.Stderr, "       corpusdiff compare [-v] BASE HEAD")
	os.Exit(2)
}

func fetchCommand(args []string) error {
	flags := flag.NewFlagSet("fetch", flag.ExitOnError)
	fhirVersion := flags.String("fhir", "r4", "FHIR version: r4 or r5")
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
	fhirVersion := flags.String("fhir", "r4", "FHIR version: r4 or r5")
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

	corpus, err := buildCorpus(coreDir)
	if err != nil {
		return err
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	n, err := evaluate(corpus, examplesDir, version.model(), w)
	fmt.Fprintf(os.Stderr, "%s: %d evaluations\n", *fhirVersion, n)
	return err
}

// fetch downloads a package from the FHIR package registry into the cache, once,
// and returns the directory its files are in.
func fetch(cache, pkg string) (string, error) {
	dir := filepath.Join(cache, strings.ReplaceAll(pkg, "/", "#"))
	if _, err := os.Stat(filepath.Join(dir, "package", "package.json")); err == nil {
		return filepath.Join(dir, "package"), nil
	}

	fmt.Fprintln(os.Stderr, "fetching", pkg)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://packages2.fhir.org/packages/"+pkg, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: %s", pkg, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}
	tmp := dir + ".partial"
	_ = os.RemoveAll(tmp)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		// Only the package's top-level JSON is needed, and a name that climbs
		// out of the directory is not written anywhere.
		name := filepath.Clean(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || filepath.Dir(name) != "package" || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := writeFile(filepath.Join(tmp, name), tr); err != nil {
			return "", err
		}
	}

	if err := os.Rename(tmp, dir); err != nil {
		return "", err
	}
	return filepath.Join(dir, "package"), nil
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
func buildCorpus(coreDir string) (map[string][]string, error) {
	files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
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
		corpus[sd.Type] = expressionsFor(&sd)
	}
	return corpus, nil
}

// expressionsFor lists a resource's corpus: its element paths, the same beneath
// a where() over a sibling, and its constraints.
func expressionsFor(sd *structureDefinition) []string {
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
		if !strings.Contains(parent, ".") || len(names) < 2 {
			continue
		}
		for _, name := range names[1:] {
			exprs[fmt.Sprintf("%s.where(%s.exists()).%s", parent, names[0], name)] = true
		}
	}

	list := make([]string, 0, len(exprs))
	for e := range exprs {
		list = append(list, e)
	}
	sort.Strings(list)
	return list
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
				// An expression this engine cannot compile is left out rather
				// than answered, on both sides alike.
				var err error
				if expr, err = fhirpath.Compile(text); err != nil {
					expr = nil
				}
				compiled[text] = expr
			}
			if expr == nil {
				continue
			}

			raw, rawErr := expr.EvaluateWithOptions(data, fhirpath.WithModel(model))
			kept, keptErr := doc.EvaluateWithOptions(expr, fhirpath.WithModel(model))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", filepath.Base(file), text, answer(raw, rawErr), answer(kept, keptErr))
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
