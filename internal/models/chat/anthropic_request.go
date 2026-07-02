package chat

import (
	"encoding/json"
	"strings"
)

func (c *AnthropicChat) buildRequest(messages []Message, opts *ChatOptions) anthropicRequest {
	req := anthropicRequest{
		Model:     c.modelName,
		MaxTokens: 4096,
		Messages:  make([]anthropicMessage, 0, len(messages)),
	}
	if opts != nil {
		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		} else if opts.MaxCompletionTokens > 0 {
			req.MaxTokens = opts.MaxCompletionTokens
		}
		if opts.Temperature > 0 {
			temperature := opts.Temperature
			req.Temperature = &temperature
		}
		if opts.TopP > 0 {
			topP := opts.TopP
			req.TopP = &topP
		}
		// 配置Thinking
		if opts.Thinking != nil && *opts.Thinking {
			req.Thinking = &anthropicThinkingConfig{
				Type:         "enabled",
				BudgetTokens: defaultThinkingBudget,
			}
			// Thinking模式下需要更高的MaxTokens
			if req.MaxTokens < defaultThinkingBudget+1024 {
				req.MaxTokens = defaultThinkingBudget + 4096
			}
		}
		// 配置Tools
		if len(opts.Tools) > 0 {
			req.Tools = make([]anthropicTool, 0, len(opts.Tools))
			for _, tool := range opts.Tools {
				req.Tools = append(req.Tools, anthropicTool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: tool.Function.Parameters,
				})
			}
			// 配置tool_choice
			switch opts.ToolChoice {
			case "auto":
				req.ToolChoice = map[string]string{"type": "auto"}
			case "required":
				req.ToolChoice = map[string]string{"type": "any"}
			case "none":
				req.ToolChoice = map[string]string{"type": "none"}
			default:
				if opts.ToolChoice != "" {
					req.ToolChoice = map[string]any{
						"type": "tool",
						"name": opts.ToolChoice,
					}
				}
			}
		}
	}

	var systemContent []anthropicContentBlock
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				content = textFromMultiContent(msg.MultiContent)
			}
			if content != "" {
				systemContent = append(systemContent, anthropicContentBlock{
					Type: "text",
					Text: content,
				})
			}
		default:
			blocks := c.messageToAnthropicBlocks(msg)
			if len(blocks) > 0 {
				req.Messages = append(req.Messages, anthropicMessage{
					Role:    c.mapRole(msg.Role),
					Content: blocks,
				})
			}
		}
	}

	// 设置system prompt
	if len(systemContent) == 1 {
		req.System = systemContent[0].Text
	} else if len(systemContent) > 1 {
		req.System = systemContent
	}

	return req
}

// mapRole maps internal role names to Anthropic role names
func (c *AnthropicChat) mapRole(role string) string {
	switch role {
	case "tool":
		return "user"
	default:
		return role
	}
}

// messageToAnthropicBlocks converts an internal Message to Anthropic content blocks
func (c *AnthropicChat) messageToAnthropicBlocks(msg Message) []anthropicContentBlock {
	var blocks []anthropicContentBlock

	// 处理assistant的thinking内容
	if msg.Role == "assistant" && msg.ReasoningContent != "" {
		blocks = append(blocks, anthropicContentBlock{
			Type:     "thinking",
			Thinking: msg.ReasoningContent,
		})
	}

	// 处理文本内容
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = textFromMultiContent(msg.MultiContent)
	}
	if content != "" {
		blocks = append(blocks, anthropicContentBlock{
			Type: "text",
			Text: content,
		})
	}

	// 处理图片内容
	for _, part := range msg.MultiContent {
		if part.Type == "image_url" && part.ImageURL != nil {
			blocks = append(blocks, anthropicContentBlock{
				Type: "image",
				Data: part.ImageURL.URL,
			})
		}
	}

	// 处理assistant的tool calls
	if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var input any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			blocks = append(blocks, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	// 处理tool role的tool results
	if msg.Role == "tool" {
		resultContent := []anthropicContentBlock{{
			Type: "text",
			Text: msg.Content,
		}}
		blocks = append(blocks, anthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   resultContent,
		})
	}

	return blocks
}

func textFromMultiContent(parts []MessageContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	textParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(textParts, "\n")
}
