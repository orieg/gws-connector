package services

import (
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
