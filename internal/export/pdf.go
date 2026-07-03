package export

import (
	"bytes"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/Vergil2599/startup/pes/internal/template"
)

// pdfCodec exporta el prompt renderizado como PDF (generación pura Go,
// sin depender de un navegador ni binarios externos).
type pdfCodec struct{}

func (pdfCodec) ID() string        { return "pdf" }
func (pdfCodec) Extension() string { return ".pdf" }

func (pdfCodec) Export(in Input) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // cp1252: cubre es/en
	pdf.SetTitle(in.Prompt.Title, true)
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.MultiCell(0, 9, tr(in.Prompt.Title), "", "L", false)
	if in.Prompt.Description != "" {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(100, 100, 100)
		pdf.MultiCell(0, 5, tr(in.Prompt.Description), "", "L", false)
	}
	pdf.Ln(3)

	for _, b := range in.Prompt.EnabledBlocks() {
		if b.IsEmpty() {
			continue
		}
		body, _ := template.RenderString(b.Content, in.Vars)
		pdf.SetTextColor(79, 70, 229)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.MultiCell(0, 7, tr(strings.ToUpper(strings.ReplaceAll(string(b.Type), "_", " "))), "", "L", false)
		pdf.SetTextColor(26, 28, 34)
		pdf.SetFont("Courier", "", 10)
		pdf.MultiCell(0, 5, tr(body), "", "L", false)
		pdf.Ln(3)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
