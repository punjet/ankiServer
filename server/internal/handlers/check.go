package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/punjet/ankiserver/internal/anki"
)

// Check handles POST /check — checks if a word already exists in Anki.
func Check(ankiClient *anki.Client, deckName string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Word string `json:"word"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Word == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no word"})
			return
		}

		query := `Word:"` + body.Word + `" deck:` + deckName
		ids, err := ankiClient.FindNotes(query)
		if err != nil {
			logger.Error("❌ /check findNotes failed", "err", err)
			writeJSON(w, http.StatusOK, map[string]any{"exists": false, "seen_count": 0})
			return
		}
		if len(ids) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"exists": false, "seen_count": 0})
			return
		}

		infos, err := ankiClient.NotesInfo(ids[:1])
		if err != nil || len(infos) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"exists": true, "seen_count": 1})
			return
		}

		seenField := infos[0].Fields["SeenCount"].Value
		seen := parseIntOrDefault(seenField, 1)

		writeJSON(w, http.StatusOK, map[string]any{"exists": true, "seen_count": seen})
	}
}

// writeJSON is a shared helper to write a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
