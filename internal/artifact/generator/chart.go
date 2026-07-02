package generator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// ChartGenerator renders data visualizations. It asks the LLM for a Chart.js
// configuration (type, data, options) grounded in source material, then
// renders that configuration to a PNG via headless Chrome.
type ChartGenerator struct{}

func NewChartGenerator() *ChartGenerator { return &ChartGenerator{} }

func (g *ChartGenerator) Type() types.ArtifactType { return types.ArtifactTypeChart }

type chartSpec struct {
	Title       string `json:"title"`
	ChartType   string `json:"chart_type"` // bar, line, pie, doughnut, radar, scatter
	Description string `json:"description"`
	Config      string `json:"config"` // raw Chart.js config JSON
}

func (g *ChartGenerator) Generate(ctx context.Context, in *Input) (*Result, error) {
	if in.Chat == nil {
		return nil, fmt.Errorf("chart generator: chat model is required")
	}
	spec, err := g.buildSpec(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := validateChartConfig(spec.Config); err != nil {
		return nil, fmt.Errorf("chart: invalid config: %w", err)
	}
	html := chartHTMLTemplate(spec.Title, spec.Config)
	pngBytes, err := htmlToPNG(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("chart: render PNG: %w", err)
	}
	if len(pngBytes) > maxArtifactSize {
		return nil, fmt.Errorf("chart: generated image too large (%d bytes, max %d)", len(pngBytes), maxArtifactSize)
	}
	meta := map[string]any{
		"chart_type":   spec.ChartType,
		"title":        spec.Title,
		"description":  spec.Description,
		"config_json":  spec.Config,
		"width":        900,
		"height":       560,
	}
	return &Result{
		MIMEType:      "image/png",
		FileExtension: "png",
		Bytes:         pngBytes,
		ExtraMetadata: meta,
	}, nil
}

func (g *ChartGenerator) buildSpec(ctx context.Context, in *Input) (*chartSpec, error) {
	sysPrompt := `You are a data visualization expert. Given a user request and source material, produce a Chart.js configuration that accurately represents the relevant data.

Return a single JSON object with exactly these fields:
- "title": short chart title (plain text, no quotes around it)
- "chart_type": one of "bar", "line", "pie", "doughnut", "radar"
- "description": one-sentence description of what the chart shows
- "config": a complete Chart.js configuration object as a JSON string (so it must be double-encoded). It MUST follow this shape:
  {
    "type": <chart_type>,
    "data": {
      "labels": [...],
      "datasets": [{ "label": "...", "data": [...], ... }]
    },
    "options": { "responsive": false, "plugins": { "legend": { "position": "top" }, "title": { "display": false } } }
  }

Rules:
- Use only data points actually present in the source material.
- If the data is not amenable to a chart, still produce a chart from the most relevant numerical data available.
- Output only the JSON object, no markdown fences or commentary.`
	userMsg := fmt.Sprintf("Request: %s\n\nSource material:\n%s", in.Prompt, truncateSource(in.SourceText, 40000))
	opts := &chat.ChatOptions{Temperature: 0.2, MaxTokens: 4096}
	resp, err := in.Chat.Chat(ctx, []chat.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userMsg},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("chart LLM call: %w", err)
	}
	raw := stripJSONFences(resp.Content)
	var spec chartSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, fmt.Errorf("chart: invalid JSON from model: %w\nraw: %.400s", err, raw)
	}
	if spec.Title == "" {
		spec.Title = in.Artifact.Title
	}
	if spec.ChartType == "" {
		spec.ChartType = "bar"
	}
	if spec.Config == "" {
		return nil, fmt.Errorf("chart: config missing from model output")
	}
	return &spec, nil
}

func chartHTMLTemplate(title, configJSON string) string {
	safeConfig := strings.ReplaceAll(configJSON, "</", "<\\/")
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>%s</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js" referrerpolicy="no-referrer" crossorigin="anonymous"></script>
<style>
body { margin: 0; padding: 24px; background: #fff; font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif; }
h1 { font-size: 18pt; color: #1d1d1f; margin: 0 0 14pt; text-align: center; }
#chartWrap { width: 900px; height: 560px; margin: 0 auto; }
</style></head><body>
<h1>%s</h1>
<div id="chartWrap"><canvas id="c" width="860" height="520"></canvas></div>
<script id="chart-config" type="application/json">%s</script>
<script>
(function(){
  var el = document.getElementById('chart-config');
  var cfg;
  try { cfg = JSON.parse(el.textContent); } catch(e) { document.body.innerHTML = '<p style="color:red">Invalid chart config</p>'; return; }
  if (typeof cfg !== 'object' || cfg === null) { document.body.innerHTML = '<p style="color:red">Invalid chart config</p>'; return; }
  new Chart(document.getElementById('c'), cfg);
})();
</script></body></html>`, escapeHTML(title), escapeHTML(title), safeConfig)
}

const maxArtifactSize = 50 * 1024 * 1024 // 50 MB

// validateChartConfig performs basic structural validation on the Chart.js
// config JSON to reject obviously malicious or malformed payloads before
// they are injected into the headless Chrome renderer.
func validateChartConfig(configJSON string) error {
	if len(configJSON) > 100*1024 {
		return fmt.Errorf("chart config too large (%d bytes)", len(configJSON))
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("chart config is not valid JSON: %w", err)
	}
	allowedTypes := map[string]bool{"bar": true, "line": true, "pie": true, "doughnut": true, "radar": true, "scatter": true, "bubble": true, "polarArea": true}
	t, ok := cfg["type"].(string)
	if !ok || !allowedTypes[t] {
		return fmt.Errorf("chart config has invalid or missing 'type' field")
	}
	data, ok := cfg["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("chart config missing 'data' object")
	}
	if _, ok := data["datasets"]; !ok {
		return fmt.Errorf("chart config missing 'data.datasets'")
	}
	return nil
}

func htmlToPNG(ctx context.Context, html string) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(html))
	dataURL := "data:text/html;base64," + encoded

	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(execCtx, opts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var buf []byte
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("#c", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			time.Sleep(800 * time.Millisecond)
			return nil
		}),
		chromedp.EmulateViewport(960, 640),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}
	return buf, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
