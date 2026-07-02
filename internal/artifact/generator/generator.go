// Package generator implements artifact generators for Markdown, PDF reports,
// charts, and presentations. Generators take an Input (prompt + source
// material + LLM) and produce a binary artifact.
package generator

import (
	"context"
	"fmt"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// Input bundles everything a Generator needs to produce output.
type Input struct {
	// Prompt is the natural-language generation request from the user
	// (e.g. "Summarize this into a 10-slide deck about X").
	Prompt string
	// SourceText is pre-assembled source material (knowledge chunks, wiki
	// page markdown, search results) concatenated for the generator to
	// ground its output in.
	SourceText string
	// Artifact is the pending artifact row; generators may mutate
	// Metadata to record generator-specific data (slide count, chart
	// spec, page count, etc.).
	Artifact *types.Artifact
	// Chat model for content generation (LLM calls).
	Chat chat.Chat
	// Language hint for output ("zh", "en").
	Language string
}

// Result is returned by a Generator on success.
type Result struct {
	// MIME type of the produced file (e.g. "text/markdown",
	// "application/pdf", "image/png", "application/vnd.openxmlformats-
	// officedocument.presentationml.presentation").
	MIMEType string
	// FileExtension without the dot (e.g. "md", "pdf", "png", "pptx").
	FileExtension string
	// Bytes is the generated file content.
	Bytes []byte
	// Extra metadata to merge back into the artifact row (slide count,
	// page count, chart spec, etc.).
	ExtraMetadata map[string]any
}

// Generator produces a single artifact type. Implementations must be safe
// for concurrent use.
type Generator interface {
	// Type returns the ArtifactType this generator handles.
	Type() types.ArtifactType
	// Generate runs the generation and returns the result.
	Generate(ctx context.Context, in *Input) (*Result, error)
}

// Registry holds generators keyed by ArtifactType.
type Registry struct {
	generators map[types.ArtifactType]Generator
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{generators: map[types.ArtifactType]Generator{}}
}

// Register registers a generator. It panics if a generator is already
// registered for the type (defends against double-init at startup).
func (r *Registry) Register(g Generator) {
	if _, ok := r.generators[g.Type()]; ok {
		panic("artifact: generator already registered for type " + string(g.Type()))
	}
	r.generators[g.Type()] = g
}

// Get returns the generator for the given type, or an error if none is
// registered.
func (r *Registry) Get(t types.ArtifactType) (Generator, error) {
	g, ok := r.generators[t]
	if !ok {
		return nil, fmt.Errorf("artifact: no generator registered for type %q", t)
	}
	return g, nil
}
