package services

import (
	"strings"
	"testing"

	docs "google.golang.org/api/docs/v1"
)

func TestExtractDocPlainText_NilSafe(t *testing.T) {
	if got := extractDocPlainText(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestExtractDocPlainText_Paragraphs(t *testing.T) {
	doc := &docs.Document{
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{TextRun: &docs.TextRun{Content: "Hello, "}},
							{TextRun: &docs.TextRun{Content: "world.\n"}},
						},
					},
				},
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{TextRun: &docs.TextRun{Content: "Second paragraph.\n"}},
						},
					},
				},
			},
		},
	}
	got := extractDocPlainText(doc)
	want := "Hello, world.\nSecond paragraph."
	if got != want {
		t.Errorf("plain text mismatch.\n got: %q\nwant: %q", got, want)
	}
}

func TestExtractDocPlainText_Table(t *testing.T) {
	doc := &docs.Document{
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{Table: &docs.Table{TableRows: []*docs.TableRow{
					{TableCells: []*docs.TableCell{
						{Content: []*docs.StructuralElement{
							{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
								{TextRun: &docs.TextRun{Content: "cell content\n"}},
							}}},
						}},
					}},
				}}},
			},
		},
	}
	got := extractDocPlainText(doc)
	if !strings.Contains(got, "cell content") {
		t.Errorf("expected table cell content in output, got %q", got)
	}
}

func TestParse1BasedIndex(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		err  bool
	}{
		{"1", 0, false},
		{"42", 41, false},
		{"end", 0, true},
		{"0", 0, true},
		{"-3", 0, true},
		{"", 0, true},
	}
	for _, tc := range cases {
		got, err := parse1BasedIndex(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("parse1BasedIndex(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parse1BasedIndex(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parse1BasedIndex(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
