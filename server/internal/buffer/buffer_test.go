package buffer

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestBuffer_AppendReadAll(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "buffer.json")
	
	b, err := New(filePath)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}
	
	note1 := Note{"_buf_id": "1", "fields": map[string]any{"Front": "hello"}}
	if err := b.Append(note1); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	
	notes, err := b.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0]["_buf_id"] != "1" {
		t.Errorf("expected note ID '1', got %v", notes[0]["_buf_id"])
	}
}

func TestBuffer_Replace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "buffer.json")
	
	b, err := New(filePath)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}
	
	notes := []Note{
		{"_buf_id": "1"},
		{"_buf_id": "2"},
	}
	if err := b.Replace(notes); err != nil {
		t.Fatalf("Replace failed: %v", err)
	}
	
	read, err := b.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(read) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(read))
	}
}

func TestBuffer_UpdateNote(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "buffer.json")
	
	b, err := New(filePath)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}
	
	_ = b.Append(Note{"_buf_id": "1", "data": "old"})
	_ = b.Append(Note{"_buf_id": "2", "data": "ignore"})
	
	err = b.UpdateNote("1", func(n Note) {
		n["data"] = "new"
	})
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}
	
	notes, _ := b.ReadAll()
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	
	if notes[0]["data"] != "new" {
		t.Errorf("expected 'new', got %v", notes[0]["data"])
	}
}

func TestBuffer_Concurrent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "buffer.json")
	
	b, err := New(filePath)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = b.Append(Note{"_buf_id": "test"})
			_, _ = b.ReadAll()
		}(i)
	}
	wg.Wait()
	
	notes, _ := b.ReadAll()
	if len(notes) != 100 {
		t.Errorf("expected 100 notes, got %d", len(notes))
	}
}
