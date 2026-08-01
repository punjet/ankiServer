package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/punjet/ankiserver/internal/buffer"
	aiClient "github.com/punjet/ankiserver/internal/ai"
)

// Add handles POST /add — saves a card to the buffer and returns immediately.
// It also launches a background goroutine to fetch TTS audio via OpenAI if configured.
func Add(buf *buffer.Buffer, getAI func() *aiClient.Client, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data buffer.Note
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "detail": "invalid JSON"})
			return
		}

		// Extract fields safely.
		word := nestedStr(data, "params", "note", "fields", "Word")
		if word == "" {
			word = "Unknown"
		}

		sourceURL, _ := data["sourceUrl"].(string)
		domainTag := extractDomainTag(sourceURL)
		today := time.Now().Format("2006-01-02")

		// Enrich fields.
		setNestedStr(data, today, "params", "note", "fields", "DateAdded")
		setNestedStr(data, sourceURL, "params", "note", "fields", "SourceURL")
		setNestedStr(data, "1", "params", "note", "fields", "SeenCount")
		setNestedStr(data, "", "params", "note", "fields", "Audio")
		setNestedStr(data, "", "params", "note", "fields", "Spelling")
		setNestedStr(data, "", "params", "note", "fields", "SpellingTranscript")

		// Add domain tag.
		if domainTag != "" {
			note, _ := data["params"].(map[string]any)
			if note == nil {
				note = map[string]any{}
				data["params"] = note
			}
			noteInner, _ := note["note"].(map[string]any)
			if noteInner == nil {
				noteInner = map[string]any{}
				note["note"] = noteInner
			}
			tags, _ := noteInner["tags"].([]any)
			hasDomain := false
			for _, t := range tags {
				if s, _ := t.(string); s == domainTag {
					hasDomain = true
					break
				}
			}
			if !hasDomain {
				noteInner["tags"] = append(tags, domainTag)
			}
		}

		// Unique buffer ID for future enrichment hooks.
		bufID := fmt.Sprintf("%s_%d", word, time.Now().UnixMilli())
		data["_buf_id"] = bufID

		if err := buf.Append(data); err != nil {
			logger.Error("buffer append failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			return
		}

		logger.Info("📥 Added to buffer", "word", word, "domain", domainTag, "url", truncate(sourceURL, 60))

		// Launch background TTS fetch
		if ai := getAI(); ai != nil {
			contextStr := nestedStr(data, "params", "note", "fields", "Context")
			go func(word, ctx, id string) {
				ttsText := word
				if ctx != "" {
					ttsText = word + ". " + ctx
				}
				logger.Info("Starting TTS background fetch", "word", word)
				base64Audio, err := ai.TextToSpeech(ttsText, "alloy")
				if err != nil {
					logger.Error("TTS failed", "word", word, "err", err)
					return
				}
				err = buf.UpdateNote(id, func(n buffer.Note) {
					n["_audio_base64"] = base64Audio
				})
				if err != nil {
					logger.Error("Failed to update note with audio", "word", word, "err", err)
				} else {
					logger.Info("TTS audio saved to buffer", "word", word)
				}
			}(word, contextStr, bufID)
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	}
}

// extractDomainTag converts "https://reddit.com/..." → "reddit".
func extractDomainTag(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := regexp.MustCompile(`^www\.`).ReplaceAllString(u.Hostname(), "")
	parts := strings.SplitN(host, ".", 2)
	return parts[0]
}

// nestedStr safely walks a Note for string["a"]["b"]["c"].
func nestedStr(note buffer.Note, keys ...string) string {
	var cur any = map[string]any(note)
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	s, _ := cur.(string)
	return s
}

// setNestedStr walks note down keys[0..n-2] and sets keys[n-1] = value.
func setNestedStr(note buffer.Note, value string, keys ...string) {
	var cur any = map[string]any(note)
	for i, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		if i == len(keys)-1 {
			m[k] = value
			return
		}
		next, exists := m[k]
		if !exists {
			child := map[string]any{}
			m[k] = child
			cur = child
		} else {
			cur = next
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
