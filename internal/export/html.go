package export

import (
	"fmt"
	"html"
	"strings"

	"github.com/Vergil2599/startup/pes/internal/template"
)

// htmlCodec exporta el prompt renderizado como documento HTML autónomo
// (sin recursos externos: imprimible y archivable).
type htmlCodec struct{}

func (htmlCodec) ID() string        { return "html" }
func (htmlCodec) Extension() string { return ".html" }

func (htmlCodec) Export(in Input) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="es"><head><meta charset="utf-8">
<title>` + html.EscapeString(in.Prompt.Title) + `</title>
<style>
 body{font-family:-apple-system,'Segoe UI',Roboto,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1a1c22;line-height:1.55}
 h1{border-bottom:2px solid #4f46e5;padding-bottom:.4rem}
 h2{color:#4f46e5;margin-top:1.6rem;font-size:1.05rem;text-transform:uppercase;letter-spacing:.05em}
 pre{white-space:pre-wrap;background:#f6f6f4;border:1px solid #e5e5e2;border-radius:8px;padding:1rem;font-family:'JetBrains Mono',ui-monospace,monospace;font-size:.92rem}
 .meta{color:#666;font-size:.85rem}
 @media print{ pre{border:none;background:none;padding:0} }
</style></head><body>
`)
	sb.WriteString("<h1>" + html.EscapeString(in.Prompt.Title) + "</h1>\n")
	if in.Prompt.Description != "" {
		sb.WriteString(`<p class="meta">` + html.EscapeString(in.Prompt.Description) + "</p>\n")
	}
	if len(in.Prompt.Tags) > 0 {
		sb.WriteString(`<p class="meta">#` + html.EscapeString(strings.Join(in.Prompt.Tags, " #")) + "</p>\n")
	}
	for _, b := range in.Prompt.EnabledBlocks() {
		if b.IsEmpty() {
			continue
		}
		body, _ := template.RenderString(b.Content, in.Vars)
		sb.WriteString(fmt.Sprintf("<h2>%s</h2>\n<pre>%s</pre>\n",
			html.EscapeString(strings.ReplaceAll(string(b.Type), "_", " ")),
			html.EscapeString(body)))
	}
	sb.WriteString("</body></html>\n")
	return []byte(sb.String()), nil
}
