package layout

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"docssearch/core/internal/texts"
)

// Сборка .docx по ГОСТ Р 7.0.97-2025: A4, поля 20/10/20/20 мм (п. 6.1), Times New Roman 14 (п. 3.3),
// абзацный отступ 1,25 см (п. 3.4), текст по ширине (п. 3.5), адресат справа в зоне до 8 см (п. 3.5, 5.15)

const (
	twipsPerMM      = 1440.0 / 25.4
	emuPerMM        = 36000
	pageWidthMM     = 210
	pageHeightMM    = 297
	marginLeftMM    = 20
	marginRightMM   = 10
	marginTopMM     = 20
	marginBottomMM  = 20
	textWidthMM     = pageWidthMM - marginLeftMM - marginRightMM
	rightZoneMM     = 80
	firstLineMM     = 12.5
	fontHalfPoints  = 28
	smallHalfPoints = 24
	emblemHeightMM  = 18
	stampMaxWidthMM = 75
	stampMaxHeight  = 35
)

type signature struct {
	Position string
	Name     string
}

type document struct {
	Kind   string
	Emblem *Image
	Stamp  *Image
	// Реквизиты в правой зоне: адресат, ниже составитель записки
	HeaderRight []string
	TypeTitle   string
	DateLine    string
	Place       string
	Title       string
	Preamble    []string
	Greeting    string
	Text        []string
	Attach      string
	Closing     string
	Signatures  []signature
	Executor    string
}

func twips(mm float64) int {
	return int(mm*twipsPerMM + 0.5)
}

type paragraphStyle struct {
	align     string
	indentMM  float64
	firstLine bool
	bold      bool
	small     bool
	spaceMM   float64
	tabRight  bool
}

type docxWriter struct {
	body   strings.Builder
	images []docxImage
}

type docxImage struct {
	id          string
	name        string
	contentType string
	body        []byte
}

func (writer *docxWriter) paragraph(text string, style paragraphStyle) {
	writer.body.WriteString(paragraph_XML(text, style))
}

func paragraph_XML(text string, style paragraphStyle) string {
	var builder strings.Builder
	builder.WriteString(`<w:p><w:pPr>`)
	if style.tabRight {
		fmt.Fprintf(&builder, `<w:tabs><w:tab w:val="right" w:pos="%d"/></w:tabs>`, twips(textWidthMM))
	}
	if style.spaceMM > 0 {
		fmt.Fprintf(&builder, `<w:spacing w:before="%d"/>`, twips(style.spaceMM))
	}
	if style.indentMM > 0 || style.firstLine {
		builder.WriteString(`<w:ind`)
		if style.indentMM > 0 {
			fmt.Fprintf(&builder, ` w:left="%d"`, twips(style.indentMM))
		}
		if style.firstLine {
			fmt.Fprintf(&builder, ` w:firstLine="%d"`, twips(firstLineMM))
		}
		builder.WriteString(`/>`)
	}
	if style.align != "" {
		fmt.Fprintf(&builder, `<w:jc w:val="%s"/>`, style.align)
	}
	builder.WriteString(`</w:pPr>`)
	// Табуляция внутри текста - отдельный элемент w:tab
	for index, part := range strings.Split(text, "\t") {
		if index > 0 {
			builder.WriteString(`<w:r><w:tab/></w:r>`)
		}
		if part == "" {
			continue
		}
		builder.WriteString(`<w:r>`)
		if style.bold || style.small {
			builder.WriteString(`<w:rPr>`)
			if style.bold {
				builder.WriteString(`<w:b/>`)
			}
			if style.small {
				fmt.Fprintf(&builder, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, smallHalfPoints, smallHalfPoints)
			}
			builder.WriteString(`</w:rPr>`)
		}
		builder.WriteString(`<w:t xml:space="preserve">`)
		builder.WriteString(escape_XML(part))
		builder.WriteString(`</w:t></w:r>`)
	}
	builder.WriteString(`</w:p>`)
	return builder.String()
}

