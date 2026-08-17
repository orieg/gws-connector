package services

import (
	"encoding/base64"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestHtmlToText_BasicTags(t *testing.T) {
	input := "<p>Hello <b>world</b></p><p>Second paragraph</p>"
	got := htmlToText(input)
	if got != "Hello world\nSecond paragraph" {
		t.Errorf("got %q", got)
	}
}

func TestHtmlToText_LinksPreserved(t *testing.T) {
	input := `<a href="https://example.com">Click here</a>`
	got := htmlToText(input)
	want := "Click here (https://example.com)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHtmlToText_LinkURLOnly(t *testing.T) {
	// When link text equals the URL, don't duplicate it
	input := `<a href="https://example.com">https://example.com</a>`
	got := htmlToText(input)
	want := "https://example.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHtmlToText_StyleScriptStripped(t *testing.T) {
	input := `<style>body{color:red}</style><script>alert(1)</script><p>Content</p>`
	got := htmlToText(input)
	if got != "Content" {
		t.Errorf("got %q", got)
	}
}

func TestHtmlToText_Entities(t *testing.T) {
	input := `<p>A &amp; B &lt; C &gt; D &quot;E&quot; F&#39;s</p>`
	got := htmlToText(input)
	want := `A & B < C > D "E" F's`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHtmlToText_BrTags(t *testing.T) {
	input := `Line 1<br>Line 2<br/>Line 3`
	got := htmlToText(input)
	want := "Line 1\nLine 2\nLine 3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractBody_PlainTextPreferred(t *testing.T) {
	plain := base64.URLEncoding.EncodeToString([]byte("plain text"))
	html := base64.URLEncoding.EncodeToString([]byte("<p>html text</p>"))

	payload := &gmail.MessagePart{
		MimeType: "multipart/alternative",
		Parts: []*gmail.MessagePart{
			{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: plain}},
			{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: html}},
		},
	}

	got := extractBody(payload)
	if got != "plain text" {
		t.Errorf("expected plain text preferred, got %q", got)
	}
}

func TestExtractBody_HTMLFallback(t *testing.T) {
	html := base64.URLEncoding.EncodeToString([]byte(`<p>Hello from <a href="https://example.com">Example</a></p>`))

	payload := &gmail.MessagePart{
		MimeType: "multipart/alternative",
		Parts: []*gmail.MessagePart{
			{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: html}},
		},
	}

	got := extractBody(payload)
	want := "Hello from Example (https://example.com)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractBody_NoParts(t *testing.T) {
	payload := &gmail.MessagePart{
		MimeType: "multipart/mixed",
	}
	got := extractBody(payload)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCollectAttachments(t *testing.T) {
	payload := &gmail.MessagePart{
		MimeType: "multipart/mixed",
		Parts: []*gmail.MessagePart{
			// Inline body — no filename/attachmentId, must be skipped.
			{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: "aGk="}},
			{
				MimeType: "application/pdf",
				Filename: "report.pdf",
				Body:     &gmail.MessagePartBody{AttachmentId: "att-1", Size: 1234},
			},
			// Nested attachment inside a sub-part.
			{
				MimeType: "multipart/related",
				Parts: []*gmail.MessagePart{
					{
						MimeType: "image/png",
						Filename: "logo.png",
						Body:     &gmail.MessagePartBody{AttachmentId: "att-2", Size: 42},
					},
				},
			},
		},
	}

	atts := collectAttachments(payload)
	if len(atts) != 2 {
		t.Fatalf("expected 2 attachments, got %d: %+v", len(atts), atts)
	}
	if atts[0].Filename != "report.pdf" || atts[0].AttachmentId != "att-1" || atts[0].Size != 1234 {
		t.Errorf("first attachment mismatch: %+v", atts[0])
	}
	if atts[1].Filename != "logo.png" || atts[1].AttachmentId != "att-2" || atts[1].MimeType != "image/png" {
		t.Errorf("nested attachment mismatch: %+v", atts[1])
	}
}

func TestCollectAttachments_NoneWhenBodyOnly(t *testing.T) {
	payload := &gmail.MessagePart{
		MimeType: "text/plain",
		Body:     &gmail.MessagePartBody{Data: "aGk="},
	}
	if atts := collectAttachments(payload); len(atts) != 0 {
		t.Errorf("expected no attachments, got %+v", atts)
	}
}

func TestHasForwardPrefix(t *testing.T) {
	cases := map[string]bool{
		"Fwd: Hello":  true,
		"fwd: hello":  true,
		"FW: hello":   true,
		"  Fwd: hi":   true,
		"Re: hello":   false,
		"Hello world": false,
		"":            false,
	}
	for in, want := range cases {
		if got := hasForwardPrefix(in); got != want {
			t.Errorf("hasForwardPrefix(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestOrDash(t *testing.T) {
	if orDash("") != "-" {
		t.Errorf("orDash(\"\") should be '-'")
	}
	if orDash("x") != "x" {
		t.Errorf("orDash(\"x\") should be 'x'")
	}
}
