package eval

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed questions.json
var questionsRaw []byte

type Question struct {
	Question  string `json:"question"`
	Paragraph string `json:"paragraph,omitempty"`
	Fallback  bool   `json:"fallback,omitempty"`
}

func Load_Questions() ([]Question, error) {
	var questions []Question
	if err := json.Unmarshal(questionsRaw, &questions); err != nil {
		return nil, err
	}
	return questions, nil
}

func Paragraph_Matches(paragraph, expected string) bool {
	if paragraph == "" || expected == "" {
		return false
	}
	return paragraph == expected || strings.HasPrefix(paragraph, expected+".")
}
