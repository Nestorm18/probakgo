package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	StyleDefault = iota
	StyleTitle
	StyleSection
	StyleHeader
	StyleDateTime
	StyleDuration
	StyleBytes
	StylePercent
	StyleInteger
	StyleOK
	StyleWarning
	StyleError
	StyleSummaryLabel
	StyleSummaryValue
)

type Cell struct {
	Value   string
	Formula string
	Number  float64
	Style   int
	Numeric bool
}

func Text(value string) Cell                  { return Cell{Value: value} }
func StyledText(value string, style int) Cell { return Cell{Value: value, Style: style} }
func Number(value float64, style int) Cell {
	return Cell{Number: value, Style: style, Numeric: true}
}
func Integer(value int64) Cell { return Number(float64(value), StyleInteger) }
func DateTime(value time.Time) Cell {
	if value.IsZero() {
		return Cell{Style: StyleDateTime}
	}
	return Number(excelDate(value), StyleDateTime)
}
func Duration(seconds int64) Cell {
	return Number(float64(seconds)/(24*60*60), StyleDuration)
}
func Bytes(value int64) Cell { return Number(float64(value), StyleBytes) }
func Percent(value float64) Cell {
	return Number(value, StylePercent)
}
func Formula(formula string, cached float64, style int) Cell {
	return Cell{Formula: formula, Number: cached, Style: style, Numeric: true}
}

type Sheet struct {
	Name         string
	Rows         [][]Cell
	ColumnWidths []float64
	FreezeRows   int
	AutoFilter   string
	Merges       []string
}

type Workbook struct {
	Sheets []Sheet
}

