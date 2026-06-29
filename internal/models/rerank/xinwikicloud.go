package rerank

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

const weKnoraCloudRerankPath = "/api/v1/rerank"

// XinWikiCloudReranker 实现 rerank.Reranker 接口，对接 XinWikiCloud /api/v1/rerank
type XinWikiCloudReranker struct {
	modelName       string
	remoteModelName string
	modelID         string
	appID           string
	apiKey          string
	baseURL         string
	client          *http.Client
}

// NewXinWikiCloudReranker 构造 XinWikiCloudReranker
func NewXinWikiCloudReranker(config *RerankerConfig) (*XinWikiCloudReranker, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("XinWikiCloud reranker: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("XinWikiCloud reranker: AppSecret is required")
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	return &XinWikiCloudReranker{
		modelName:       config.ModelName,
		remoteModelName: remoteModelName,
		modelID:         config.ModelID,
		appID:           config.AppID,
		apiKey:          config.AppSecret,
		baseURL:         strings.TrimRight(config.BaseURL, "/"),
		client:          &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type weKnoraCloudRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type weKnoraCloudRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document"`
	} `json:"results"`
}

func (r *XinWikiCloudReranker) Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error) {
	reqBody := weKnoraCloudRerankRequest{
		Model:     r.effectiveModelName(),
		Query:     query,
		Documents: documents,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud reranker: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(r.appID, r.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+weKnoraCloudRerankPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud reranker: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud reranker: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xinwikicloud reranker: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xinwikicloud reranker: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var rerankResp weKnoraCloudRerankResponse
	if err := json.Unmarshal(respBytes, &rerankResp); err != nil {
		return nil, fmt.Errorf("xinwikicloud reranker: unmarshal: %w", err)
	}

	results := make([]RankResult, 0, len(rerankResp.Results))
	for _, item := range rerankResp.Results {
		results = append(results, RankResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
			Document:       DocumentInfo{Text: item.Document.Text},
		})
	}
	return results, nil
}

func (r *XinWikiCloudReranker) effectiveModelName() string {
	if r.remoteModelName != "" {
		return r.remoteModelName
	}
	return r.modelName
}

func (r *XinWikiCloudReranker) GetModelName() string { return r.modelName }
func (r *XinWikiCloudReranker) GetModelID() string   { return r.modelID }
