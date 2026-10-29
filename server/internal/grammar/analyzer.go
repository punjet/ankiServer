package grammar

import (
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"

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
						"Front":         FrontWithContext(card.Front, card.ErrorFragment, originalText),
						"Back":          toHTML(card.Back),
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

// FrontWithContext makes sure the card front shows the learner's sentence: the model sometimes
// asks "What should replace 'X' in this sentence?" without quoting the sentence itself.
func FrontWithContext(front, fragment, originalText string) string {
	out := toHTML(front)
	sentence := sentenceAround(originalText, fragment)
	if sentence == "" || mentionsSentence(front, sentence, fragment) {
		return out
	}
	return out + "<br><br><i>" + toHTML(sentence) + "</i>"
}

// toHTML escapes model text for an Anki field and keeps its line breaks.
func toHTML(s string) string {
	return strings.ReplaceAll(html.EscapeString(strings.TrimSpace(s)), "\n", "<br>")
}

// sentenceAround returns the sentence of text that contains fragment, or "" if it isn't found.
func sentenceAround(text, fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	i := strings.Index(strings.ToLower(text), strings.ToLower(fragment))
	if i < 0 {
		return ""
	}
	start := strings.LastIndexAny(text[:i], ".!?\n") + 1
	end := len(text)
	if j := strings.IndexAny(text[i+len(fragment):], ".!?\n"); j >= 0 {
		end = i + len(fragment) + j + 1
	}
	return strings.TrimSpace(text[start:end])
}

// mentionsSentence reports whether front already carries most of the sentence's words
// besides the fragment (a quoted sentence or a fill-in-the-blank version of it).
func mentionsSentence(front, sentence, fragment string) bool {
	inFront := map[string]bool{}
	for _, w := range words(front) {
		inFront[w] = true
	}
	skip := map[string]bool{}
	for _, w := range words(fragment) {
		skip[w] = true
	}
	total, found := 0, 0
	for _, w := range words(sentence) {
		if skip[w] {
			continue
		}
		total++
		if inFront[w] {
			found++
		}
	}
	return total == 0 || found*2 >= total
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
}
