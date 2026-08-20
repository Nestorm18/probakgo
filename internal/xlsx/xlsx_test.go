package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
)

func TestWorkbookWritesValidStyledPackage(t *testing.T) {
	workbook := Workbook{Sheets: []Sheet{{
		Name: "Resumen",
		Rows: [][]Cell{
			{StyledText("Informe", StyleTitle)},
			{StyledText("Nombre", StyleHeader), StyledText("Fecha", StyleHeader), StyledText("Tamaño", StyleHeader)},
			{Text("VM 100"), DateTime(time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)), Bytes(1_870_000_000_000)},
		},
		ColumnWidths: []float64{24, 20, 16}, FreezeRows: 2, AutoFilter: "A2:C3", Merges: []string{"A1:C1"},
	}}}
	data, err := workbook.Bytes()
	if err != nil {
		t.Fatalf("Workbook.Bytes: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open XLSX zip: %v", err)
	}
	required := map[string]bool{
		"[Content_Types].xml":      false,
		"xl/workbook.xml":          false,
		"xl/styles.xml":            false,
		"xl/worksheets/sheet1.xml": false,
	}
	for _, file := range reader.File {
		if _, ok := required[file.Name]; !ok {
			continue
		}
		required[file.Name] = true
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		raw, _ := io.ReadAll(stream)
		_ = stream.Close()
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		for {
			if _, err := decoder.Token(); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("invalid XML in %s: %v", file.Name, err)
			}
		}
		if file.Name == "xl/worksheets/sheet1.xml" {
			text := string(raw)
			for _, want := range []string{"VM 100", `state="frozen"`, `<autoFilter ref="A2:C3"/>`, `<mergeCell ref="A1:C1"/>`} {
				if !strings.Contains(text, want) {
					t.Fatalf("worksheet missing %q", want)
				}
			}
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("XLSX package missing %s", name)
		}
	}
}
