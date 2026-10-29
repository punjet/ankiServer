package grammar

import (
	"strings"
	"testing"
)

func TestFrontWithContext_AddsMissingSentence(t *testing.T) {
	got := FrontWithContext("What should replace 'for test' in this sentence?", "for test",
		"Hello there. I am waiting for test results tomorrow! Bye.")
	want := "<i>I am waiting for test results tomorrow!</i>"
	if !strings.HasSuffix(got, want) {
		t.Fatalf("got %q, want suffix %q", got, want)
	}
}

func TestFrontWithContext_KeepsFrontThatQuotesSentence(t *testing.T) {
	front := `Complete correctly: "I ___ to the store yesterday"`
	got := FrontWithContext(front, "goed", "I goed to the store yesterday.")
	if strings.Contains(got, "<i>") {
		t.Fatalf("sentence appended twice: %q", got)
	}
}

func TestFrontWithContext_UnknownFragment(t *testing.T) {
	got := FrontWithContext("Fix it", "nowhere", "Some text.")
	if got != "Fix it" {
		t.Fatalf("got %q", got)
	}
}

func TestToHTML_KeepsLineBreaks(t *testing.T) {
	got := toHTML("✅ for a test\n\n📌 Rule: use 'a'")
	if !strings.Contains(got, "for a test<br><br>📌") {
		t.Fatalf("got %q", got)
	}
}
