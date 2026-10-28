package fhirpath_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// rescalingMethods are the methods of shopspring's Decimal whose cost grows
// with the distance between the operands' exponents, or with the value's
// digits: on a decimal beyond types.DecimalDigits they write it out (#232).
var rescalingMethods = []string{
	"Add", "Sub", "Mul", "Div", "DivRound", "QuoRem", "Mod", "Pow", "PowInt32",
	"PowBigInt", "PowWithPrecision", "Cmp", "Compare", "Equal", "Equals",
	"GreaterThan", "GreaterThanOrEqual", "LessThan", "LessThanOrEqual",
	"Round", "RoundBank", "RoundCash", "RoundCeil", "RoundFloor", "RoundUp",
	"RoundDown", "Truncate", "Floor", "Ceil", "IntPart", "BigInt", "BigFloat",
	"Rat", "Float64", "InexactFloat64", "String", "StringFixed",
	"StringFixedBank", "StringFixedCash", "Sqrt", "Ln", "ExpTaylor",
	"ExpHullAbrham", "Abs", "Neg", "Atan", "Sin", "Cos", "Tan", "NumDigits",
}

// guardedDecimalSites names the functions outside the types package that call
// a rescaling method on a shopspring Decimal, each of which makes sure none of
// its operands is beyond the bound first. A new site fails this test until it
// is guarded and named here.
var guardedDecimalSites = []string{
	// beyondOperand, first
	"eval.IntegerDivide", "eval.Modulo",
	// noNumber on the input and each number argument, first
	"funcs.fnAbs", "funcs.numberInput", "funcs.quantityResult", "funcs.fnTruncate",
	"funcs.naturalLog", "funcs.fnRound",
	// holdsNoAmount, first
	"funcs.fnSum", "funcs.sumNumbers", "funcs.fnAvg",
	// boundaryOf's InRange, first
	"funcs.decimalBoundary",
	// no decimal of the input: a precision's interval, a timezone's minutes
	"funcs.boundaryInterval", "funcs.offsetInHours",
}

// checkedPackage is a production package, type-checked once for the guards
// below.
type checkedPackage struct {
	name  string
	files []*ast.File
	info  *types.Info
}

var (
	checkOnce sync.Once
	checked   map[string]checkedPackage
	checkErr  error
)

// productionPackages type-checks every production package of this module,
// found by walking it, once and with one importer, so that their
// dependencies are checked once too. Keyed by directory.
func productionPackages(t *testing.T) map[string]checkedPackage {
	t.Helper()
	checkOnce.Do(func() {
		checked = map[string]checkedPackage{}
		fset := token.NewFileSet()
		conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
		checkErr = filepath.WalkDir(".", func(dir string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if dir != "." {
				if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil || d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") {
					return filepath.SkipDir
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				return err
			}
			var files []*ast.File
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				f, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
				if parseErr != nil {
					return parseErr
				}
				files = append(files, f)
			}
			if files == nil {
				return nil
			}
			info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Uses: map[*ast.Ident]types.Object{}}
			pkg, err := conf.Check(dir, fset, files, info)
			if err != nil {
				return err
			}
			checked[dir] = checkedPackage{name: pkg.Name(), files: files, info: info}
			return nil
		})
	})
	if checkErr != nil {
		t.Fatal(checkErr)
	}
	return checked
}

// declName names a declaration as the guards report it: a function by its
// name, a method by its receiver and name, any other declaration "var".
func declName(decl ast.Decl) string {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok {
		return "var"
	}
	if fn.Recv != nil && len(fn.Recv.List) == 1 {
		return types.ExprString(fn.Recv.List[0].Type) + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// decimalSites lists the functions of a package that call a rescaling method
// on a shopspring Decimal, as "dir.Func".
func decimalSites(t *testing.T, dir string) []string {
	t.Helper()
	pkg, ok := productionPackages(t)[dir]
	if !ok {
		t.Fatalf("no production package in %s", dir)
	}
	var sites []string
	for _, f := range pkg.files {
		for _, decl := range f.Decls {
			name := declName(decl)
			ast.Inspect(decl, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				s, ok := pkg.info.Selections[sel]
				if !ok || s.Kind() != types.MethodVal || !slices.Contains(rescalingMethods, sel.Sel.Name) {
					return true
				}
				if named, ok := s.Recv().(*types.Named); ok && named.Obj().Pkg() != nil &&
					named.Obj().Pkg().Path() == "github.com/shopspring/decimal" && named.Obj().Name() == "Decimal" {
					if site := dir + "." + name; !slices.Contains(sites, site) {
						sites = append(sites, site)
					}
				}
				return true
			})
		}
	}
	return sites
}

func TestEveryRescalingOfADecimalOutsideTypesIsGuarded(t *testing.T) {
	for _, dir := range []string{"eval", "funcs"} {
		for _, site := range decimalSites(t, dir) {
			if !slices.Contains(guardedDecimalSites, site) {
				t.Errorf("%s rescales a shopspring Decimal and is not in guardedDecimalSites: guard it against a decimal beyond the bound, then name it", site)
			}
		}
	}
}

// decodingFuncs are shopspring's functions and methods that read a decimal
// from text or bytes. Since v1.5.0 they refuse an exponent past
// MaxDecodeExponent, a process-wide setting, and v1.6.0 reads a Unicode minus,
// so text from the input is read by types.decodeDecimal instead (#235).
var decodingFuncs = []string{
	"NewFromString", "RequireFromString", "NewFromFormattedString", "NewFromBigFloat",
	"UnmarshalJSON", "UnmarshalText", "UnmarshalBinary", "GobDecode", "Scan",
	"DecodeSpanner", "UnmarshalBSONValue",
}

// rereadDecimalSites names the functions that read a decimal from text the
// engine wrote itself, a value's String() or a constant. String() writes no
// exponent, so no exponent past MaxDecodeExponent reaches them, though a
// value beyond the bound may. A new site fails this test until it decodes
// through types.decodeDecimal or is named here.
var rereadDecimalSites = []string{
	// a value's String(): #237 takes these off String(), which
	// UseScientificNotation would give an exponent
	"types.dropTrailingZeros", "funcs.quantityResult", "ucum.fromRat",
	// "0.0" and "1.0"
	"funcs.quantityOf",
}

// decodingSites lists, over every production package, the functions that
// use one of decodingFuncs of shopspring/decimal, as "pkg.Func" or
// "pkg.Recv.Method"; a use in a package-level declaration is named "pkg.var".
func decodingSites(t *testing.T) []string {
	t.Helper()
	var sites []string
	for _, pkg := range productionPackages(t) {
		for _, f := range pkg.files {
			for _, decl := range f.Decls {
				name := declName(decl)
				ast.Inspect(decl, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}
					obj, ok := pkg.info.Uses[id].(*types.Func)
					if !ok || obj.Pkg() == nil || obj.Pkg().Path() != "github.com/shopspring/decimal" || !slices.Contains(decodingFuncs, obj.Name()) {
						return true
					}
					if site := pkg.name + "." + name; !slices.Contains(sites, site) {
						sites = append(sites, site)
					}
					return true
				})
			}
		}
	}
	return sites
}

func TestEveryDecodingOfADecimalGoesThroughTheEngine(t *testing.T) {
	if _, ok := productionPackages(t)["types"]; !ok {
		t.Fatal("the walk found no types package")
	}
	for _, site := range decodingSites(t) {
		if !slices.Contains(rereadDecimalSites, site) {
			t.Errorf("%s decodes a decimal with shopspring/decimal: read input text through types.decodeDecimal, or name a re-read of the engine's own text in rereadDecimalSites", site)
		}
	}
}
