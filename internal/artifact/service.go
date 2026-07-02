package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/artifact/generator"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

const (
	maxConcurrentGenerations = 4
	generationTimeout        = 5 * time.Minute
)

type GenerationService struct {
	registry     *generator.Registry
	artifacts    interfaces.ArtifactService
	files        interfaces.FileService
	modelService interfaces.ModelService
	wikiPages    interfaces.WikiPageService

	mu        sync.Mutex
	inFlight  map[string]context.CancelFunc
	sem       chan struct{}
}

func NewGenerationService(
	registry *generator.Registry,
	artifacts interfaces.ArtifactService,
	files interfaces.FileService,
	modelService interfaces.ModelService,
	wikiPages interfaces.WikiPageService,
) *GenerationService {
	return &GenerationService{
		registry:     registry,
		artifacts:    artifacts,
		files:        files,
		modelService: modelService,
		wikiPages:    wikiPages,
		inFlight:     make(map[string]context.CancelFunc),
		sem:          make(chan struct{}, maxConcurrentGenerations),
	}
}

var errGenerationInProgress = fmt.Errorf("artifact/generate: generation already in progress for this artifact")

func (s *GenerationService) Generate(parentCtx context.Context, artifactID string, tenantID uint64, userID, prompt string) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(parentCtx, "[Artifact] panic during generation %s: %v", artifactID, r)
			retErr = fmt.Errorf("artifact/generate: internal panic: %v", r)
		}
	}()

	s.mu.Lock()
	if _, busy := s.inFlight[artifactID]; busy {
		s.mu.Unlock()
		return errGenerationInProgress
	}
	runCtx, cancel := context.WithTimeout(parentCtx, generationTimeout)
	s.inFlight[artifactID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.inFlight, artifactID)
		s.mu.Unlock()
		cancel()
	}()

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-runCtx.Done():
		return fmt.Errorf("artifact/generate: timed out waiting for concurrency slot")
	}

	caller := interfaces.ArtifactCaller{UserID: userID}
	art, err := s.artifacts.Get(runCtx, tenantID, caller, artifactID)
	if err != nil {
		return fmt.Errorf("artifact/generate: load artifact: %w", err)
	}
	gen, err := s.registry.Get(art.Type)
	if err != nil {
		return s.failArtifact(runCtx, art, tenantID, caller, err)
	}
	chatModel, err := s.resolveChatModel(runCtx)
	if err != nil {
		return s.failArtifact(runCtx, art, tenantID, caller, err)
	}
	in, err := s.buildInput(runCtx, art, prompt, chatModel)
	if err != nil {
		return s.failArtifact(runCtx, art, tenantID, caller, err)
	}
	res, err := gen.Generate(runCtx, in)
	if err != nil {
		return s.failArtifact(runCtx, art, tenantID, caller, err)
	}
	fileName := fmt.Sprintf("%s-%s.%s", art.Type, art.ID[:12], res.FileExtension)
	path, err := s.files.SaveBytes(runCtx, res.Bytes, tenantID, fileName, false)
	if err != nil {
		return s.failArtifact(runCtx, art, tenantID, caller, fmt.Errorf("save file: %w", err))
	}
	if err := s.artifacts.UpdateStatus(runCtx, tenantID, caller, art.ID,
		types.ArtifactStatusReady, path, int64(len(res.Bytes))); err != nil {
		return fmt.Errorf("artifact/generate: update status: %w", err)
	}
	logger.Infof(runCtx, "[Artifact] Generated %s artifact %s (%d bytes)", art.Type, art.ID, len(res.Bytes))
	return nil
}

// GenerateByStringIDs is a convenience wrapper for task payloads that come
// in as strings (asynq tasks serialize IDs as strings).
func (s *GenerationService) GenerateByStringIDs(ctx context.Context, artifactID, tenantID, userID, prompt string) error {
	tid, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		return fmt.Errorf("artifact/generate: invalid tenant id %q: %w", tenantID, err)
	}
	return s.Generate(ctx, artifactID, tid, userID, prompt)
}

// resolveChatModel picks a default KnowledgeQA chat model from the model
// service. It lists all models and takes the first KnowledgeQA model it
// finds (suitable for offline/background generation where the caller did
// not specify a model).
func (s *GenerationService) resolveChatModel(ctx context.Context) (chat.Chat, error) {
	if s.modelService == nil {
		return nil, fmt.Errorf("artifact/generate: model service is not configured")
	}
	models, err := s.modelService.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	var chatModelID string
	for _, m := range models {
		if m == nil {
			continue
		}
		if m.Type == types.ModelTypeKnowledgeQA {
			chatModelID = m.ID
			break
		}
	}
	if chatModelID == "" {
		return nil, fmt.Errorf("artifact/generate: no chat (KnowledgeQA) model available")
	}
	return s.modelService.GetChatModel(ctx, chatModelID)
}

func (s *GenerationService) buildInput(ctx context.Context, art *types.Artifact, promptOverride string, chatModel chat.Chat) (*generator.Input, error) {
	prompt := promptOverride
	if prompt == "" {
		if art.Metadata != nil {
			var meta map[string]any
			if err := json.Unmarshal(art.Metadata, &meta); err == nil {
				if p, ok := meta["prompt"].(string); ok && p != "" {
					prompt = p
				}
			}
		}
	}
	if prompt == "" && art.Title != "" {
		prompt = "Generate: " + art.Title
	}
	if prompt == "" {
		prompt = "Generate output"
	}
	var sourceText string
	var citations []generator.Citation
	if art.SourceWikiPageID != "" && s.wikiPages != nil {
		if page, err := s.wikiPages.GetPageByID(ctx, art.SourceWikiPageID); err == nil && page != nil {
			sourceText = "# Wiki Page: " + page.Title + "\n" + page.Content
			citations = generator.BuildCitationsFromWikiPage(page)
		}
	}
	return &generator.Input{
		Prompt:     prompt,
		SourceText: sourceText,
		Artifact:   art,
		Chat:       chatModel,
		Language:   "zh",
		Citations:  citations,
	}, nil
}

func (s *GenerationService) failArtifact(ctx context.Context, art *types.Artifact, tenantID uint64, caller interfaces.ArtifactCaller, genErr error) error {
	logger.Errorf(ctx, "[Artifact] Generation failed for %s (%s): %v", art.ID, art.Type, genErr)
	_ = s.artifacts.UpdateStatus(ctx, tenantID, caller, art.ID, types.ArtifactStatusFailed, "", 0)
	return genErr
}

// RegisterDefaultGenerators populates the registry with the built-in
// generators (Markdown, Report, Chart, PPT).
func RegisterDefaultGenerators(r *generator.Registry) {
	r.Register(generator.NewMarkdownGenerator())
	r.Register(generator.NewReportGenerator())
	r.Register(generator.NewChartGenerator())
	r.Register(generator.NewPPTGenerator())
}