func (w Workbook) Write(out io.Writer) error {
	if len(w.Sheets) == 0 {
		return fmt.Errorf("xlsx: workbook has no sheets")
	}
	zw := zip.NewWriter(out)
	files := []struct {
		name string
		data []byte
	}{
		{"[Content_Types].xml", []byte(contentTypes(len(w.Sheets)))},
		{"_rels/.rels", []byte(rootRelationships)},
		{"docProps/app.xml", []byte(appProperties(w.Sheets))},
		{"docProps/core.xml", []byte(coreProperties())},
		{"xl/workbook.xml", []byte(workbookXML(w.Sheets))},
		{"xl/_rels/workbook.xml.rels", []byte(workbookRelationships(len(w.Sheets)))},
		{"xl/styles.xml", []byte(stylesXML)},
	}
	for i, sheet := range w.Sheets {
		data, err := worksheetXML(sheet)
		if err != nil {
			_ = zw.Close()
			return fmt.Errorf("xlsx: render sheet %q: %w", sheet.Name, err)
		}
		files = append(files, struct {
			name string
			data []byte
		}{fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), data})
	}
	for _, file := range files {
		entry, err := zw.Create(file.name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := entry.Write(file.data); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func (w Workbook) Bytes() ([]byte, error) {
	var out bytes.Buffer
	if err := w.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func worksheetXML(sheet Sheet) ([]byte, error) {
	if strings.TrimSpace(sheet.Name) == "" {
		return nil, fmt.Errorf("sheet name is empty")
	}
	maxCols := len(sheet.ColumnWidths)
	for _, row := range sheet.Rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		maxCols = 1
	}
	maxRows := len(sheet.Rows)
	if maxRows == 0 {
		maxRows = 1
	}
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<dimension ref="A1:` + cellReference(maxRows, maxCols) + `"/>`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0" showGridLines="0">`)
	if sheet.FreezeRows > 0 {
		b.WriteString(fmt.Sprintf(`<pane ySplit="%d" topLeftCell="A%d" activePane="bottomLeft" state="frozen"/>`, sheet.FreezeRows, sheet.FreezeRows+1))
	}
	b.WriteString(`</sheetView></sheetViews><sheetFormatPr defaultRowHeight="15"/>`)
	if len(sheet.ColumnWidths) > 0 {
		b.WriteString(`<cols>`)
		for i, width := range sheet.ColumnWidths {
			if width <= 0 {
				continue
			}
			b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%s" customWidth="1"/>`, i+1, i+1, strconv.FormatFloat(width, 'f', 2, 64)))
		}
		b.WriteString(`</cols>`)
	}
	b.WriteString(`<sheetData>`)
	for rowIndex, row := range sheet.Rows {
		rowNumber := rowIndex + 1
		height := ""
		if rowNumber == 1 {
			height = ` ht="26" customHeight="1"`
		} else if hasStyle(row, StyleHeader) {
			height = ` ht="30" customHeight="1"`
		}
		b.WriteString(fmt.Sprintf(`<row r="%d"%s>`, rowNumber, height))
		for colIndex, cell := range row {
			if cell.Value == "" && cell.Formula == "" && !cell.Numeric && cell.Style == StyleDefault {
				continue
			}
			ref := cellReference(rowNumber, colIndex+1)
			style := ""
			if cell.Style != StyleDefault {
				style = fmt.Sprintf(` s="%d"`, cell.Style)
			}
			switch {
			case cell.Formula != "":
				b.WriteString(`<c r="` + ref + `"` + style + `><f>`)
				writeEscaped(&b, cell.Formula)
				b.WriteString(`</f><v>` + strconv.FormatFloat(cell.Number, 'g', -1, 64) + `</v></c>`)
			case cell.Numeric:
				b.WriteString(`<c r="` + ref + `"` + style + `><v>` + strconv.FormatFloat(cell.Number, 'g', -1, 64) + `</v></c>`)
			default:
				b.WriteString(`<c r="` + ref + `" t="inlineStr"` + style + `><is><t xml:space="preserve">`)
				writeEscaped(&b, cell.Value)
				b.WriteString(`</t></is></c>`)
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData>`)
	if sheet.AutoFilter != "" {
		b.WriteString(`<autoFilter ref="`)
		writeEscaped(&b, sheet.AutoFilter)
		b.WriteString(`"/>`)
	}
	if len(sheet.Merges) > 0 {
		b.WriteString(fmt.Sprintf(`<mergeCells count="%d">`, len(sheet.Merges)))
		for _, merge := range sheet.Merges {
			b.WriteString(`<mergeCell ref="`)
			writeEscaped(&b, merge)
			b.WriteString(`"/>`)
		}
		b.WriteString(`</mergeCells>`)
	}
	b.WriteString(`<pageMargins left="0.35" right="0.35" top="0.5" bottom="0.5" header="0.2" footer="0.2"/>`)
	b.WriteString(`</worksheet>`)
	return []byte(b.String()), nil
}

func hasStyle(row []Cell, style int) bool {
	for _, cell := range row {
		if cell.Style == style {
			return true
		}
	}
	return false
}

func writeEscaped(out io.Writer, value string) {
	_ = xml.EscapeText(out, []byte(value))
}

func cellReference(row, col int) string {
	var letters [8]byte
	position := len(letters)
	for col > 0 {
		col--
		position--
		letters[position] = byte('A' + col%26)
		col /= 26
	}
	return string(letters[position:]) + strconv.Itoa(row)
}

func excelDate(value time.Time) float64 {
	value = value.In(time.UTC)
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	return value.Sub(base).Hours() / 24
}

func contentTypes(sheetCount int) string {
	var sheets strings.Builder
	for i := 1; i <= sheetCount; i++ {
		sheets.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i))
	}
	return xml.Header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` + sheets.String() + `</Types>`
}

func workbookXML(sheets []Sheet) string {
	var rows strings.Builder
	for i, sheet := range sheets {
		rows.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escapeAttribute(sheet.Name), i+1, i+1))
	}
	return xml.Header + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView/></bookViews><sheets>` + rows.String() + `</sheets><calcPr calcId="191029" calcMode="auto" fullCalcOnLoad="1" forceFullCalc="1"/></workbook>`
}

func workbookRelationships(sheetCount int) string {
	var rels strings.Builder
	for i := 1; i <= sheetCount; i++ {
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i))
	}
	rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, sheetCount+1))
	return xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + rels.String() + `</Relationships>`
}

