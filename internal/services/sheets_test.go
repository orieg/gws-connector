package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateA1Range_Accepts(t *testing.T) {
	ok := []string{
		"A1",
		"A1:B10",
		"AA1:AB10",
		"Sheet1!A1",
		"Sheet1!A1:C10",
		"'My Sheet'!A1:B10",
		"'Sheet with ''quotes'''!A1:B2",
		"Sheet1",
		"A:A",
		"1:1",
	}
	for _, s := range ok {
		if err := validateA1Range(s); err != nil {
			t.Errorf("validateA1Range(%q) expected ok, got %v", s, err)
		}
	}
}

func TestValidateA1Range_Rejects(t *testing.T) {
	bad := []string{
		"",
		"Sheet1!A1\nA2",
		"A1:B2,C1:D2",  // multi-range — use a batch call
		"not a range!", // trailing bang is not a sheet separator
		"!A1",          // empty sheet name
	}
	for _, s := range bad {
		if err := validateA1Range(s); err == nil {
			t.Errorf("validateA1Range(%q) expected error", s)
		} else if !strings.Contains(err.Error(), "example") && !strings.Contains(err.Error(), "required") {
			t.Errorf("validateA1Range(%q) error should show an example or name the required field: %v", s, err)
		}
	}
}

func TestCoerceCellGrid_RejectsNonArray(t *testing.T) {
	if _, err := coerceCellGrid("A1"); err == nil {
		t.Error("expected error for scalar input")
	}
}

func TestCoerceCellGrid_RejectsNonRowArray(t *testing.T) {
	_, err := coerceCellGrid([]any{"A1", "B1"})
	if err == nil {
		t.Error("expected error for 1D array")
	}
}

func TestCoerceCellGrid_AcceptsMixedScalars(t *testing.T) {
	in := []any{
		[]any{"a", float64(1), true, nil},
		[]any{"b", float64(2.5), false, "end"},
	}
	out, err := coerceCellGrid(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 || len(out[0]) != 4 {
		t.Fatalf("grid shape wrong: %v", out)
	}
	if out[0][0] != "a" || out[0][2] != true || out[0][3] != nil {
		t.Errorf("row 0 wrong: %v", out[0])
	}
}

func TestCoerceCellGrid_RejectsUnsupportedCell(t *testing.T) {
	in := []any{
		[]any{map[string]any{"foo": "bar"}},
	}
	if _, err := coerceCellGrid(in); err == nil {
		t.Error("expected error for map cell type")
	}
}

func TestRenderCellGrid_Empty(t *testing.T) {
	if got := renderCellGrid(nil); !strings.Contains(got, "empty") {
		t.Errorf("nil grid should render 'empty' marker, got %q", got)
	}
}

func TestRenderCellGrid_FormatsFloatsAndBools(t *testing.T) {
	grid := [][]interface{}{
		{"name", "qty", "in_stock"},
		{"widget", float64(3), true},
		{"gizmo", float64(1.5), false},
	}
	got := renderCellGrid(grid)
	if !strings.Contains(got, "widget\t3\tTRUE") {
		t.Errorf("int-valued float or bool formatted wrong: %q", got)
	}
	if !strings.Contains(got, "gizmo\t1.5\tFALSE") {
		t.Errorf("fractional float or bool formatted wrong: %q", got)
	}
}

// A JSON decoder configured with UseNumber() hands us json.Number instead
// of float64. coerceCellGrid must route those to numeric cell values so
// "3" goes over the wire to Sheets as 3 (not "3"), and "3.14" as 3.14.
func TestCoerceCellGrid_HandlesJSONNumber(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`[[3, 3.14, "x", true]]`))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	out, err := coerceCellGrid(raw)
	if err != nil {
		t.Fatalf("coerce: %v", err)
	}
	if len(out) != 1 || len(out[0]) != 4 {
		t.Fatalf("grid shape wrong: %v", out)
	}
	// "3" should become an integer (int64), not a string.
	if _, ok := out[0][0].(int64); !ok {
		t.Errorf("expected int64 for integer JSON number, got %T (%v)", out[0][0], out[0][0])
	}
	// "3.14" should become a float64.
	if _, ok := out[0][1].(float64); !ok {
		t.Errorf("expected float64 for fractional JSON number, got %T (%v)", out[0][1], out[0][1])
	}
	if out[0][2] != "x" || out[0][3] != true {
		t.Errorf("other scalars wrong: %v", out[0])
	}
}

// An invalid json.Number that isn't a parseable int or float falls back to
// the string form — degrading gracefully rather than erroring the whole
// write.
func TestCoerceCellGrid_JSONNumberFallbackToString(t *testing.T) {
	in := []any{
		[]any{json.Number("not-a-number")},
	}
	out, err := coerceCellGrid(in)
	if err != nil {
		t.Fatalf("coerce: %v", err)
	}
	if s, ok := out[0][0].(string); !ok || s != "not-a-number" {
		t.Errorf("expected string fallback, got %T %v", out[0][0], out[0][0])
	}
}

func TestCoerceCellGrid_AcceptsNestedAllowedTypes(t *testing.T) {
	// int32 and int arrive through this package only via tests; ensure the
	// type-switch arms stay exercised.
	in := []any{
		[]any{int(7), int32(8), int64(9)},
	}
	out, err := coerceCellGrid(in)
	if err != nil {
		t.Fatalf("coerce: %v", err)
	}
	if len(out[0]) != 3 {
		t.Fatalf("shape wrong: %v", out)
	}
}

func TestFormatCell_TrueInt(t *testing.T) {
	if got := formatCell(float64(5)); got != "5" {
		t.Errorf("expected '5' for 5.0, got %q", got)
	}
	if got := formatCell(float64(3.14)); got != "3.14" {
		t.Errorf("expected '3.14', got %q", got)
	}
	if got := formatCell(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestFormatCell_Bool(t *testing.T) {
	if got := formatCell(true); got != "TRUE" {
		t.Errorf("expected 'TRUE' for true, got %q", got)
	}
	if got := formatCell(false); got != "FALSE" {
		t.Errorf("expected 'FALSE' for false, got %q", got)
	}
}

func TestFormatCell_UnknownTypeFallsBack(t *testing.T) {
	// Exercise the default arm — unexpected types still render to SOME
	// string rather than panic.
	got := formatCell([]int{1, 2, 3})
	if got == "" {
		t.Errorf("unknown type should not render to empty string")
	}
}

func TestFormatOptionalError(t *testing.T) {
	if got := formatOptionalError(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
	if got := formatOptionalError(errorString("boom")); got != "boom" {
		t.Errorf("expected 'boom', got %q", got)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
