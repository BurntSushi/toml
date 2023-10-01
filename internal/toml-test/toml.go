package tomltest

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// CompareTOML compares the given arguments.
//
// The returned value is a copy of Test with Failure set to a (human-readable)
// description of the first element that is unequal. If both arguments are equal
// Test is returned unchanged.
//
// Reflect.DeepEqual could work here, but it won't tell us how the two
// structures are different.
func (r Test) CompareTOML(want, have any) Test {
	if isTomlValue(want) {
		if !isTomlValue(have) {
			return r.fail("Type for key '%s' differs:\n"+
				"  Expected:     %v (%s)\n"+
				"  Your encoder: %v (%s)",
				r.Key, want, fmtType(want), have, fmtType(have))
		}

		if !deepEqual(want, have) {
			return r.fail("Values for key '%s' differ:\n"+
				"  Expected:     %v (%s)\n"+
				"  Your encoder: %v (%s)",
				r.Key, want, fmtType(want), have, fmtType(have))
		}
		return r
	}

	switch w := want.(type) {
	case map[string]any:
		return r.cmpTOMLMap(w, have)
	case []map[string]any:
		ww := make([]any, 0, len(w))
		for _, v := range w {
			ww = append(ww, v)
		}
		return r.cmpTOMLArrays(ww, have)
	case []any:
		return r.cmpTOMLArrays(w, have)
	default:
		return r.fail("Unrecognized TOML structure: %s", fmtType(want))
	}
}

func (r Test) cmpTOMLMap(want map[string]any, have any) Test {
	haveMap, ok := have.(map[string]any)
	if !ok {
		return r.mismatch("table", want, haveMap)
	}

	// Check that the keys of each map are equivalent.
	for k := range want {
		if _, ok := haveMap[k]; !ok {
			bunk := r.kjoin(k)
			return bunk.fail("Could not find key '%s' in encoder output", bunk.Key)
		}
	}
	for k := range haveMap {
		if _, ok := want[k]; !ok {
			bunk := r.kjoin(k)
			return bunk.fail("Could not find key '%s' in expected output", bunk.Key)
		}
	}

	// Okay, now make sure that each value is equivalent.
	for k := range want {
		if sub := r.kjoin(k).CompareTOML(want[k], haveMap[k]); sub.Failed() {
			return sub
		}
	}
	return r
}

func (r Test) cmpTOMLArrays(want []any, have any) Test {
	// Slice can be decoded to []any for an array of primitives, or
	// []map[string]any for an array of tables.
	//
	// TODO: it would be nicer if it could always decode to []any?
	haveSlice, ok := have.([]any)
	if !ok {
		tblArray, ok := have.([]map[string]any)
		if !ok {
			return r.mismatch("array", want, have)
		}

		haveSlice = make([]any, len(tblArray))
		for i := range tblArray {
			haveSlice[i] = tblArray[i]
		}
	}

	if len(want) != len(haveSlice) {
		return r.fail("Array lengths differ for key '%s'"+
			"  Expected:     %[2]v (len=%[4]d)\n"+
			"  Your encoder: %[3]v (len=%[5]d)",
			r.Key, want, haveSlice, len(want), len(haveSlice))
	}
	for i := 0; i < len(want); i++ {
		if sub := r.CompareTOML(want[i], haveSlice[i]); sub.Failed() {
			return sub
		}
	}
	return r
}

// reflect.DeepEqual() that deals with NaN != NaN
func deepEqual(want, have any) bool {
	var wantF, haveF float64
	switch f := want.(type) {
	case float32:
		wantF = float64(f)
	case float64:
		wantF = f
	}
	switch f := have.(type) {
	case float32:
		haveF = float64(f)
	case float64:
		haveF = f
	}
	if math.IsNaN(wantF) && math.IsNaN(haveF) {
		return true
	}

	return reflect.DeepEqual(want, have)
}

func isTomlValue(v any) bool {
	switch v.(type) {
	case map[string]any, []map[string]any, []any:
		return false
	}
	return true
}

// fmt %T with "interface {}" replaced with "any", which is far more readable.
func fmtType(t any) string  { return strings.ReplaceAll(fmt.Sprintf("%T", t), "interface {}", "any") }
func fmtHashV(t any) string { return strings.ReplaceAll(fmt.Sprintf("%#v", t), "interface {}", "any") }
