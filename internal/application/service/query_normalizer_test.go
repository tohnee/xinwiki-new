package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryNormalizer_Normalize(t *testing.T) {
	n := NewQueryNormalizer(DefaultQueryNormalizeConfig())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim whitespace",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "collapses multiple spaces",
			input:    "hello   world\t\nfoo",
			expected: "hello world foo",
		},
		{
			name:     "lowercases english",
			input:    "Hello World! HOW ARE YOU?",
			expected: "hello world how are you",
		},
		{
			name:     "removes punctuation",
			input:    "hello, world! how are you? i'm fine.",
			expected: "hello world how are you i m fine",
		},
		{
			name:     "full width to half width - letters",
			input:    "Ｈｅｌｌｏ　Ｗｏｒｌｄ",
			expected: "hello world",
		},
		{
			name:     "full width to half width - numbers",
			input:    "１２３４５",
			expected: "12345",
		},
		{
			name:     "full width punctuation removed",
			input:    "你好，世界！今天天气怎么样？",
			expected: "你好 世界 今天天气怎么样",
		},
		{
			name:     "mixed chinese english",
			input:    "  什么是 RAG？Retrieval Augmented Generation  ",
			expected: "什么是 rag retrieval augmented generation",
		},
		{
			name:     "preserves chinese characters",
			input:    "向量数据库的工作原理是什么",
			expected: "向量数据库的工作原理是什么",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "handles only punctuation",
			input:    "!!!???...，。！？",
			expected: "",
		},
		{
			name:     "removes symbols like emojis (basic)",
			input:    "hello 😊 world 🔥",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryNormalizer_DedupImprovesHitRate(t *testing.T) {
	n := NewQueryNormalizer(DefaultQueryNormalizeConfig())

	queriesThatShouldMatch := [][]string{
		{"What is RAG?", "what is rag", "  what is rag?  ", "What is RAG！"},
		{"你好，世界", "你好 世界", "你好　世界", "你好,世界!"},
		{"Hello   World", "hello world", "HELLO WORLD", "Hello, World."},
		{"Ｈｅｌｌｏ", "hello", "HELLO", "  Hello  "},
	}

	for _, group := range queriesThatShouldMatch {
		normalized := n.Normalize(group[0])
		for _, q := range group[1:] {
			assert.Equal(t, normalized, n.Normalize(q), "queries should normalize to same key: %q vs %q", group[0], q)
		}
	}
}

func TestQueryNormalizer_Disabled(t *testing.T) {
	config := DefaultQueryNormalizeConfig()
	config.Enabled = false
	n := NewQueryNormalizer(config)

	input := "  Hello, World!  "
	result := n.Normalize(input)
	assert.Equal(t, input, result, "disabled normalizer should return input unchanged")
}

func TestQueryNormalizer_IsNormalized(t *testing.T) {
	n := NewQueryNormalizer(DefaultQueryNormalizeConfig())

	assert.True(t, n.IsNormalized("hello world"))
	assert.True(t, n.IsNormalized("你好世界"))
	assert.False(t, n.IsNormalized("Hello World"))
	assert.False(t, n.IsNormalized("  hello world  "))
	assert.False(t, n.IsNormalized("hello, world!"))
}

func BenchmarkQueryNormalizer_Normalize(b *testing.B) {
	n := NewQueryNormalizer(DefaultQueryNormalizeConfig())
	query := "  What is RAG (Retrieval Augmented Generation)? How does it work in vector databases?  "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.Normalize(query)
	}
}
