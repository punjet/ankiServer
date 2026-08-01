package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/punjet/ankiserver/internal/deepl"
)

// charState tracks monthly DeepL character usage in memory (backed by a simple counter).
type charState struct {
	mu    sync.Mutex
	month string
	count int
}

var chars charState

func init() {
	chars.month = time.Now().Format("2006-01")
}

// addChars adds n to the monthly counter (resets on month change).
func addChars(n int) {
	chars.mu.Lock()
	defer chars.mu.Unlock()
	month := time.Now().Format("2006-01")
	if chars.month != month {
		chars.month = month
		chars.count = 0
	}
	chars.count += n
}

func getCharStats() (string, int) {
	chars.mu.Lock()
	defer chars.mu.Unlock()
	return chars.month, chars.count
}

// Translate handles POST /translate — translates text via DeepL.
func Translate(getClient func() *deepl.Client, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text    string `json:"text"`
			Context string `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no text"})
			return
		}

		client := getClient()
		if client == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "DeepL key not set"})
			return
		}

		result, err := client.Translate(body.Text, body.Context)
		if err != nil {
			logger.Error("💥 DeepL error", "err", err)
			status := http.StatusInternalServerError
			msg := err.Error()
			if msg == "invalid DeepL key (403)" {
				status = http.StatusUnauthorized
				msg = "invalid DeepL key"
			}
			writeJSON(w, status, map[string]string{"error": msg})
			return
		}

		addChars(result.CharCount)
		logger.Info("🌐 Translated", "word", body.Text[:min(len(body.Text), 30)], "chars", result.CharCount)

		resp := map[string]string{"translation": result.WordTranslation}
		if result.ContextTranslation != "" {
			resp["context_translation"] = result.ContextTranslation
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// CharStats returns current monthly usage (used by /config endpoint).
func CharStats() (string, string) {
	month, count := getCharStats()
	return month, strconv.Itoa(count)
}
