package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/punjet/ankiserver/internal/anki"
	"github.com/punjet/ankiserver/internal/buffer"
)

// Sync handles POST /sync — flushes the buffer into AnkiConnect.
func Sync(buf *buffer.Buffer, ankiClient *anki.Client, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notes, err := buf.ReadAll()
		if err != nil {
			logger.Error("buffer read failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			return
		}
		if len(notes) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "empty"})
			return
		}

		logger.Info("🔄 Starting sync", "count", len(notes))

		var remaining []buffer.Note
		success, duplicates := 0, 0

		for _, note := range notes {
			word := nestedStr(note, "params", "note", "fields", "Word")
			if word == "" {
				word = "Unknown"
			}

			// Strip internal _buf_id before sending to Anki.
			payload := copyNote(note)
			delete(payload, "_buf_id")

			res, err := ankiClient.DoRaw(payload)
			if err != nil {
				logger.Error("❗ Anki connection error", "word", word, "err", err)
				remaining = append(remaining, note)
				continue
			}

			// Determine success.
			switch {
			case res.Error == nil:
				logger.Info("✅ Synced", "word", word)
				success++

			case strings.Contains(strings.ToLower(*res.Error), "duplicate"):
				logger.Warn("⚠️ Duplicate — incrementing SeenCount", "word", word)
				incrementSeenCount(ankiClient, word, logger)
				duplicates++

			default:
				logger.Error("❌ Anki rejected", "word", word, "error", *res.Error)
				remaining = append(remaining, note)
			}
		}

		if err := buf.Replace(remaining); err != nil {
			logger.Error("buffer replace failed", "err", err)
		}

		logger.Info("🏁 Sync done", "success", success, "duplicates", duplicates, "remaining", len(remaining))
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "done",
			"added":      success,
			"duplicates": duplicates,
			"remaining":  len(remaining),
		})
	}
}

// incrementSeenCount finds the card and bumps its SeenCount field.
func incrementSeenCount(client *anki.Client, word string, logger *slog.Logger) {
	ids, err := client.FindNotes(`Word:"` + word + `" deck:WordsFromSafari`)
	if err != nil || len(ids) == 0 {
		return
	}
	infos, err := client.NotesInfo(ids[:1])
	if err != nil || len(infos) == 0 {
		return
	}
	field := infos[0].Fields["SeenCount"]
	seen := 1
	if n := parseIntOrDefault(field.Value, 1); n > 0 {
		seen = n
	}
	if err := client.UpdateNoteFields(ids[0], map[string]string{
		"SeenCount": intToStr(seen + 1),
	}); err != nil {
		logger.Error("increment SeenCount failed", "word", word, "err", err)
	} else {
		logger.Info("🔢 SeenCount incremented", "word", word, "new", seen+1)
	}
}

func copyNote(n buffer.Note) buffer.Note {
	out := make(buffer.Note, len(n))
	for k, v := range n {
		out[k] = v
	}
	return out
}

func parseIntOrDefault(s string, def int) int {
	var n int
	if _, err := (&struct{ v *int }{&n}), json.Unmarshal([]byte(s), &n); err == nil {
		return n
	}
	// Try sscanf-style.
	v := def
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			v = v*10 + int(ch-'0')
		}
	}
	return v
}

func intToStr(n int) string {
	return strings.TrimSpace(strings.Join([]string{""}, "") + func() string {
		b := make([]byte, 0, 10)
		if n == 0 {
			return "0"
		}
		tmp := n
		for tmp > 0 {
			b = append([]byte{byte('0' + tmp%10)}, b...)
			tmp /= 10
		}
		return string(b)
	}())
}
