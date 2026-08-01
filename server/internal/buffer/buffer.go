package buffer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Note represents a raw AnkiConnect note payload stored in the buffer.
type Note map[string]any

// Buffer is a thread-safe, file-backed queue of Anki notes.
type Buffer struct {
	mu       sync.RWMutex
	filePath string
}

// New creates a new Buffer backed by the given file path.
// The parent directory is created if it does not exist.
func New(filePath string) (*Buffer, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, fmt.Errorf("buffer: create dir: %w", err)
	}
	return &Buffer{filePath: filePath}, nil
}

// Append adds a note to the end of the buffer and persists to disk.
func (b *Buffer) Append(note Note) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	notes, err := b.load()
	if err != nil {
		return err
	}
	notes = append(notes, note)
	return b.save(notes)
}

// ReadAll returns a snapshot of all notes in the buffer.
func (b *Buffer) ReadAll() ([]Note, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.load()
}

// Replace overwrites the buffer with the provided notes (used after sync).
func (b *Buffer) Replace(notes []Note) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.save(notes)
}

// UpdateNote finds a note by its _buf_id field and applies fn to it in-place.
func (b *Buffer) UpdateNote(bufID string, fn func(Note)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	notes, err := b.load()
	if err != nil {
		return err
	}
	for _, n := range notes {
		if id, _ := n["_buf_id"].(string); id == bufID {
			fn(n)
			break
		}
	}
	return b.save(notes)
}

// load reads the JSON file without holding the lock (caller must hold it).
func (b *Buffer) load() ([]Note, error) {
	data, err := os.ReadFile(b.filePath)
	if os.IsNotExist(err) {
		return []Note{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buffer: read: %w", err)
	}
	if len(data) == 0 {
		return []Note{}, nil
	}
	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		// Corrupted file — start fresh rather than crashing.
		return []Note{}, nil
	}
	return notes, nil
}

// save writes notes to a temp file and atomically renames it.
func (b *Buffer) save(notes []Note) error {
	data, err := json.MarshalIndent(notes, "", "    ")
	if err != nil {
		return fmt.Errorf("buffer: marshal: %w", err)
	}
	tmp := b.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("buffer: write tmp: %w", err)
	}
	if err := os.Rename(tmp, b.filePath); err != nil {
		return fmt.Errorf("buffer: rename: %w", err)
	}
	return nil
}
