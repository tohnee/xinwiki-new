package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// MarkdownGenerator produces Markdown artifacts directly from the LLM. It is
// the simplest generator: it sends a "write markdown" system prompt to the
// model, collects the response, and returns the raw .md bytes.
type MarkdownGenerator struct{}

func NewMarkdownGenerator() *MarkdownGenerator { return &MarkdownGenerator{} }

func (g *MarkdownGenerator) Type() types.ArtifactType { return types.ArtifactTypeMarkdown }

func (g *MarkdownGenerator) Generate(ctx context.Context, in *Input) (*Result, error) {
	if in.Chat == nil {
		return nil, fmt.Errorf("markdown generator: chat model is required")
	}
	systemPrompt := buildMarkdownSystemPrompt(in.Language)
	userPrompt := buildMarkdownUserPrompt(in)
	messages := []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	opts := &chat.ChatOptions{Temperature: 0.4, MaxTokens: 8192}
	resp, err := in.Chat.Chat(ctx, messages, opts)
	if err != nil {
		return nil, fmt.Errorf("markdown generator: %w", err)
	}
	content := resp.Content
	// Strip any ```markdown fences the model may have wrapped around.
	content = stripMarkdownFences(content)
	content = strings.TrimSpace(content) + "\n"
	return &Result{
		MIMEType:      "text/markdown; charset=utf-8",
		FileExtension: "md",
		Bytes:         []byte(content),
		ExtraMetadata: map[string]any{
			"char_count": len(content),
			"citations":  in.Citations,
		},
	}, nil
}

func buildMarkdownSystemPrompt(lang string) string {
	if lang == "zh" || strings.HasPrefix(lang, "zh-") {
		return `你是一名严谨的技术文档作者。请根据用户提供的来源材料，撰写一份结构清晰、可直接使用的 Markdown 文档。
要求：
- 包含一级标题、若干二级标题，必要时使用三级标题；
- 使用列表、表格、引用块增强可读性；
- 只依据提供的来源材料，不要编造事实；来源中未提及的数据不要臆测；
- 在文末 "## 来源" 章节用无序列表列出所依据的来源编号（如 [1]、[2]）；
- 直接输出 Markdown 正文，不要输出任何解释性前言或代码围栏。`
	}
	return `You are a precise technical writer. Produce a clean, publication-ready Markdown document based strictly on the provided source material.
Requirements:
- Include a single level-1 heading and well-structured level-2/3 sections;
- Use lists, tables, and blockquotes where helpful;
- Ground every claim in the supplied sources; do not invent facts or numbers;
- End with a "## Sources" section listing referenced source numbers (e.g. [1], [2]);
- Output raw Markdown only, with no fencing or commentary.`
}

func buildMarkdownUserPrompt(in *Input) string {
	var b strings.Builder
	b.WriteString("## 请求\n")
	b.WriteString(in.Prompt)
	b.WriteString("\n\n## 来源材料\n")
	if in.SourceText == "" {
		b.WriteString("(no source material supplied; produce the document from general knowledge but flag any assumptions.)\n")
	} else {
		b.WriteString(truncateSource(in.SourceText, 60000))
	}
	return b.String()
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```markdown") {
		s = strings.TrimPrefix(s, "```markdown")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```md") {
		s = strings.TrimPrefix(s, "```md")
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

func truncateSource(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
