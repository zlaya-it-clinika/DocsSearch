package httpapi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"docssearch/core/internal/contacts"
	"docssearch/core/internal/livestore"
	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

var fragmentNumber = regexp.MustCompile(`\d+`)

type chatTurn struct {
	Role string
	Text string
}

func build_LiveAssistantReply(chatter interface {
	Complete_Chat(messages []ollama.ChatMessage) (string, error)
}, question string, history []chatTurn, visible []livestore.SimilarChunk, closed []livestore.SimilarChunk) (assistantReply, []shownDocument, error) {
	kind, selected := livestore.Choose_LiveReplyKind(visible, closed)
	if kind == texts.KindFallback {
		return reply_WithContacts(build_PlainReply(texts.KindFallback, texts.FallbackText), nil), []shownDocument{}, nil
	}
	if kind == texts.KindAccessDenied {
		return build_PlainReply(texts.KindAccessDenied, texts.AccessDeniedText), []shownDocument{}, nil
	}
	if chatter == nil {
		return assistantReply{}, nil, livestore.ErrEmbedUnavailable
	}
	if kind == texts.KindClarify {
		text, err := chatter.Complete_Chat(build_ChatMessages(texts.LiveClarifyPrompt, question, history, selected))
		if err != nil || strings.TrimSpace(text) == "" {
			return assistantReply{}, nil, livestore.ErrEmbedUnavailable
		}
		return build_PlainReply(texts.KindClarify, strings.TrimSpace(text)), []shownDocument{}, nil
	}
	selectText, err := chatter.Complete_Chat(build_ChatMessages(texts.LiveSelectPrompt, question, history, selected))
	if err != nil {
		return assistantReply{}, nil, livestore.ErrEmbedUnavailable
	}
	// Пустой список номеров - отказ. Второй вызов модели не делается.
	selected = chunks_AtIndexes(selected, parse_FragmentNumbers(selectText, len(selected)))
	if len(selected) == 0 {
		return reply_WithContacts(build_PlainReply(texts.KindFallback, texts.FallbackText), nil), []shownDocument{}, nil
	}
	text, err := chatter.Complete_Chat(build_ChatMessages(texts.LiveAnswerPrompt, question, history, selected))
	if err != nil || strings.TrimSpace(text) == "" {
		return assistantReply{}, nil, livestore.ErrEmbedUnavailable
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
			DocumentID:   item.DocumentID,
			Scope:        item.Scope,
		})
		documents = append(documents, shownDocument{
			DocumentID: item.DocumentID,
			Title:      item.Document,
			Paragraph:  item.Paragraph,
			Scope:      item.Scope,
		})
	}
	return reply_WithContacts(reply, nil), documents, nil
}

func parse_FragmentNumbers(raw string, count int) []int {
	trimmed := strings.TrimSpace(raw)
	folded := strings.Trim(strings.ToLower(trimmed), ".,!;: ")
	if trimmed == "" || folded == texts.BlankSkipNo {
		return nil
	}
	matches := fragmentNumber.FindAllString(trimmed, -1)
	if len(matches) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, match := range matches {
		number, convError := strconv.Atoi(match)
		if convError != nil || number < 1 || number > count || seen[number] {
			continue
		}
		seen[number] = true
		indexes = append(indexes, number-1)
	}
	return indexes
}

func chunks_AtIndexes(chunks []livestore.SimilarChunk, indexes []int) []livestore.SimilarChunk {
	chosen := map[int]bool{}
	for _, index := range indexes {
		chosen[index] = true
	}
	picked := make([]livestore.SimilarChunk, 0, len(indexes))
	for index, chunk := range chunks {
		if chosen[index] {
			picked = append(picked, chunk)
		}
	}
	return picked
}

func contacts_ForText(text, question string) []replyContact {
	return contacts_FromEntries(contacts.Parse_Entries(text), question)
}

func contacts_FromEntries(entries []contacts.Entry, question string) []replyContact {
	entry, ok := contacts.Choose_Entry(entries, question)
	if !ok {
		return []replyContact{}
	}
	return []replyContact{{Department: entry.Department, Channel: entry.Channel}}
}

func reply_WithContacts(reply assistantReply, list []replyContact) assistantReply {
	if reply.Kind != texts.KindAnswer && reply.Kind != texts.KindFallback {
		return reply
	}
	if list == nil {
		list = []replyContact{}
	}
	reply.Contacts = &list
	return reply
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
