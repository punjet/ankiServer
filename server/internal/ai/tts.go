package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openAITTSURL = "https://api.openai.com/v1/audio/speech"

// TextToSpeech calls the OpenAI TTS API and returns the audio as a base64 encoded string.
func (c *Client) TextToSpeech(text string, voice string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("empty text")
	}
	if voice == "" {
		voice = "alloy" // Default voice
	}

	payload := map[string]any{
		"model":          "tts-1",
		"input":          text,
		"voice":          voice,
		"response_format": "mp3",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("tts: marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", openAITTSURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tts: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("tts: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tts: error %d: %s", resp.StatusCode, string(raw[:min(300, len(raw))]))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("tts: read audio data: %w", err)
	}

	base64Str := base64.StdEncoding.EncodeToString(audioData)
	return base64Str, nil
}
