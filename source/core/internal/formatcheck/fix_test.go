package formatcheck

import (
	"strings"
	"testing"

	"docssearch/core/internal/texts"
)

func TestFix_DocxFixesFormatting(t *testing.T) {
	format := must_Format(t)
	styles := `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi"/><w:sz w:val="24"/><w:lang w:val="ru-RU"/></w:rPr></w:rPrDefault></w:docDefaults>`
	section := `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1134" w:right="850" w:bottom="1134" w:left="1134" w:header="708" w:footer="708" w:gutter="0"/><w:cols w:space="708"/></w:sectPr>`
	body := `<w:p><w:pPr><w:spacing w:after="160" w:line="240" w:lineRule="auto"/><w:ind w:left="0"/></w:pPr>` +
		`<w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:b/><w:lang w:val="ru-RU"/></w:rPr><w:t xml:space="preserve">` + bodyText + `</w:t></w:r></w:p>` +
		paragraph("", "Приказ от «05» июня 2026 г. и письмо от 5.6.2026, подпись И.И. Иванов") +
		`<w:p><w:r><w:rPr><w:color w:val="FF0000"/></w:rPr><w:t>ДАТА / РЕГИСТРАЦИОННЫЙ НОМЕР</w:t></w:r></w:p>`
	theme := `<?xml version="1.0" encoding="UTF-8"?><a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:fontScheme><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`
	original := build_Docx(t, body, section, styles, map[string]string{"word/footer1.xml": footer("center"), "word/theme/theme1.xml": theme})
	before, err := Check_Docx(original, format)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Fix_Docx(original, format)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Check_Docx(result.Body, format)
	if err != nil {
		t.Fatalf("fixed docx does not parse: %v", err)
	}
	remaining := ""
	for _, finding := range after {
		remaining += finding.String() + "\n"
	}
	// Красный текст - содержание, его убирает человек. Остальное исправлено
	if len(after) != 1 || !strings.Contains(remaining, "красный текст") {
		t.Fatalf("before %d, remaining:\n%s", len(before), remaining)
	}
	for _, want := range []string{texts.FormatFixPage, texts.FormatFixMargins, texts.FormatFixFirstPage, texts.FormatFixFont, texts.FormatFixSize,
		texts.FormatFixSpacing, texts.FormatFixAlign, texts.FormatFixIndent, texts.FormatFixPageNumber, texts.FormatFixDates, texts.FormatFixInitials} {
		found := false
		for _, applied := range result.Applied {
			found = found || applied == want
		}
		if !found {
			t.Errorf("not applied %q: %v", want, result.Applied)
		}
	}
	info, _ := read_Document(result.Body)
	all := ""
	for _, paragraph := range info.Paragraphs {
		all += paragraph.Text + "\n"
	}
	if !strings.Contains(all, "от 05.06.2026 и письмо от 05.06.2026, подпись И. И. Иванов") || !strings.Contains(all, bodyText) {
		t.Fatalf("text after fix:\n%s", all)
	}
	// Порядок свойств по схеме: полужирный и язык сохранены, размер встал между ними
	document := string(read_Entry(t, result.Body, "word/document.xml"))
	if !strings.Contains(document, `<w:b/><w:sz w:val="26"/><w:szCs w:val="26"/><w:lang w:val="ru-RU"/>`) {
		t.Fatalf("run props order: %s", document)
	}
	if !strings.Contains(document, `w:after="160"`) || !strings.Contains(document, `<w:cols w:space="708"/><w:titlePg/>`) {
		t.Fatalf("props lost: %s", document)
	}
}

func TestFix_DocxKeepsCleanDocument(t *testing.T) {
	body := paragraph("", "Генеральному директору В. В. Капибарову") + paragraph("", bodyText)
	original := build_Docx(t, body, goodSection, goodStyles, map[string]string{"word/footer1.xml": footer("right")})
	result, err := Fix_Docx(original, must_Format(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("clean document changed: %v", result.Applied)
	}
}
