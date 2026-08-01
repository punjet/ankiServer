package deepl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Client calls the DeepL translation API.
type Client struct {
	apiKey string
	apiURL string
	http   *http.Client
}

// New creates a DeepL client. The apiURL is chosen automatically based on
// whether the key ends with ":fx" (free tier).
func New(apiKey string) *Client {
	apiURL := "https://api.deepl.com/v2/translate"
	if strings.HasSuffix(apiKey, ":fx") {
		apiURL = "https://api-free.deepl.com/v2/translate"
	}
	return &Client{
		apiKey: apiKey,
		apiURL: apiURL,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// TranslateResult holds the DeepL translation result.
type TranslateResult struct {
	// WordTranslation is the translation of the single word/phrase.
	WordTranslation string
	// ContextTranslation is the translation of the surrounding sentence.
	// Empty when no context was provided.
	ContextTranslation string
	// CharCount is the number of characters billed to the account.
	CharCount int
}

var tagRe = regexp.MustCompile(`</?w>`)
var wTagRe = regexp.MustCompile(`<w>(.*?)</w>`)

// Translate translates text from English to Russian.
// If context is non-empty and different from text, a context-aware XML trick
// is used to get both the word translation and sentence translation in one call.
func (c *Client) Translate(text, context string) (*TranslateResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("deepl key not configured")
	}

	headers := map[string]string{
		"Authorization": "DeepL-Auth-Key " + c.apiKey,
		"Content-Type":  "application/json",
	}

	// Context-aware translation using XML tags.
	if context != "" && context != text {
		escapedText := regexp.QuoteMeta(text)
		re := regexp.MustCompile(escapedText)
		// Replace only the first occurrence.
		replaced := false
		taggedContext := re.ReplaceAllStringFunc(context, func(match string) string {
			if replaced {
				return match
			}
			replaced = true
			return "<w>" + text + "</w>"
		})

		payload := map[string]any{
			"text":         []string{taggedContext},
			"source_lang":  "EN",
			"target_lang":  "RU",
			"tag_handling": "xml",
		}

		translated, err := c.post(payload, headers)
		if err == nil && translated != "" {
			m := wTagRe.FindStringSubmatch(translated)
			if len(m) == 2 {
				wordTrans := m[1]
				ctxTrans := tagRe.ReplaceAllString(translated, "")
				return &TranslateResult{
					WordTranslation:    wordTrans,
					ContextTranslation: ctxTrans,
					CharCount:          len(taggedContext),
				}, nil
			}
		}
		// Fall through to plain translation if XML trick failed.
	}

	// Plain translation.
	payload := map[string]any{
		"text":        []string{text},
		"source_lang": "EN",
		"target_lang": "RU",
	}
	translated, err := c.post(payload, headers)
	if err != nil {
		return nil, err
	}
	return &TranslateResult{
		WordTranslation: translated,
		CharCount:       len(text),
	}, nil
}

type deeplResponse struct {
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
}

func (c *Client) post(payload any, headers map[string]string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("deepl: marshal: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("deepl: new request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("deepl: request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 403 {
		return "", fmt.Errorf("invalid DeepL key (403)")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("deepl error %d: %s", resp.StatusCode, string(raw[:min(200, len(raw))]))
	}

	var result deeplResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("deepl: unmarshal: %w", err)
	}
	if len(result.Translations) == 0 {
		return "", fmt.Errorf("deepl: empty response")
	}
	return result.Translations[0].Text, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
