package grammar

import (
	"fmt"
	"time"

	"github.com/punjet/ankiserver/internal/ai"
	"github.com/punjet/ankiserver/internal/buffer"
)

const (
	grammarModelFields = "Front\x1fBack\x1fRuleTag\x1fOriginalText\x1fCorrectedText\x1fDifficulty\x1fDateAdded\x1fErrorType"
)

// GrammarFields maps to the GrammarErrors Anki note model fields.
type GrammarFields struct {
	Front         string
	Back          string
	RuleTag       string
	OriginalText  string
	CorrectedText string
	Difficulty    string
	DateAdded     string
	ErrorType     string
}

// CardResult is what we return to the caller (HTTP handler + buffer).
type CardResult struct {
	Card    ai.GrammarCard
	BufNote buffer.Note
}

// Analyzer converts AI results into buffer-ready Anki notes.
type Analyzer struct {
	deckName  string
	modelName string
}

// New creates a new Analyzer.
func New(deckName, modelName string) *Analyzer {
	return &Analyzer{deckName: deckName, modelName: modelName}
}

// BuildNotes converts an AnalysisResult into a slice of buffer Notes.
// Each note is a fully-formed AnkiConnect addNote payload.
func (a *Analyzer) BuildNotes(result *ai.AnalysisResult, originalText, sourceURL string) []buffer.Note {
	today := time.Now().Format("2006-01-02")
	notes := make([]buffer.Note, 0, len(result.Cards))

	for i, card := range result.Cards {
		bufID := fmt.Sprintf("grammar_%d_%d", time.Now().UnixMilli(), i)

		tags := []string{"grammar", card.RuleTag}
		if card.Difficulty != "" {
			tags = append(tags, "difficulty_"+card.Difficulty)
		}

		note := buffer.Note{
			"action":    "addNote",
			"version":   6,
			"sourceUrl": sourceURL,
			"_buf_id":   bufID,
			"_type":     "grammar",
			"params": map[string]any{
				"note": map[string]any{
					"deckName":  a.deckName,
					"modelName": a.modelName,
					"fields": map[string]string{
						"Front":         card.Front,
						"Back":          card.Back,
						"RuleTag":       card.RuleTag,
						"OriginalText":  originalText,
						"CorrectedText": result.TextCorrected,
						"Difficulty":    card.Difficulty,
						"DateAdded":     today,
						"ErrorType":     card.Type,
					},
					"tags":    tags,
					"options": map[string]bool{"allowDuplicate": false},
				},
			},
		}
		notes = append(notes, note)
	}

	return notes
}

// RequiredFields returns the list of fields the GrammarErrors model must have.
func RequiredFields() []string {
	return []string{
		"Front", "Back", "RuleTag", "OriginalText",
		"CorrectedText", "Difficulty", "DateAdded", "ErrorType",
	}
}
