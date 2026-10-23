package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"docssearch/core/internal/livestore"
	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

type chatTurn struct {
	Role string
	Text string
}

func build_LiveAssistantReply(chatter interface {
	Complete_Chat(messages []ollama.ChatMessage) (string, error)
}, question string, history []chatTurn, chunks []livestore.SimilarChunk) (assistantReply, []shownDocument, error) {
	kind, selected := livestore.Choose_LiveReplyKind(chunks)
	if kind == texts.KindFallback {
		return build_PlainReply(texts.KindFallback, texts.FallbackText), []shownDocument{}, nil
	}
	if chatter == nil {
		return assistantReply{}, nil, livestore.ErrEmbedUnavailable
	}
	var prompt string
	if kind == texts.KindClarify {
		prompt = texts.LiveClarifyPrompt
	} else {
		prompt = texts.LiveAnswerPrompt
	}
	text, err := chatter.Complete_Chat(build_ChatMessages(prompt, question, history, selected))
	if err != nil || strings.TrimSpace(text) == "" {
		return assistantReply{}, nil, livestore.ErrEmbedUnavailable
	}
	if kind == texts.KindClarify {
		return build_PlainReply(texts.KindClarify, strings.TrimSpace(text)), []shownDocument{}, nil
	}
	reply := build_PlainReply(texts.KindAnswer, strings.TrimSpace(text))
	reply.RelevancePercent = livestore.Relevance_Percent(selected[0])
	citations := livestore.Build_ReplyCitations(selected)
	reply.Citations = make([]citation, 0, len(citations))
	documents := make([]shownDocument, 0, len(citations))
	for _, item := range citations {
		reply.Citations = append(reply.Citations, citation{
			Document:     item.Document,
			Edition:      item.Edition,
			Section:      item.Section,
			Paragraph:    item.Paragraph,
			Appendix:     item.Appendix,
			Quote:        item.Quote,
			OriginalLink: item.OriginalLink,
		})
		documents = append(documents, shownDocument{
			DocumentID: item.DocumentID,
			Title:      item.Document,
			Paragraph:  item.Paragraph,
		})
	}
	return reply, documents, nil
}

func build_ChatMessages(prompt, question string, history []chatTurn, chunks []livestore.SimilarChunk) []ollama.ChatMessage {
	messages := make([]ollama.ChatMessage, 0, len(history)+2)
	messages = append(messages, ollama.ChatMessage{Role: "system", Content: prompt})
	for _, turn := range history {
		messages = append(messages, ollama.ChatMessage{Role: turn.Role, Content: turn.Text})
	}
	messages = append(messages, ollama.ChatMessage{
		Role:    "user",
		Content: format_QuestionWithChunks(question, chunks),
	})
	return messages
}

func format_QuestionWithChunks(question string, chunks []livestore.SimilarChunk) string {
	var builder strings.Builder
	builder.WriteString(texts.LiveQuestionLabel)
	builder.WriteString(": ")
	builder.WriteString(question)
	builder.WriteString("\n\n")
	builder.WriteString(texts.LiveFragmentsLabel)
	builder.WriteString(":\n")
	for index, chunk := range chunks {
		if index > 0 {
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "%d. %s: %s", index+1, texts.LiveDocumentLabel, chunk.Title)
		if chunk.Edition != "" {
			fmt.Fprintf(&builder, ". %s: %s", texts.LiveEditionLabel, chunk.Edition)
		}
		if chunk.Section != "" {
			fmt.Fprintf(&builder, ". %s: %s", texts.LiveSectionLabel, chunk.Section)
		}
		if chunk.Paragraph != "" {
			fmt.Fprintf(&builder, ". %s: %s", texts.LiveParagraphLabel, chunk.Paragraph)
		}
		if chunk.Appendix != "" {
			fmt.Fprintf(&builder, ". %s: %s", texts.LiveAppendixLabel, chunk.Appendix)
		}
		builder.WriteString("\n")
		builder.WriteString(chunk.Content)
		builder.WriteString("\n")
	}
	return builder.String()
}

func read_ChatHistory(rawMessages [][]byte) []chatTurn {
	turns := []chatTurn{}
	for _, raw := range rawMessages {
		var stored struct {
			Role string `json:"role"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &stored); err != nil {
			continue
		}
		if stored.Role != "user" && stored.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(stored.Text) == "" {
			continue
		}
		turns = append(turns, chatTurn{Role: stored.Role, Text: stored.Text})
	}
	return turns
}
