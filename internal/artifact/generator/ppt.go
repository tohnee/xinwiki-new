package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// PPTGenerator produces slide decks. It asks the LLM for a structured slide
// outline (title, bullets, notes), then renders the slides as an HTML
// presentation and prints to PDF via chromedp. The output is a PDF file.
type PPTGenerator struct{}

func NewPPTGenerator() *PPTGenerator { return &PPTGenerator{} }

func (g *PPTGenerator) Type() types.ArtifactType { return types.ArtifactTypePPT }

type slide struct {
	Title    string   `json:"title"`
	Layout   string   `json:"layout"` // title, content, two_col, bullets, closing
	Bullets  []string `json:"bullets,omitempty"`
	Subtitle string   `json:"subtitle,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}
type slideDeck struct {
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Slides      []slide `json:"slides"`
	TotalSlides int     `json:"total_slides"`
}

func (g *PPTGenerator) Generate(ctx context.Context, in *Input) (*Result, error) {
	if in.Chat == nil {
		return nil, fmt.Errorf("ppt generator: chat model is required")
	}
	deck, err := g.buildDeck(ctx, in)
	if err != nil {
		return nil, err
	}
	html := pptHTMLTemplate(deck)
	pdfBytes, _, err := htmlToPDF(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("ppt: render PDF: %w", err)
	}
	total := len(deck.Slides) + 2 // cover + closing
	return &Result{
		MIMEType:      "application/pdf",
		FileExtension: "pdf",
		Bytes:         pdfBytes,
		ExtraMetadata: map[string]any{
			"slide_count": total,
			"title":       deck.Title,
		},
	}, nil
}

func (g *PPTGenerator) buildDeck(ctx context.Context, in *Input) (*slideDeck, error) {
	sysPrompt := `You are a presentation designer. Given a user request and source material, produce a structured slide deck as a single JSON object.

JSON schema (return ONLY this object, no markdown fences):
{
  "title": "<slide deck title>",
  "subtitle": "<optional subtitle>",
  "slides": [
    { "title": "...", "layout": "content|bullets|two_col", "bullets": ["..."], "subtitle": "...", "notes": "..." }
  ]
}

Rules:
- Produce 8-12 content slides.
- Use "bullets" layout for most slides (3-5 short bullets each).
- Use "content" for single-message slides.
- Use "two_col" for comparisons.
- Ground every bullet in the source material; do not invent numbers.
- Titles ≤ 8 words; bullets ≤ 20 words each.
- Output MUST be valid JSON.`
	userMsg := fmt.Sprintf("Request: %s\n\nSource material:\n%s", in.Prompt, truncateSource(in.SourceText, 40000))
	opts := &chat.ChatOptions{Temperature: 0.5, MaxTokens: 8192}
	resp, err := in.Chat.Chat(ctx, []chat.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userMsg},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("ppt LLM call: %w", err)
	}
	raw := stripJSONFences(resp.Content)
	var deck slideDeck
	if err := decodeJSONObj(raw, &deck); err != nil {
		return nil, fmt.Errorf("ppt: invalid deck JSON: %w\nraw: %.600s", err, raw)
	}
	if deck.Title == "" {
		deck.Title = in.Artifact.Title
	}
	deck.TotalSlides = len(deck.Slides) + 2
	return &deck, nil
}

func decodeJSONObj(raw string, out any) error {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < 0 || end < start {
		return fmt.Errorf("no JSON object found")
	}
	return json.Unmarshal([]byte(raw[start:end+1]), out)
}

func pptHTMLTemplate(deck *slideDeck) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh"><head><meta charset="utf-8"><title>`)
	b.WriteString(escapeHTML(deck.Title))
	b.WriteString(`</title><style>
@page { size: 1280px 720px; margin: 0; }
body { margin: 0; font-family: -apple-system, "Helvetica Neue", "PingFang SC", "Microsoft YaHei", Arial, sans-serif; color: #1d1d1f; }
.slide { page-break-after: always; width: 1280px; height: 720px; padding: 72px 96px; box-sizing: border-box; position: relative; background: #fff; }
.slide.cover { background: linear-gradient(135deg,#007aff 0%, #5856d6 100%); color: #fff; display: flex; flex-direction: column; justify-content: center; }
.slide.cover h1 { font-size: 48pt; margin: 0 0 16pt; color: #fff; border: none; }
.slide.cover .sub { font-size: 20pt; opacity: 0.9; }
.slide.cover .meta { position: absolute; bottom: 64px; left: 96px; font-size: 12pt; opacity: 0.75; }
.slide.closing { background: #1d1d1f; color: #fff; display:flex; align-items:center; justify-content:center; }
.slide.closing h1 { color:#fff; font-size: 48pt; font-weight: 600; }
h1 { font-size: 32pt; margin: 0 0 24pt; color: #007aff; border-bottom: none; }
ul { padding-left: 28pt; margin: 0; font-size: 22pt; line-height: 1.55; }
li { margin: 8pt 0; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; gap: 36pt; font-size: 20pt; }
.content { font-size: 22pt; line-height: 1.6; }
.subtitle { color: #8e8e93; font-size: 16pt; margin-bottom: 20pt; }
.footer { position: absolute; bottom: 28px; left: 96px; right: 96px; display: flex; justify-content: space-between; font-size: 11pt; color: #8e8e93; }
</style></head><body>`)

	fmt.Fprintf(&b, `<section class="slide cover"><h1>%s</h1><div class="sub">%s</div><div class="meta">Generated by XinWiki</div></section>`,
		escapeHTML(deck.Title), escapeHTML(deck.Subtitle))
	for i, s := range deck.Slides {
		b.WriteString(`<section class="slide">`)
		fmt.Fprintf(&b, `<h1>%s</h1>`, escapeHTML(s.Title))
		if s.Subtitle != "" {
			fmt.Fprintf(&b, `<div class="subtitle">%s</div>`, escapeHTML(s.Subtitle))
		}
		switch s.Layout {
		case "two_col":
			mid := len(s.Bullets) / 2
			b.WriteString(`<div class="two-col"><ul>`)
			for _, bullet := range s.Bullets[:max1(mid, 1)] {
				fmt.Fprintf(&b, `<li>%s</li>`, escapeHTML(bullet))
			}
			b.WriteString(`</ul><ul>`)
			for _, bullet := range s.Bullets[mid:] {
				fmt.Fprintf(&b, `<li>%s</li>`, escapeHTML(bullet))
			}
			b.WriteString(`</ul></div>`)
		case "content":
			fmt.Fprintf(&b, `<div class="content">%s</div>`, escapeHTML(strings.Join(s.Bullets, "\n\n")))
		default:
			b.WriteString(`<ul>`)
			for _, bullet := range s.Bullets {
				fmt.Fprintf(&b, `<li>%s</li>`, escapeHTML(bullet))
			}
			b.WriteString(`</ul>`)
		}
		fmt.Fprintf(&b, `<div class="footer"><span>%s</span><span>%d / %d</span></div>`,
			escapeHTML(deck.Title), i+2, deck.TotalSlides)
		b.WriteString(`</section>`)
	}
	b.WriteString(`<section class="slide closing"><h1>Thank You</h1></section>`)
	b.WriteString(`</body></html>`)
	return b.String()
}

func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}