func appProperties(sheets []Sheet) string {
	var titles strings.Builder
	for _, sheet := range sheets {
		titles.WriteString(`<vt:lpstr>` + escapeAttribute(sheet.Name) + `</vt:lpstr>`)
	}
	return xml.Header + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"><Application>Probakgo</Application><DocSecurity>0</DocSecurity><ScaleCrop>false</ScaleCrop><HeadingPairs><vt:vector size="2" baseType="variant"><vt:variant><vt:lpstr>Worksheets</vt:lpstr></vt:variant><vt:variant><vt:i4>` + strconv.Itoa(len(sheets)) + `</vt:i4></vt:variant></vt:vector></HeadingPairs><TitlesOfParts><vt:vector size="` + strconv.Itoa(len(sheets)) + `" baseType="lpstr">` + titles.String() + `</vt:vector></TitlesOfParts></Properties>`
}

func coreProperties() string {
	now := time.Now().UTC().Format(time.RFC3339)
	return xml.Header + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:creator>Probakgo</dc:creator><cp:lastModifiedBy>Probakgo</cp:lastModifiedBy><dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified></cp:coreProperties>`
}

func escapeAttribute(value string) string {
	var out strings.Builder
	writeEscaped(&out, value)
	return out.String()
}

const rootRelationships = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <numFmts count="3"><numFmt numFmtId="164" formatCode="dd/mm/yyyy hh:mm"/><numFmt numFmtId="165" formatCode="[h]:mm:ss"/><numFmt numFmtId="166" formatCode="[&gt;=1000000000000]0.00,,,,&quot; TB&quot;;[&gt;=1000000000]0.00,,,&quot; GB&quot;;0&quot; B&quot;"/></numFmts>
  <fonts count="4"><font><sz val="10"/><name val="Aptos"/><family val="2"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="16"/><name val="Aptos Display"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="10"/><name val="Aptos"/></font><font><b/><color rgb="FF172033"/><sz val="12"/><name val="Aptos"/></font></fonts>
  <fills count="8"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF172033"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FF2F6FED"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFDDF5E7"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFFFF1CC"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFFDE2E2"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEAF0FB"/><bgColor indexed="64"/></patternFill></fill></fills>
  <borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD7DCE5"/></left><right style="thin"><color rgb="FFD7DCE5"/></right><top style="thin"><color rgb="FFD7DCE5"/></top><bottom style="thin"><color rgb="FFD7DCE5"/></bottom><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="14">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"><alignment vertical="center"/></xf>
    <xf numFmtId="0" fontId="2" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1"><alignment vertical="center"/></xf>
    <xf numFmtId="0" fontId="2" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf>
    <xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
    <xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
    <xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
    <xf numFmtId="10" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
    <xf numFmtId="3" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
    <xf numFmtId="0" fontId="0" fillId="4" borderId="0" xfId="0" applyFill="1"/>
    <xf numFmtId="0" fontId="0" fillId="5" borderId="0" xfId="0" applyFill="1"/>
    <xf numFmtId="0" fontId="0" fillId="6" borderId="0" xfId="0" applyFill="1"/>
    <xf numFmtId="0" fontId="0" fillId="7" borderId="1" xfId="0" applyFill="1" applyBorder="1"/>
    <xf numFmtId="3" fontId="3" fillId="0" borderId="0" xfId="0" applyFont="1" applyNumberFormat="1"/>
  </cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles><dxfs count="0"/><tableStyles count="0" defaultTableStyle="TableStyleMedium2" defaultPivotStyle="PivotStyleLight16"/>
</styleSheet>`
