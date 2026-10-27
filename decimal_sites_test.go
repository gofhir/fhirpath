package fhirpath_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// decimalSites type-checks a package and lists the functions in it that call a
// rescaling method on a shopspring Decimal, as "dir.Func".
func decimalSites(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check(dir, fset, files, info); err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				name = types.ExprString(fn.Recv.List[0].Type) + "." + name
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				s, ok := info.Selections[sel]
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
