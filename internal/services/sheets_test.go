package services

import (
	"encoding/json"
	"strings"
	"testing"

	sheets "google.golang.org/api/sheets/v4"
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

func testTabs() []*sheets.Sheet {
	return []*sheets.Sheet{
		{Properties: &sheets.SheetProperties{Title: "2026年7月", SheetId: 0, Index: 0}},
		{Properties: &sheets.SheetProperties{Title: "2026年8月", SheetId: 123, Index: 1}},
		{Properties: nil}, // tolerate malformed entries
	}
}

func TestEnsureTabTitleFree(t *testing.T) {
	tabs := testTabs()
	if err := ensureTabTitleFree(tabs, "2026年9月"); err != nil {
		t.Errorf("new title should be free, got %v", err)
	}
	err := ensureTabTitleFree(tabs, "2026年8月")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate title should error with 'already exists', got %v", err)
	}
}

// Google Sheets tab names are unique case-insensitively ("Sheet1" and
// "sheet1" clash), so the pre-check must be too.
func TestEnsureTabTitleFree_CaseInsensitive(t *testing.T) {
	tabs := []*sheets.Sheet{{Properties: &sheets.SheetProperties{Title: "Invoice", SheetId: 5}}}
	if err := ensureTabTitleFree(tabs, "invoice"); err == nil {
		t.Error("expected case-insensitive clash to error")
	}
}

func TestEnsureTabTitleFree_Whitespace(t *testing.T) {
	tabs := []*sheets.Sheet{{Properties: &sheets.SheetProperties{Title: "Invoice", SheetId: 5}}}
	if err := ensureTabTitleFree(tabs, "  invoice  "); err == nil {
		t.Error("expected whitespace-padded clash to error")
	}
}

func TestFindSourceTab(t *testing.T) {
	tabs := testTabs()

	p, err := findSourceTab(tabs, "2026年8月", 0, false)
	if err != nil || p.SheetId != 123 {
		t.Errorf("by title: got %v, %v", p, err)
	}
	// sheet_id 0 is valid (the first tab usually has it).
	p, err = findSourceTab(tabs, "", 0, true)
	if err != nil || p.Title != "2026年7月" {
		t.Errorf("by id 0: got %v, %v", p, err)
	}
	p, err = findSourceTab(tabs, "2026年8月", 123, true)
	if err != nil || p.SheetId != 123 {
		t.Errorf("title+id agree: got %v, %v", p, err)
	}
}

func TestFindSourceTab_Errors(t *testing.T) {
	tabs := testTabs()
	cases := []struct {
		name  string
		title string
		id    int64
		hasID bool
		want  string
	}{
		{"missing title", "2025年1月", 0, false, "not found"},
		{"missing id", "", 999, true, "not found"},
		{"title and id disagree", "2026年8月", 0, true, "different tabs"},
	}
	for _, c := range cases {
		if _, err := findSourceTab(tabs, c.title, c.id, c.hasID); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: expected error containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestOptionalNonNegativeInt(t *testing.T) {
	if _, present, err := optionalNonNegativeInt(map[string]any{}, "index"); present || err != nil {
		t.Errorf("absent key: present=%v err=%v", present, err)
	}
	if _, present, err := optionalNonNegativeInt(map[string]any{"index": nil}, "index"); present || err != nil {
		t.Errorf("null value: present=%v err=%v", present, err)
	}
	v, present, err := optionalNonNegativeInt(map[string]any{"index": float64(0)}, "index")
	if !present || err != nil || v != 0 {
		t.Errorf("zero must be present: v=%d present=%v err=%v", v, present, err)
	}
	v, present, err = optionalNonNegativeInt(map[string]any{"index": json.Number("3")}, "index")
	if !present || err != nil || v != 3 {
		t.Errorf("json.Number 3: v=%d present=%v err=%v", v, present, err)
	}
	for _, bad := range []any{float64(-1), float64(1.5), "2", true} {
		if _, _, err := optionalNonNegativeInt(map[string]any{"index": bad}, "index"); err == nil {
			t.Errorf("expected error for %#v", bad)
		}
	}
}
