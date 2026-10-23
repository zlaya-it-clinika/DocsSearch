package chunk

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestSplit_DocumentTextUsesHeadings(t *testing.T) {
	raw := strings.Join([]string{
		"Раздел 1",
		"Общие положения",
		"Пункт 3.2 Текст пункта",
		"Приложение А",
		"Форма бланка",
	}, "\n")
	chunks := Split_DocumentText(raw)
	if len(chunks) != 3 {
		t.Fatalf("chunks %d %+v", len(chunks), chunks)
	}
	if chunks[0].Section != "1" || chunks[0].Paragraph != "" || chunks[0].Content != "Общие положения" {
		t.Fatalf("section %+v", chunks[0])
	}
	if chunks[1].Section != "1" || chunks[1].Paragraph != "3.2" || chunks[1].Content != "Текст пункта" {
		t.Fatalf("paragraph %+v", chunks[1])
	}
	if chunks[2].Appendix != "А" || chunks[2].Section != "" || chunks[2].Content != "Форма бланка" {
		t.Fatalf("appendix %+v", chunks[2])
	}
	numbered := Split_DocumentText("3.2 Регистрация\nДальше")
	if len(numbered) != 1 || numbered[0].Paragraph != "3.2" || numbered[0].Content != "Регистрация\nДальше" {
		t.Fatalf("numbered %+v", numbered)
	}
}

func TestSplit_DocumentTextFixedLengthWithoutHeadings(t *testing.T) {
	raw := strings.Repeat("а", chunkRuneLimit+1)
	chunks := Split_DocumentText(raw)
	if len(chunks) != 2 {
		t.Fatalf("chunks %d", len(chunks))
	}
	if runeCount(chunks[0].Content) != chunkRuneLimit || runeCount(chunks[1].Content) != 1 {
		t.Fatalf("sizes %d %d", runeCount(chunks[0].Content), runeCount(chunks[1].Content))
	}
	if chunks[0].Section != "" || chunks[0].Paragraph != "" || chunks[0].Appendix != "" {
		t.Fatalf("metadata %+v", chunks[0])
	}
}

func TestExtract_DocumentTextTxtAndDocx(t *testing.T) {
	plain, err := Extract_DocumentText("note.txt", []byte("Пункт 1.1 Строка"))
	if err != nil || !strings.Contains(plain, "Пункт 1.1") {
		t.Fatalf("txt %v %s", err, plain)
	}
	docxBody := build_Docx(t, "Раздел 4", "Содержание раздела")
	docxText, err := Extract_DocumentText("blank.docx", docxBody)
	if err != nil {
		t.Fatal(err)
	}
	chunks := Split_DocumentText(docxText)
	if len(chunks) != 1 || chunks[0].Section != "4" || chunks[0].Content != "Содержание раздела" {
		t.Fatalf("docx %+v text %q", chunks, docxText)
	}
	if _, err := Extract_DocumentText("sheet.xlsx", []byte("nope")); err != ErrUnsupportedFile {
		t.Fatalf("xlsx error %v", err)
	}
}

func build_Docx(t *testing.T, lines ...string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, line := range lines {
		document.WriteString(`<w:p><w:r><w:t>`)
		document.WriteString(line)
		document.WriteString(`</w:t></w:r></w:p>`)
	}
	document.WriteString(`</w:body></w:document>`)
	if _, err := file.Write([]byte(document.String())); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
