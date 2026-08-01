package anki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the AnkiConnect HTTP API.
type Client struct {
	url  string
	http *http.Client
}

// New creates a new AnkiConnect client.
func New(url string) *Client {
	return &Client{
		url: url,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Request is the generic AnkiConnect request envelope.
type Request struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
	Params  any    `json:"params,omitempty"`
}

// Response is the generic AnkiConnect response envelope.
type Response struct {
	Result any    `json:"result"`
	Error  *string `json:"error"`
}

// Do sends a request and decodes the response.
func (c *Client) Do(action string, params any) (*Response, error) {
	payload := Request{
		Action:  action,
		Version: 6,
		Params:  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anki: marshal: %w", err)
	}

	resp, err := c.http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anki: post: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anki: read body: %w", err)
	}

	var result Response
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("anki: unmarshal: %w", err)
	}
	return &result, nil
}

// DoRaw sends a pre-built payload (from the buffer) directly to AnkiConnect.
func (c *Client) DoRaw(payload any) (*Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anki: marshal raw: %w", err)
	}

	resp, err := c.http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anki: post raw: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anki: read body raw: %w", err)
	}

	// AnkiConnect can return a plain number (note ID) instead of a JSON object.
	var result Response
	if err := json.Unmarshal(raw, &result); err != nil {
		// Try parsing as a bare number.
		var id float64
		if err2 := json.Unmarshal(raw, &id); err2 == nil {
			result.Result = id
			return &result, nil
		}
		return nil, fmt.Errorf("anki: unmarshal raw: %w", err)
	}
	return &result, nil
}

// FindNotes returns note IDs matching the given query.
func (c *Client) FindNotes(query string) ([]int64, error) {
	res, err := c.Do("findNotes", map[string]string{"query": query})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("anki: findNotes error: %s", *res.Error)
	}
	// result is []interface{} of float64
	raw, _ := json.Marshal(res.Result)
	var ids []int64
	json.Unmarshal(raw, &ids)
	return ids, nil
}

// NoteField holds field value from notesInfo.
type NoteField struct {
	Value string `json:"value"`
	Order int    `json:"order"`
}

// NoteInfo holds info about a single note.
type NoteInfo struct {
	NoteID int64                `json:"noteId"`
	Fields map[string]NoteField `json:"fields"`
}

// NotesInfo returns detailed info about the given note IDs.
func (c *Client) NotesInfo(ids []int64) ([]NoteInfo, error) {
	res, err := c.Do("notesInfo", map[string]any{"notes": ids})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(res.Result)
	var info []NoteInfo
	json.Unmarshal(raw, &info)
	return info, nil
}

// UpdateNoteFields updates specific fields on an existing note.
func (c *Client) UpdateNoteFields(noteID int64, fields map[string]string) error {
	_, err := c.Do("updateNoteFields", map[string]any{
		"note": map[string]any{
			"id":     noteID,
			"fields": fields,
		},
	})
	return err
}

// EnsureModelFields checks that all required fields exist in the model and
// logs warnings for any that are missing (auto-add requires Anki to be open).
func (c *Client) EnsureModelFields(modelName string, required []string) error {
	res, err := c.Do("modelFieldNames", map[string]string{"modelName": modelName})
	if err != nil {
		return err
	}
	if res.Error != nil {
		return fmt.Errorf(*res.Error)
	}
	raw, _ := json.Marshal(res.Result)
	var existing []string
	json.Unmarshal(raw, &existing)

	existingSet := make(map[string]bool, len(existing))
	for _, f := range existing {
		existingSet[f] = true
	}

	for _, f := range required {
		if !existingSet[f] {
			c.Do("addNoteField", map[string]any{
				"modelName": modelName,
				"fieldName": f,
				"index":     nil,
			})
		}
	}
	return nil
}
