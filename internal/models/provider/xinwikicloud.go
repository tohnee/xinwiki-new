package provider

import "github.com/Tencent/XinWiki/internal/types"

const (
	ProviderXinWikiCloud ProviderName = "xinwikicloud"

	// XinWikiCloudBaseURL XinWikiCloud 服务硬编码 Base URL（统一入口，路径由各实现拼接）
	XinWikiCloudBaseURL = "https://xinwiki.weixin.qq.com"
)

type XinWikiCloudProvider struct{}

func init() {
	Register(&XinWikiCloudProvider{})
}

func (p *XinWikiCloudProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderXinWikiCloud,
		DisplayName: "XinWikiCloud",
		Description: "XinWiki云服务，模型：chat, embedding, rerank, vlm",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: XinWikiCloudBaseURL,
			types.ModelTypeEmbedding:   XinWikiCloudBaseURL,
			types.ModelTypeRerank:      XinWikiCloudBaseURL,
			types.ModelTypeVLLM:        XinWikiCloudBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
		},
		RequiresAuth: true,
	}
}

func (p *XinWikiCloudProvider) ValidateConfig(config *Config) error {
	// AppID/AppSecret 通过专用初始化接口写入，此处仅做结构校验。
	// 其中 AppSecret 字段当前实际承载上游 API Key。
	return nil
}
