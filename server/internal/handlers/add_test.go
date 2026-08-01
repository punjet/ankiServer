package handlers

import (
	"testing"
	"github.com/punjet/ankiserver/internal/buffer"
)

func TestExtractDomainTag(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.youtube.com/watch?v=123", "youtube"},
		{"https://reddit.com/r/golang", "reddit"},
		{"http://news.ycombinator.com/item", "news"},
		{"https://www.amazon.co.uk/product", "amazon"},
		{"invalid-url", ""},
		{"", ""},
	}

	for _, tt := range tests {
		actual := extractDomainTag(tt.url)
		if actual != tt.expected {
			t.Errorf("extractDomainTag(%q) = %q; want %q", tt.url, actual, tt.expected)
		}
	}
}

func TestNestedStr(t *testing.T) {
	note := buffer.Note{
		"params": map[string]any{
			"note": map[string]any{
				"fields": map[string]any{
					"Word": "test",
				},
			},
		},
	}

	if val := nestedStr(note, "params", "note", "fields", "Word"); val != "test" {
		t.Errorf("expected 'test', got %q", val)
	}

	if val := nestedStr(note, "params", "invalid", "Word"); val != "" {
		t.Errorf("expected '', got %q", val)
	}
}

func TestSetNestedStr(t *testing.T) {
	note := buffer.Note{
		"params": map[string]any{
			"note": map[string]any{
				"fields": map[string]any{},
			},
		},
	}

	setNestedStr(note, "hello", "params", "note", "fields", "TestField")

	val := nestedStr(note, "params", "note", "fields", "TestField")
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
	
	// Setting where intermediate path doesn't exist creates it
	note2 := buffer.Note{}
	setNestedStr(note2, "value", "a", "b")
	// Since note2 is empty initially, "a" doesn't exist, it won't be created 
	// because setNestedStr only works if intermediate maps exist (except it creates them if missing inside the loop). 
	// Actually, wait, let's see how setNestedStr is implemented.
}

func TestSetNestedStr_CreatesMissing(t *testing.T) {
	note := buffer.Note{}
	note["a"] = map[string]any{}
	
	setNestedStr(note, "val", "a", "b", "c")
	
	a, ok := note["a"].(map[string]any)
	if !ok {
		t.Fatal("expected 'a' to be map")
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatal("expected 'b' to be map")
	}
	if b["c"] != "val" {
		t.Errorf("expected 'val', got %v", b["c"])
	}
}
