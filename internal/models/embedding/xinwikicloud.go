package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/models/utils"
	"github.com/google/uuid"
)

const weKnoraCloudEmbedPath = "/api/v1/embeddings"

// XinWikiCloudEmbedder 实现 embedding.Embedder 接口，对接 XinWikiCloud /api/v1/embeddings
type XinWikiCloudEmbedder struct {
	modelName                 string
	remoteModelName           string
	modelID                   string
	appID                     string
	apiKey                    string
	baseURL                   string
	dimensions                int
	supportsDimensionOverride bool
	client                    *http.Client
	EmbedderPooler
}

// NewXinWikiCloudEmbedder 构造 XinWikiCloudEmbedder
func NewXinWikiCloudEmbedder(config Config) (*XinWikiCloudEmbedder, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("XinWikiCloud embedder: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("XinWikiCloud embedder: AppSecret is required")
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	return &XinWikiCloudEmbedder{
		modelName:                 config.ModelName,
		remoteModelName:           remoteModelName,
		modelID:                   config.ModelID,
		appID:                     config.AppID,
		apiKey:                    config.AppSecret,
		baseURL:                   strings.TrimRight(config.BaseURL, "/"),
		dimensions:                config.Dimensions,
		supportsDimensionOverride: config.SupportsDimensionOverride,
		client:                    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type weKnoraCloudEmbedRequest struct {
	Model                string   `json:"model"`
	Input                []string `json:"input"`
	Dimensions           int      `json:"dimensions,omitempty"`
	TruncatePromptTokens int      `json:"truncate_prompt_tokens,omitempty"`
}

type weKnoraCloudEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *XinWikiCloudEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("xinwikicloud embedder: empty response")
	}
	return results[0], nil
}

func (e *XinWikiCloudEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := weKnoraCloudEmbedRequest{Model: e.effectiveModelName(), Input: texts}
	if e.supportsDimensionOverride && e.dimensions > 0 {
		reqBody.Dimensions = e.dimensions
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud embedder: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(e.appID, e.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+weKnoraCloudEmbedPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud embedder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud embedder: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud embedder: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xinwikicloud embedder: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var embedResp weKnoraCloudEmbedResponse
	if err := json.Unmarshal(respBytes, &embedResp); err != nil {
		return nil, fmt.Errorf("xinwikicloud embedder: unmarshal: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, item := range embedResp.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}

func (e *XinWikiCloudEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(ctx, texts)
}

func (e *XinWikiCloudEmbedder) SetSupportsDimensionOverride(supported bool) {
	e.supportsDimensionOverride = supported
}

func (e *XinWikiCloudEmbedder) effectiveModelName() string {
	if e.remoteModelName != "" {
		return e.remoteModelName
	}
	return e.modelName
}

func (e *XinWikiCloudEmbedder) GetModelName() string { return e.modelName }
func (e *XinWikiCloudEmbedder) GetModelID() string   { return e.modelID }
func (e *XinWikiCloudEmbedder) GetDimensions() int   { return e.dimensions }
