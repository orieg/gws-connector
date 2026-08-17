package services

import (
	"strings"
	"testing"

	slides "google.golang.org/api/slides/v1"
)

func TestExtractSlideText_NilSafe(t *testing.T) {
	if got := extractSlideText(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestExtractSlideText_ShapesAndTable(t *testing.T) {
	slide := &slides.Page{
		ObjectId: "slide-1",
		PageElements: []*slides.PageElement{
			{
				Shape: &slides.Shape{
					Text: &slides.TextContent{
						TextElements: []*slides.TextElement{
							{TextRun: &slides.TextRun{Content: "Title line\n"}},
						},
					},
				},
			},
			{
				Table: &slides.Table{
					TableRows: []*slides.TableRow{
						{TableCells: []*slides.TableCell{
							{Text: &slides.TextContent{TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "cell A"}},
							}}},
							{Text: &slides.TextContent{TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "cell B"}},
							}}},
						}},
					},
				},
			},
			// Non-text element (image) — contributes nothing.
			{Image: &slides.Image{ContentUrl: "https://example.com/x.png"}},
		},
	}

	got := extractSlideText(slide)
	if !strings.Contains(got, "Title line") {
		t.Errorf("expected shape text, got %q", got)
	}
	if !strings.Contains(got, "cell A") || !strings.Contains(got, "cell B") {
		t.Errorf("expected table cell text, got %q", got)
	}
}

func TestExtractSlideText_Group(t *testing.T) {
	slide := &slides.Page{
		PageElements: []*slides.PageElement{
			{
				ElementGroup: &slides.Group{
					Children: []*slides.PageElement{
						{Shape: &slides.Shape{Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "grouped text"}},
							},
						}}},
					},
				},
			},
		},
	}
	got := extractSlideText(slide)
	if !strings.Contains(got, "grouped text") {
		t.Errorf("expected grouped element text, got %q", got)
	}
}

func TestExtractSlideText_EmptySlide(t *testing.T) {
	slide := &slides.Page{ObjectId: "empty"}
	if got := extractSlideText(slide); got != "" {
		t.Errorf("expected empty string for slide with no text, got %q", got)
	}
}
