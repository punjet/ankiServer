package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

// Client wraps the OpenAI Chat Completions API.
type Client struct {
	apiKey  string
	model   string
	maxCards int
	http    *http.Client
}

// New creates a new OpenAI client.
func New(apiKey, model string, maxCards int) *Client {
	return &Client{
		apiKey:   apiKey,
		model:    model,
		maxCards: maxCards,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// GrammarCard is a single Anki card produced by the AI.
type GrammarCard struct {
	Type          string `json:"type"`
	ErrorFragment string `json:"error_fragment"`
	Front         string `json:"front"`
	Back          string `json:"back"`
	RuleTag       string `json:"rule_tag"`
	Difficulty    string `json:"difficulty"`
}

// AnalysisResult is the structured response from the AI.
type AnalysisResult struct {
	ErrorsFound   int           `json:"errors_found"`
	TextCorrected string        `json:"text_corrected"`
	Cards         []GrammarCard `json:"cards"`
}

// Analyze sends text to OpenAI and returns structured grammar analysis.
func (c *Client) Analyze(text string) (*AnalysisResult, error) {
	sysprompt := BuildSystemPrompt(c.maxCards)
	userprompt := BuildUserPrompt(text)

	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": sysprompt},
			{"role": "user", "content": userprompt},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.2, // Low temperature → consistent, deterministic cards
		"max_tokens":      2048,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", openAIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read body: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid OpenAI API key (401)")
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("OpenAI rate limit exceeded (429)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(raw[:min(300, len(raw))]))
	}

	// Parse outer OpenAI envelope.
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("ai: unmarshal envelope: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("openai error: %s", envelope.Error.Message)
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("ai: empty choices in response")
	}

	content := envelope.Choices[0].Message.Content

	// Parse the inner JSON the model produced.
	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("ai: unmarshal model output: %w\nraw content: %s", err, content[:min(500, len(content))])
	}

	// Enforce max cards server-side as a safety net.
	if len(result.Cards) > c.maxCards {
		result.Cards = result.Cards[:c.maxCards]
	}

	return &result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
