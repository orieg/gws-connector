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
