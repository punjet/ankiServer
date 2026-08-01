package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/punjet/ankiserver/internal/ai"
	"github.com/punjet/ankiserver/internal/buffer"
	"github.com/punjet/ankiserver/internal/grammar"
)

// GrammarRequest is the body of POST /grammar.
type GrammarRequest struct {
	Text      string `json:"text"`
	SourceURL string `json:"source_url"`
}

// GrammarResponse is returned after analysis and buffering.
type GrammarResponse struct {
	ErrorsFound   int              `json:"errors_found"`
	TextCorrected string           `json:"text_corrected"`
	CardsAdded    int              `json:"cards_added"`
	Cards         []ai.GrammarCard `json:"cards"`
}

// Grammar handles POST /grammar — analyzes English text, generates Anki cards.
func Grammar(
	buf *buffer.Buffer,
	getAI func() *ai.Client,
	analyzer *grammar.Analyzer,
	maxTextLength int,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req GrammarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}

		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
			return
		}

		// Unicode-aware length check.
		runes := []rune(req.Text)
		if len(runes) > maxTextLength {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":      "text_too_long",
				"detail":     "text exceeds maximum allowed length",
				"max_length": maxTextLength,
				"your_length": len(runes),
			})
			return
		}

		// Minimum length guard — single words are handled by /translate.
		words := strings.Fields(req.Text)
		if len(words) < 3 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "text_too_short",
				"detail": "minimum 3 words required — use POST /translate for single words",
			})
			return
		}

		aiClient := getAI()
		if aiClient == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "OpenAI key not configured — set OPENAI_API_KEY environment variable or POST to /config",
			})
			return
		}

		logger.Info("🧠 Grammar analysis started", "words", len(words), "preview", truncate(req.Text, 60))

		result, err := aiClient.Analyze(req.Text)
		if err != nil {
			statusCode := http.StatusInternalServerError
			if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "invalid") {
				statusCode = http.StatusUnauthorized
			} else if strings.Contains(err.Error(), "429") {
				statusCode = http.StatusTooManyRequests
			}
			logger.Error("💥 AI analysis failed", "err", err)
			writeJSON(w, statusCode, map[string]string{"error": err.Error()})
			return
		}

		logger.Info("✅ Analysis complete",
			"errors_found", result.ErrorsFound,
			"cards_generated", len(result.Cards),
		)

		// Nothing to save if text was correct.
		if len(result.Cards) == 0 {
			writeJSON(w, http.StatusOK, GrammarResponse{
				ErrorsFound:   0,
				TextCorrected: result.TextCorrected,
				CardsAdded:    0,
				Cards:         []ai.GrammarCard{},
			})
			return
		}

		// Build and save notes to buffer.
		notes := analyzer.BuildNotes(result, req.Text, req.SourceURL)
		for _, note := range notes {
			if err := buf.Append(note); err != nil {
				logger.Error("buffer append failed", "err", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save cards"})
				return
			}
		}

		logger.Info("💾 Grammar cards saved to buffer", "count", len(notes))

		writeJSON(w, http.StatusOK, GrammarResponse{
			ErrorsFound:   result.ErrorsFound,
			TextCorrected: result.TextCorrected,
			CardsAdded:    len(notes),
			Cards:         result.Cards,
		})
	}
}
