package funcs

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// now(), today() and timeOfDay() have one implementation each, the one that
// reads the evaluation's instant (#225); a second registration once
// overrode the first by the order the files' init functions ran in.
func TestTheCurrentTimeFunctionsHaveOneImplementation(t *testing.T) {
	for name, want := range map[string]string{"now": "fnNow", "today": "fnToday", "timeOfDay": "fnTimeOfDay"} {
		def, ok := Get(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		fn := runtime.FuncForPC(reflect.ValueOf(def.Fn).Pointer())
		file, _ := fn.FileLine(fn.Entry())
		if got := filepath.Ext(fn.Name()); got != "."+want || filepath.Base(file) != "temporal.go" {
			t.Errorf("%s is %s in %s, want %s in temporal.go", name, fn.Name(), filepath.Base(file), want)
		}
	}
}
