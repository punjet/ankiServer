package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/punjet/ankiserver/internal/buffer"
)

// GetBuffer handles GET /buffer — returns all notes currently in the buffer.
func GetBuffer(buf *buffer.Buffer, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notes, err := buf.ReadAll()
		if err != nil {
			logger.Error("buffer read failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(notes); err != nil {
			logger.Error("failed to encode buffer", "err", err)
		}
	}
}

// DeleteBufferReq defines the body for DELETE /buffer.
type DeleteBufferReq struct {
	IDs []string `json:"ids"`
}

// DeleteBuffer handles DELETE /buffer — removes specific notes by their _buf_id.
func DeleteBuffer(buf *buffer.Buffer, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DeleteBufferReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if len(req.IDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "no ids provided"})
			return
		}

		// Convert slice to map for fast lookup
		toDelete := make(map[string]bool, len(req.IDs))
		for _, id := range req.IDs {
			toDelete[id] = true
		}

		notes, err := buf.ReadAll()
		if err != nil {
			logger.Error("buffer read failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			return
		}

		var remaining []buffer.Note
		deletedCount := 0

		for _, note := range notes {
			id, _ := note["_buf_id"].(string)
			if toDelete[id] {
				deletedCount++
			} else {
				remaining = append(remaining, note)
			}
		}

		if deletedCount > 0 {
			if err := buf.Replace(remaining); err != nil {
				logger.Error("buffer replace failed", "err", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
				return
			}
		}

		logger.Info("🧹 Buffer cleared items via API", "deleted", deletedCount, "remaining", len(remaining))
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"deleted": deletedCount,
			"remaining": len(remaining),
		})
	}
}
