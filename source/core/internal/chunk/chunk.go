package chunk

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const chunkRuneLimit = 800

var (
	sectionHeading   = regexp.MustCompile(`(?i)^\s*раздел\s+(\S+)`)
	paragraphHeading = regexp.MustCompile(`(?i)^\s*пункт\s+(\S+)`)
	appendixHeading  = regexp.MustCompile(`(?i)^\s*приложение\s+(\S+)`)
	numberedHeading  = regexp.MustCompile(`^\s*(\d+(?:\.\d+)+)\b`)
)

var ErrUnsupportedFile = errors.New("unsupported file")

type TextChunk struct {
	Section   string
	Paragraph string
	Appendix  string
	Content   string
}

func Extract_DocumentText(fileName string, body []byte) (string, error) {
	switch strings.ToLower(path.Ext(fileName)) {
	case ".txt":
		return string(body), nil
	case ".pdf":
		return extract_PdfText(body)
	case ".docx":
		return extract_DocxText(body)
	default:
		return "", ErrUnsupportedFile
	}
}

func extract_PdfText(body []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for pageNumber := 1; pageNumber <= reader.NumPage(); pageNumber++ {
		page := reader.Page(pageNumber)
		if page.V.IsNull() {
			continue
		}
		plain, plainError := page.GetPlainText(nil)
		if plainError != nil {
			return "", plainError
		}
		builder.WriteString(plain)
		builder.WriteByte('\n')
	}
	return builder.String(), nil
}

func extract_DocxText(body []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}
	var documentXML *zip.File
	for _, file := range archive.File {
		if file.Name == "word/document.xml" {
			documentXML = file
			break
		}
	}
	if documentXML == nil {
		return "", errors.New("docx has no document.xml")
	}
	reader, err := documentXML.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(reader)
	var builder strings.Builder
	for {
		token, tokenError := decoder.Token()
		if tokenError == io.EOF {
			break
		}
		if tokenError != nil {
			return "", tokenError
		}
		switch node := token.(type) {
		case xml.CharData:
			builder.Write(node)
		case xml.EndElement:
			if node.Name.Local == "p" {
				builder.WriteByte('\n')
			}
		}
	}
	return builder.String(), nil
}

func Split_DocumentText(raw string) []TextChunk {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	var chunks []TextChunk
	var section, paragraph, appendix string
	var current strings.Builder
	structured := false
	flush := func() {
		text := strings.TrimSpace(current.String())
		current.Reset()
		if text == "" {
			return
		}
		chunks = append(chunks, TextChunk{
			Section:   section,
			Paragraph: paragraph,
			Appendix:  appendix,
			Content:   text,
		})
	}
	for _, line := range strings.Split(normalized, "\n") {
		kind, value, remainder, isHeading := classify_HeadingLine(line)
		if !isHeading {
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}
		structured = true
		flush()
		switch kind {
		case "section":
			section = value
			paragraph = ""
		case "appendix":
			appendix = value
			section = ""
			paragraph = ""
		default:
			paragraph = value
		}
		if remainder != "" {
			current.WriteString(remainder)
			current.WriteByte('\n')
		}
	}
	flush()
	if !structured {
		return split_FixedRunes(normalized, chunkRuneLimit)
	}
	return chunks
}

// Строка с разделом, пунктом или приложением задаёт метаданные фрагмента.
// Текст без такой структуры режется кусками фиксированной длины.
func classify_HeadingLine(line string) (kind, value, remainder string, ok bool) {
	trimmed := strings.TrimSpace(line)
	patterns := []struct {
		kind    string
		pattern *regexp.Regexp
	}{
		{kind: "section", pattern: sectionHeading},
		{kind: "paragraph", pattern: paragraphHeading},
		{kind: "appendix", pattern: appendixHeading},
		{kind: "paragraph", pattern: numberedHeading},
	}
	for _, candidate := range patterns {
		match := candidate.pattern.FindStringSubmatchIndex(trimmed)
		if match == nil {
			continue
		}
		value = strings.Trim(trimmed[match[2]:match[3]], ".:)")
		remainder = strings.TrimSpace(trimmed[match[1]:])
		return candidate.kind, value, remainder, true
	}
	return "", "", "", false
}

func split_FixedRunes(raw string, limit int) []TextChunk {
	runes := []rune(strings.TrimSpace(raw))
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]TextChunk, 0, (len(runes)+limit-1)/limit)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		piece := strings.TrimSpace(string(runes[start:end]))
		if piece == "" {
			continue
		}
		chunks = append(chunks, TextChunk{Content: piece})
	}
	return chunks
}

func runeCount(text string) int {
	return utf8.RuneCountInString(text)
}