func (writer *docxWriter) empty() {
	writer.body.WriteString(`<w:p/>`)
}

// Картинка в отдельном абзаце. Размер - по пропорциям файла в пределах ширины и высоты
func (writer *docxWriter) image_XML(image *Image, name string, maxWidthMM, maxHeightMM float64, align string) string {
	widthMM, heightMM := fit_ImageSize(image.Body, maxWidthMM, maxHeightMM)
	if widthMM == 0 {
		return ""
	}
	index := len(writer.images) + 1
	id := fmt.Sprintf("rIdImage%d", index)
	extension := "png"
	if image.ContentType == "image/jpeg" {
		extension = "jpeg"
	}
	writer.images = append(writer.images, docxImage{id: id, name: fmt.Sprintf("image%d.%s", index, extension), contentType: image.ContentType, body: image.Body})
	cx, cy := int(widthMM*emuPerMM), int(heightMM*emuPerMM)
	return fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="%s"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">`+
		`<wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="%s"/>`+
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
		`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
		`<pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic>`+
		`</a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`,
		align, cx, cy, index, escape_XML(name), index, escape_XML(name), id, cx, cy)
}

func fit_ImageSize(body []byte, maxWidthMM, maxHeightMM float64) (float64, float64) {
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width == 0 || config.Height == 0 {
		return 0, 0
	}
	ratio := float64(config.Width) / float64(config.Height)
	heightMM := maxHeightMM
	widthMM := heightMM * ratio
	if widthMM > maxWidthMM {
		widthMM = maxWidthMM
		heightMM = widthMM / ratio
	}
	return widthMM, heightMM
}

// Таблица без рамок из двух колонок: угловой бланк письма - слева реквизиты организации, справа адресат
func two_ColumnsXML(left, right string) string {
	half := twips(textWidthMM / 2)
	cell := func(content string) string {
		if content == "" {
			content = `<w:p/>`
		}
		return fmt.Sprintf(`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr>%s</w:tc>`, half, content)
	}
	borders := `<w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders>`
	return fmt.Sprintf(`<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>%s`+
		`<w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr>`+
		`<w:tblGrid><w:gridCol w:w="%d"/><w:gridCol w:w="%d"/></w:tblGrid><w:tr>%s%s</w:tr></w:tbl>`,
		half*2, borders, half, half, cell(left), cell(right))
}

func build_Docx(doc document) ([]byte, error) {
	writer := &docxWriter{}
	rightZone := paragraphStyle{indentMM: textWidthMM - rightZoneMM}
	switch doc.Kind {
	case texts.LayoutKindLetter:
		var left strings.Builder
		if doc.Emblem != nil {
			left.WriteString(writer.image_XML(doc.Emblem, "emblem", textWidthMM/2, emblemHeightMM, "center"))
		}
		if doc.Stamp != nil {
			left.WriteString(writer.image_XML(doc.Stamp, "stamp", stampMaxWidthMM, stampMaxHeight, "center"))
		}
		left.WriteString(paragraph_XML(doc.DateLine, paragraphStyle{align: "center", spaceMM: 2}))
		var right strings.Builder
		for _, line := range doc.HeaderRight {
			right.WriteString(paragraph_XML(line, paragraphStyle{indentMM: 10}))
		}
		writer.body.WriteString(two_ColumnsXML(left.String(), right.String()))
	case texts.LayoutKindAct, texts.LayoutKindProtocol:
		// Общий бланк: герб посередине над реквизитами организации (п. 5.2, 6.5)
		if doc.Emblem != nil {
			writer.body.WriteString(writer.image_XML(doc.Emblem, "emblem", textWidthMM, emblemHeightMM, "center"))
		}
		if doc.Stamp != nil {
			writer.body.WriteString(writer.image_XML(doc.Stamp, "stamp", stampMaxWidthMM, stampMaxHeight, "center"))
		}
		writer.paragraph(strings.ToUpper(doc.TypeTitle), paragraphStyle{align: "center", bold: true, spaceMM: 4})
		writer.paragraph(doc.DateLine, paragraphStyle{spaceMM: 4})
		if doc.Place != "" {
			writer.paragraph(doc.Place, paragraphStyle{})
		}
	default:
		for _, line := range doc.HeaderRight {
			if line == "" {
				writer.empty()
				continue
			}
			writer.paragraph(line, rightZone)
		}
		writer.empty()
		writer.paragraph(strings.ToUpper(doc.TypeTitle), paragraphStyle{bold: true})
		writer.paragraph(doc.DateLine, paragraphStyle{})
	}
	if doc.Title != "" {
		writer.paragraph(doc.Title, paragraphStyle{spaceMM: 6})
	}
	writer.empty()
	for _, line := range doc.Preamble {
		writer.paragraph(line, paragraphStyle{align: "both", firstLine: true})
	}
	if doc.Greeting != "" {
		writer.paragraph(doc.Greeting, paragraphStyle{align: "center", spaceMM: 2})
		writer.empty()
	}
	for _, line := range doc.Text {
		writer.paragraph(line, paragraphStyle{align: "both", firstLine: true})
	}
	if doc.Attach != "" {
		writer.empty()
		writer.paragraph(texts.LayoutAttachLabel+" "+doc.Attach, paragraphStyle{})
	}
	if doc.Closing != "" {
		writer.empty()
		writer.paragraph(doc.Closing, paragraphStyle{})
	}
	for index, sign := range doc.Signatures {
		space := 3.0
		if index == 0 {
			space = 10
		}
		writer.paragraph(sign.Position+"\t"+sign.Name, paragraphStyle{tabRight: true, spaceMM: space})
	}
	if doc.Executor != "" {
		writer.paragraph(doc.Executor, paragraphStyle{small: true, spaceMM: 20})
	}
	return writer.package_Docx()
}

func (writer *docxWriter) package_Docx() ([]byte, error) {
	section := fmt.Sprintf(`<w:sectPr><w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="567" w:footer="567" w:gutter="0"/></w:sectPr>`,
		twips(pageWidthMM), twips(pageHeightMM), twips(marginTopMM), twips(marginRightMM), twips(marginBottomMM), twips(marginLeftMM))
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">` +
		`<w:body>` + writer.body.String() + section + `</w:body></w:document>`
	var relations strings.Builder
	relations.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	relations.WriteString(`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	for _, item := range writer.images {
		fmt.Fprintf(&relations, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`, item.id, item.name)
	}
	relations.WriteString(`</Relationships>`)
	files := []struct {
		name string
		body []byte
	}{
		{"[Content_Types].xml", []byte(contentTypesXML)},
		{"_rels/.rels", []byte(packageRelsXML)},
		{"word/document.xml", []byte(documentXML)},
		{"word/styles.xml", []byte(styles_XML())},
		{"word/_rels/document.xml.rels", []byte(relations.String())},
	}
	for _, item := range writer.images {
		files = append(files, struct {
			name string
			body []byte
		}{"word/media/" + item.name, item.body})
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, err := archive.Create(file.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(entry, bytes.NewReader(file.body)); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Шрифт и одинарный интервал по умолчанию: реквизиты через один интервал (п. 3.4)
func styles_XML() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults>`+
		`<w:rPrDefault><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman" w:eastAsia="Times New Roman"/>`+
		`<w:sz w:val="%d"/><w:szCs w:val="%d"/><w:lang w:val="ru-RU"/></w:rPr></w:rPrDefault>`+
		`<w:pPrDefault><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault>`+
		`</w:docDefaults></w:styles>`, fontHalfPoints, fontHalfPoints)
}

func escape_XML(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="png" ContentType="image/png"/>
<Default Extension="jpeg" ContentType="image/jpeg"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const packageRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
