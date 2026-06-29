package types

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// WikiCategoryMaxDepth is the hard cap on how many folder levels a wiki page's
// category_path may keep. The ingest prompts intentionally ask the model for at
// most 2 levels to keep the directory tree shallow; this storage cap is one
// level deeper as a defensive bound so a slightly over-eager model (or a
// reconcile remap) cannot create an unbounded breadcrumb. It is the single
// source of truth shared by the service, repository, and taxonomy layers so
// stored paths and queried paths are always cleaned identically.
const WikiCategoryMaxDepth = 3

var wikiCategorySeparatorReplacer = strings.NewReplacer("／", "/", "｜", "/", "|", "/")

// CleanWikiCategoryPart normalizes a single raw category label that may itself
// carry embedded separators, wrapping quotes/brackets, or page-type noise, and
// returns the cleaned sub-labels (type labels such as "entity"/"实体" dropped).
func CleanWikiCategoryPart(part string) []string {
	part = strings.TrimSpace(part)
	if part == "" {
		return nil
	}
	part = wikiCategorySeparatorReplacer.Replace(part)
	rawParts := strings.Split(part, "/")
	cleaned := make([]string, 0, len(rawParts))
	for _, raw := range rawParts {
		label := strings.TrimSpace(raw)
		label = strings.Trim(label, `"'“”‘’[]（）()`)
		label = strings.TrimSpace(label)
		if label == "" || isWikiTypeCategoryLabel(label) {
			continue
		}
		cleaned = append(cleaned, label)
	}
	return cleaned
}

// CleanWikiCategoryPath cleans, deduplicates, and caps a full category path at
// WikiCategoryMaxDepth. Centralizing this guarantees that the path a page is
// stored with and the path a list/filter query is matched against go through
// the exact same normalization, so directory filters cannot silently drift.
func CleanWikiCategoryPath(parts []string) []string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		for _, label := range CleanWikiCategoryPart(part) {
			if containsWikiString(cleaned, label) {
				continue
			}
			cleaned = append(cleaned, label)
			if len(cleaned) >= WikiCategoryMaxDepth {
				return cleaned
			}
		}
	}
	return cleaned
}

// SplitWikiPageTypes parses a page_type value that may carry several
// comma-separated types (e.g. "entity,concept") into a deduplicated slice,
// dropping blanks. An empty/whitespace-only input yields nil ("no filter").
// Shared by the handler (query parsing) and repository (List filter) so the
// two layers split identically.
func SplitWikiPageTypes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func isWikiTypeCategoryLabel(label string) bool {
	normalized := strings.ToLower(strings.TrimSpace(label))
	normalized = strings.TrimSuffix(normalized, "s")
	switch normalized {
	case "entity", "实体", "實體", "concept", "概念", "summary", "摘要", "wiki", "页面", "頁面":
		return true
	default:
		return false
	}
}

func containsWikiString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// WikiPageType constants define the types of wiki pages
const (
	// WikiPageTypeSummary represents a document summary page
	WikiPageTypeSummary = "summary"
	// WikiPageTypeEntity represents an entity page (person, organization, place, etc.)
	WikiPageTypeEntity = "entity"
	// WikiPageTypeConcept represents a concept/topic page
	WikiPageTypeConcept = "concept"
	// WikiPageTypeIndex represents the wiki index page (index.md)
	WikiPageTypeIndex = "index"
	// WikiPageTypeLog represents the operation log page (log.md)
	WikiPageTypeLog = "log"
	// WikiPageTypeSynthesis represents a synthesis/analysis page.
	// NOT auto-created by ingest — Agent creates these via wiki_write_page tool
	// when it generates cross-document analysis, trends, or insights during conversations.
	WikiPageTypeSynthesis = "synthesis"
	// WikiPageTypeComparison represents a comparison page.
	// NOT auto-created by ingest — Agent creates these via wiki_write_page tool
	// when the user asks to compare entities, concepts, or approaches.
	WikiPageTypeComparison = "comparison"
)

// WikiPageStatus constants
const (
	// WikiPageStatusDraft indicates the page is a draft
	WikiPageStatusDraft = "draft"
	// WikiPageStatusReviewing indicates the page is under review
	WikiPageStatusReviewing = "reviewing"
	// WikiPageStatusPublished indicates the page is published and visible
	WikiPageStatusPublished = "published"
	// WikiPageStatusDeprecated indicates the page is deprecated but still visible
	WikiPageStatusDeprecated = "deprecated"
	// WikiPageStatusSuperseded indicates the page has been superseded by a newer page
	WikiPageStatusSuperseded = "superseded"
	// WikiPageStatusArchived indicates the page is archived
	WikiPageStatusArchived = "archived"
)

var validWikiPageStatuses = map[string]bool{
	WikiPageStatusDraft:      true,
	WikiPageStatusReviewing:  true,
	WikiPageStatusPublished:  true,
	WikiPageStatusDeprecated: true,
	WikiPageStatusSuperseded: true,
	WikiPageStatusArchived:   true,
}

var wikiPageStatusTransitions = map[string]map[string]bool{
	WikiPageStatusDraft: {
		WikiPageStatusDraft:     true,
		WikiPageStatusReviewing: true,
	},
	WikiPageStatusReviewing: {
		WikiPageStatusReviewing: true,
		WikiPageStatusDraft:     true,
		WikiPageStatusPublished: true,
	},
	WikiPageStatusPublished: {
		WikiPageStatusPublished:  true,
		WikiPageStatusDeprecated: true,
		WikiPageStatusSuperseded: true,
	},
	WikiPageStatusDeprecated: {
		WikiPageStatusDeprecated: true,
		WikiPageStatusArchived:   true,
	},
	WikiPageStatusSuperseded: {
		WikiPageStatusSuperseded: true,
		WikiPageStatusArchived:   true,
	},
	WikiPageStatusArchived: {
		WikiPageStatusArchived: true,
	},
}

// InvalidWikiStatusTransitionError is returned when a wiki page status
// transition is not allowed by the state machine.
type InvalidWikiStatusTransitionError struct {
	From string
	To   string
}

func (e *InvalidWikiStatusTransitionError) Error() string {
	return "invalid wiki page status transition: " + e.From + " -> " + e.To
}

// IsValidWikiPageStatus reports whether s is a recognized wiki page status.
func IsValidWikiPageStatus(s string) bool {
	return validWikiPageStatuses[s]
}

// ValidateWikiPageStatusTransition checks whether transitioning from status
// `from` to status `to` is allowed. Returns nil if the transition is valid,
// or *InvalidWikiStatusTransitionError if it is not.
func ValidateWikiPageStatusTransition(from, to string) error {
	if !IsValidWikiPageStatus(from) {
		return &InvalidWikiStatusTransitionError{From: from, To: to}
	}
	if !IsValidWikiPageStatus(to) {
		return &InvalidWikiStatusTransitionError{From: from, To: to}
	}
	allowed, ok := wikiPageStatusTransitions[from]
	if !ok {
		return &InvalidWikiStatusTransitionError{From: from, To: to}
	}
	if !allowed[to] {
		return &InvalidWikiStatusTransitionError{From: from, To: to}
	}
	return nil
}

// NormalizeWikiPageStatus normalizes a wiki page status string, returning
// WikiPageStatusPublished as the default for empty or unknown values.
func NormalizeWikiPageStatus(s string) string {
	s = strings.TrimSpace(s)
	if IsValidWikiPageStatus(s) {
		return s
	}
	return WikiPageStatusPublished
}

// CanTransitionTo reports whether the page can transition to the given status.
func (p *WikiPage) CanTransitionTo(to string) bool {
	return ValidateWikiPageStatusTransition(p.Status, to) == nil
}

// --- Milestone C: Confidence, Quality & Freshness Constants ---

// Criticality Level constants (P0 = most critical, P3 = least critical)
const (
	CriticalityP0 = "P0"
	CriticalityP1 = "P1"
	CriticalityP2 = "P2"
	CriticalityP3 = "P3"
)

var validCriticalityLevels = map[string]bool{
	CriticalityP0: true,
	CriticalityP1: true,
	CriticalityP2: true,
	CriticalityP3: true,
}

// Freshness State constants
const (
	FreshnessActive   = "active"
	FreshnessWarm     = "warm"
	FreshnessCold     = "cold"
	FreshnessArchived = "archived"
)

var validFreshnessStates = map[string]bool{
	FreshnessActive:   true,
	FreshnessWarm:     true,
	FreshnessCold:     true,
	FreshnessArchived: true,
}

// Score weight constants for confidence calculation
const (
	WeightSourceAuthority     = 0.30
	WeightEvidenceSupport     = 0.20
	WeightRecency             = 0.20
	WeightConsistency         = 0.15
	WeightExpertValidation    = 0.10
	WeightUsageFeedback       = 0.05
)

// Quality dimension weights
const (
	WeightContentCompleteness = 0.20
	WeightSourceReliability   = 0.20
	WeightTimeliness          = 0.15
	WeightReadability         = 0.15
	WeightCitationSufficiency = 0.15
	WeightQualityFeedback     = 0.15
)

// Freshness time thresholds (days)
const (
	FreshnessActiveThresholdDays = 30
	FreshnessWarmThresholdDays   = 90
	FreshnessColdThresholdDays   = 180
)

// Retrieval boost values by freshness state
const (
	RetrievalBoostActive   = 1.0
	RetrievalBoostWarm     = 0.8
	RetrievalBoostCold     = 0.5
	RetrievalBoostArchived = 0.0
	RetrievalBoostDeprecated = 0.3
	RetrievalBoostSuperseded = 0.1
)

// Penalty constants
const (
	ContradictionPenaltyPerItem = 0.1
	MaxContradictionPenalty     = 0.5
	StalenessPenaltyFactor      = 0.003
	MinScore                    = 0.0
	MaxScore                    = 1.0
)

// IsValidCriticalityLevel reports whether s is a recognized criticality level
func IsValidCriticalityLevel(s string) bool {
	return validCriticalityLevels[s]
}

// NormalizeCriticalityLevel normalizes a criticality level string
func NormalizeCriticalityLevel(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if IsValidCriticalityLevel(s) {
		return s
	}
	return CriticalityP3
}

// IsValidFreshnessState reports whether s is a recognized freshness state
func IsValidFreshnessState(s string) bool {
	return validFreshnessStates[s]
}

// NormalizeFreshnessState normalizes a freshness state string
func NormalizeFreshnessState(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if IsValidFreshnessState(s) {
		return s
	}
	return FreshnessActive
}

// ClampScore clamps a score value between 0.0 and 1.0
func ClampScore(score float64) float64 {
	if score < MinScore {
		return MinScore
	}
	if score > MaxScore {
		return MaxScore
	}
	return score
}

// WikiPage represents a single wiki page in a wiki knowledge base.
// Wiki pages are LLM-generated, interlinked markdown documents that form
// a persistent, compounding knowledge artifact.
type WikiPage struct {
	// Unique identifier (UUID)
	ID string `json:"id" gorm:"type:varchar(36);primaryKey"`
	// Tenant ID for multi-tenant isolation
	TenantID uint64 `json:"tenant_id" gorm:"index"`
	// Knowledge base this page belongs to
	KnowledgeBaseID string `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	// URL-friendly slug for addressing, e.g. "entity/acme-corp", "concept/rag"
	// Unique within a knowledge base
	Slug string `json:"slug" gorm:"type:varchar(255);uniqueIndex:idx_kb_slug"`
	// Human-readable title
	Title string `json:"title" gorm:"type:varchar(512)"`
	// Page type: summary, entity, concept, index, log, synthesis, comparison
	PageType string `json:"page_type" gorm:"type:varchar(32);index"`
	// Page status: draft, published, archived
	Status string `json:"status" gorm:"type:varchar(32);default:'published'"`
	// SecurityLevel records the highest source classification inherited by this
	// derived wiki page. Empty legacy values are normalized to L1.
	SecurityLevel string `json:"security_level" gorm:"type:varchar(16);not null;default:'L1';index"`
	// AllowedUserIDs is the optional intersection of source user ACLs. Empty
	// does not mean public; tenant / KB RBAC still applies at read time.
	AllowedUserIDs StringArray `json:"allowed_user_ids" gorm:"type:json"`
	// AllowedGroupIDs is the optional intersection of source group ACLs. Empty
	// does not mean public; tenant / KB RBAC still applies at read time.
	AllowedGroupIDs StringArray `json:"allowed_group_ids" gorm:"type:json"`
	// Full markdown content
	Content string `json:"content" gorm:"type:text"`
	// One-line summary for index listing
	Summary string `json:"summary" gorm:"type:text"`
	// Alternate names, abbreviations, acronyms or translated names
	Aliases StringArray `json:"aliases" gorm:"type:json"`
	// ParentSlug optionally points at the wiki page that should act as this
	// page's semantic parent in the directory tree. The parent may be empty
	// when the page is grouped only by FolderID.
	ParentSlug string `json:"parent_slug,omitempty" gorm:"type:varchar(255);index"`
	// FolderID is the single source of truth for where this page sits in the
	// directory tree — a reference to wiki_folders.id ("" = wiki root). The
	// CategoryPath / WikiPath / Depth fields below are denormalized caches
	// recomputed from this folder's chain on every write so list/index/search
	// queries don't have to join wiki_folders.
	FolderID string `json:"folder_id,omitempty" gorm:"column:folder_id;type:varchar(36);index;default:''"`
	// CategoryPath is the directory breadcrumb that groups this page in the
	// wiki browser, e.g. ["AI", "LLM 应用", "RAG"]. Derived cache of the
	// folder chain identified by FolderID.
	CategoryPath StringArray `json:"category_path,omitempty" gorm:"type:json"`
	// WikiPath is a normalized, sortable path derived from page_type,
	// category_path, and title. It keeps large directory listings cheap to sort.
	WikiPath string `json:"wiki_path,omitempty" gorm:"type:varchar(1024);index"`
	// Depth is len(CategoryPath), cached for filtering / display.
	Depth int `json:"depth,omitempty" gorm:"default:0;index"`
	// SortOrder allows generated or manually edited pages to control sibling
	// ordering before falling back to title.
	SortOrder int `json:"sort_order,omitempty" gorm:"default:0;index"`
	// References to source knowledge IDs that contributed to this page.
	// Format matches the legacy "<knowledge_id>|<doc_title>" convention used
	// across the ingest pipeline, so retract / display code can split on `|`
	// to recover the title. Document-level granularity.
	SourceRefs StringArray `json:"source_refs" gorm:"type:json"`
	// ChunkRefs records the specific source-document chunks this page was
	// built from — one UUID per cited chunk. Populated during ingest from
	// the chunk-citation pass; refreshed wholesale whenever the page is
	// re-materialized. Empty for summary pages (they are document-level
	// synopses and don't carry chunk-level citations). Use this when you
	// need to surface the underlying evidence for a wiki page, or to
	// retract citations when a source document is deleted.
	ChunkRefs StringArray `json:"chunk_refs" gorm:"type:json"`
	// Slugs of pages that link TO this page (backlinks)
	InLinks StringArray `json:"in_links" gorm:"type:json"`
	// Slugs of pages this page links to (outbound links)
	OutLinks StringArray `json:"out_links" gorm:"type:json"`
	// Arbitrary metadata (tags, categories, dates, etc.)
	PageMetadata JSON `json:"page_metadata" gorm:"column:page_metadata;type:json"`
	// Version number. Incremented only when a user-visible content field
	// (title, content, summary, page_type, status) actually changes; pure
	// bookkeeping writes (link maintenance, same-content re-ingest, status
	// sync from background jobs) leave it untouched so it can be used as a
	// real "the page was edited" signal.
	Version int `json:"version" gorm:"default:1"`

	// --- Milestone C: Confidence, Quality & Freshness ---
	// CriticalityLevel indicates how critical this page is (P0/P1/P2/P3)
	// P0/P1 pages are never automatically archived
	CriticalityLevel string `json:"criticality_level" gorm:"type:varchar(8);default:'P3';index"`
	// LastAccessedAt records when the page was last viewed/used
	LastAccessedAt time.Time `json:"last_accessed_at"`
	// ViewCount tracks total page views
	ViewCount int64 `json:"view_count" gorm:"default:0"`
	// PositiveFeedback counts positive user feedback (helpful votes)
	PositiveFeedback int64 `json:"positive_feedback" gorm:"default:0"`
	// NegativeFeedback counts negative user feedback (not helpful votes)
	NegativeFeedback int64 `json:"negative_feedback" gorm:"default:0"`
	// ExpertValidated indicates whether an expert has reviewed/validated this page
	ExpertValidated bool `json:"expert_validated" gorm:"default:false"`
	// ExpertValidatedAt records when expert validation occurred
	ExpertValidatedAt *time.Time `json:"expert_validated_at,omitempty"`
	// ContradictionCount tracks number of detected contradictions
	ContradictionCount int `json:"contradiction_count" gorm:"default:0"`

	// Confidence score fields (0.0 - 1.0)
	ConfidenceScore     float64 `json:"confidence_score" gorm:"type:decimal(5,4);default:0.5"`
	SourceAuthority     float64 `json:"source_authority" gorm:"type:decimal(5,4);default:0.5"`
	EvidenceSupport     float64 `json:"evidence_support" gorm:"type:decimal(5,4);default:0.5"`
	RecencyScore        float64 `json:"recency_score" gorm:"type:decimal(5,4);default:1.0"`
	ConsistencyScore    float64 `json:"consistency_score" gorm:"type:decimal(5,4);default:1.0"`
	ExpertValidation    float64 `json:"expert_validation" gorm:"type:decimal(5,4);default:0.0"`
	UsageFeedback       float64 `json:"usage_feedback" gorm:"type:decimal(5,4);default:0.5"`
	ContradictionPenalty float64 `json:"contradiction_penalty" gorm:"type:decimal(5,4);default:1.0"`
	StalenessPenalty    float64 `json:"staleness_penalty" gorm:"type:decimal(5,4);default:1.0"`
	FinalScore          float64 `json:"final_score" gorm:"type:decimal(5,4);default:0.5;index"`

	// Quality score fields (0.0 - 1.0)
	QualityScore        float64 `json:"quality_score" gorm:"type:decimal(5,4);default:0.5"`
	ContentCompleteness float64 `json:"content_completeness" gorm:"type:decimal(5,4);default:0.5"`
	SourceReliability   float64 `json:"source_reliability" gorm:"type:decimal(5,4);default:0.5"`
	TimelinessScore     float64 `json:"timeliness_score" gorm:"type:decimal(5,4);default:1.0"`
	ReadabilityScore    float64 `json:"readability_score" gorm:"type:decimal(5,4);default:0.5"`
	CitationSufficiency float64 `json:"citation_sufficiency" gorm:"type:decimal(5,4);default:0.5"`

	// Freshness state: active, warm, cold, archived
	FreshnessState string `json:"freshness_state" gorm:"type:varchar(16);default:'active';index"`
	// RetrievalBoost is the multiplier applied during search (0.0 - 1.0)
	RetrievalBoost float64 `json:"retrieval_boost" gorm:"type:decimal(5,4);default:1.0;index"`
	// ScoreLastCalculatedAt records when scores were last computed
	ScoreLastCalculatedAt time.Time `json:"score_last_calculated_at"`

	// Creation time
	CreatedAt time.Time `json:"created_at"`
	// Last update time
	UpdatedAt time.Time `json:"updated_at"`
	// Soft delete
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// NormalizeACLDefaults applies conservative defaults for wiki page ACL metadata.
// Empty ACL arrays are not public grants; they preserve the existing tenant / KB
// RBAC behavior until explicit derived ACL propagation narrows them.
func (p *WikiPage) NormalizeACLDefaults() {
	if p == nil {
		return
	}
	if strings.TrimSpace(p.SecurityLevel) == "" {
		p.SecurityLevel = SecurityLevelL1
	}
	if p.AllowedUserIDs == nil {
		p.AllowedUserIDs = StringArray{}
	}
	if p.AllowedGroupIDs == nil {
		p.AllowedGroupIDs = StringArray{}
	}
}

// TableName specifies the database table name
func (WikiPage) TableName() string {
	return "wiki_pages"
}

// WikiFolderRootID is the sentinel parent/folder id meaning "the wiki root"
// (a page or folder directly under the top level, with no parent folder).
const WikiFolderRootID = ""

// WikiFolder is a first-class directory node in the wiki browser. Folders
// exist independently of pages — an empty folder persists so users can lay
// out a skeleton and file pages into it later. The tree is an adjacency list
// (ParentID, "" = root); Path is the materialized "/"-joined name chain kept
// purely for cheap display/sort. A wiki page's placement is WikiPage.FolderID.
type WikiFolder struct {
	ID              string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64         `json:"tenant_id" gorm:"index"`
	KnowledgeBaseID string         `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	ParentID        string         `json:"parent_id" gorm:"column:parent_id;type:varchar(36);index;default:''"`
	Name            string         `json:"name" gorm:"type:varchar(255)"`
	Path            string         `json:"path" gorm:"type:varchar(1024)"`
	Depth           int            `json:"depth" gorm:"default:0"`
	SortOrder       int            `json:"sort_order" gorm:"default:0"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName specifies the database table name
func (WikiFolder) TableName() string {
	return "wiki_folders"
}

// WikiFolderNode is one directory node returned to the browser, enriched with
// the live page count directly under it and whether it has child folders so
// the UI can render an expand affordance without a second round-trip.
type WikiFolderNode struct {
	WikiFolder
	PageCount   int64 `json:"page_count"`
	HasChildren bool  `json:"has_children"`
}

// WikiFolderListResponse is the payload for listing the direct children of a
// folder (parent_id="" = root level).
type WikiFolderListResponse struct {
	ParentID string           `json:"parent_id"`
	Folders  []WikiFolderNode `json:"folders"`
}

// WikiFolderCreateRequest creates a new (initially empty) folder under ParentID.
type WikiFolderCreateRequest struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
}

// WikiFolderUpdateRequest renames and/or reparents a folder. ParentID is
// applied only when MoveParent is true so a pure rename doesn't have to
// re-send the (possibly root "") parent and risk an accidental move.
type WikiFolderUpdateRequest struct {
	Name       string `json:"name,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	MoveParent bool   `json:"move_parent,omitempty"`
}

// WikiPageMoveRequest relocates the page identified by Slug into FolderID
// ("" = root). Slug is carried in the body (not the path) because wiki slugs
// are hierarchical ("entity/acme") and would collide with gin's catch-all.
type WikiPageMoveRequest struct {
	Slug     string `json:"slug" binding:"required"`
	FolderID string `json:"folder_id"`
}

// WikiExtractionGranularity controls how aggressive Pass 0 (candidate slug
// extraction) is. Higher granularity = more slugs, lower = tighter focus on
// the document's main subjects.
type WikiExtractionGranularity string

const (
	// WikiExtractionFocused keeps only the document's main subjects (e.g.
	// a resume yields the person + their projects, nothing else). Most
	// aggressive slug pruning; avoids index bloat from incidental technology
	// names and generic concepts.
	WikiExtractionFocused WikiExtractionGranularity = "focused"

	// WikiExtractionStandard is the default: main subjects plus entities /
	// concepts that are substantively discussed (a dedicated paragraph or
	// multiple bullet points). Skips one-off mentions and commodity terms.
	WikiExtractionStandard WikiExtractionGranularity = "standard"

	// WikiExtractionExhaustive extracts every named entity and recognizable
	// concept, including stacks/libs mentioned in passing. Matches the
	// pre-granularity behavior. Useful when the KB is being used as a
	// glossary rather than a curated wiki.
	WikiExtractionExhaustive WikiExtractionGranularity = "exhaustive"
)

// IsValid reports whether g is one of the three recognized levels.
func (g WikiExtractionGranularity) IsValid() bool {
	switch g {
	case WikiExtractionFocused, WikiExtractionStandard, WikiExtractionExhaustive:
		return true
	}
	return false
}

// Normalize returns g if valid, otherwise WikiExtractionStandard. Callers
// pipe config through this so historical rows with empty / unknown values
// don't surprise the extraction prompt.
func (g WikiExtractionGranularity) Normalize() WikiExtractionGranularity {
	if g.IsValid() {
		return g
	}
	return WikiExtractionStandard
}

// WikiConfig stores wiki-specific configuration for a knowledge base.
// Applicable to document-type knowledge bases with wiki feature enabled.
// Whether the wiki feature is turned on is controlled by IndexingStrategy.WikiEnabled;
// this struct only carries wiki-specific tunables.
type WikiConfig struct {
	// SynthesisModelID is the LLM model ID used for wiki page generation and updates
	SynthesisModelID string `yaml:"synthesis_model_id" json:"synthesis_model_id"`
	// MaxPagesPerIngest limits pages created/updated per ingest operation (0 = no limit)
	MaxPagesPerIngest int `yaml:"max_pages_per_ingest" json:"max_pages_per_ingest"`
	// ExtractionGranularity controls how many candidate slugs Pass 0 extracts
	// per document. Empty / unknown value is treated as WikiExtractionStandard.
	ExtractionGranularity WikiExtractionGranularity `yaml:"extraction_granularity" json:"extraction_granularity,omitempty"`

	// IngestBatchSize controls how many pending ops a single batch
	// processes before scheduling a follow-up. 0 falls back to the
	// hard-coded default (5). Operators on large KBs (4w+ docs) can
	// raise this to 10–20 to amortize the lock-acquire / index-rebuild
	// overhead across more documents per round.
	IngestBatchSize int `yaml:"ingest_batch_size" json:"ingest_batch_size,omitempty"`

	// IngestMapParallel sets the errgroup limit for the Map phase
	// (per-document extraction + summary + chunk citation). 0 falls
	// back to 10. Bound by the LLM provider's concurrency limit and
	// the worker's outbound HTTP pool.
	IngestMapParallel int `yaml:"ingest_map_parallel" json:"ingest_map_parallel,omitempty"`

	// IngestReduceParallel sets the errgroup limit for the Reduce phase
	// (per-slug page write). 0 falls back to 10. Bound by the same
	// LLM concurrency / HTTP pool considerations as the Map phase,
	// plus DB connection pool size.
	IngestReduceParallel int `yaml:"ingest_reduce_parallel" json:"ingest_reduce_parallel,omitempty"`
}

// IngestBatchSizeOrDefault returns IngestBatchSize when set (> 0),
// otherwise the hard-coded fallback. Centralized so callers don't have
// to repeat the 0-check.
func (c *WikiConfig) IngestBatchSizeOrDefault(fallback int) int {
	if c == nil || c.IngestBatchSize <= 0 {
		return fallback
	}
	return c.IngestBatchSize
}

// IngestMapParallelOrDefault returns IngestMapParallel when set,
// otherwise the hard-coded fallback.
func (c *WikiConfig) IngestMapParallelOrDefault(fallback int) int {
	if c == nil || c.IngestMapParallel <= 0 {
		return fallback
	}
	return c.IngestMapParallel
}

// IngestReduceParallelOrDefault returns IngestReduceParallel when set,
// otherwise the hard-coded fallback.
func (c *WikiConfig) IngestReduceParallelOrDefault(fallback int) int {
	if c == nil || c.IngestReduceParallel <= 0 {
		return fallback
	}
	return c.IngestReduceParallel
}

// Value implements the driver.Valuer interface
func (c WikiConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface
func (c *WikiConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// WikiPageListRequest represents a request to list wiki pages with filtering
type WikiPageListRequest struct {
	KnowledgeBaseID string      `json:"knowledge_base_id"`
	PageType        string      `json:"page_type,omitempty"`      // filter by type
	Status          string      `json:"status,omitempty"`         // filter by status
	Query           string      `json:"query,omitempty"`          // full-text search
	FolderID        *string     `json:"folder_id,omitempty"`      // exact folder placement ("" = root)
	CategoryPath    StringArray `json:"category_path,omitempty"`  // exact directory path
	CategoryDepth   *int        `json:"category_depth,omitempty"` // exact directory depth, including 0 for root
	Page            int         `json:"page,omitempty"`           // pagination page (1-based)
	PageSize        int         `json:"page_size,omitempty"`      // pagination size
	SortBy          string      `json:"sort_by,omitempty"`        // "updated_at", "created_at", "title"
	SortOrder       string      `json:"sort_order,omitempty"`     // "asc" or "desc"
}

// WikiPageListResponse represents a paginated list of wiki pages
type WikiPageListResponse struct {
	Pages      []*WikiPage `json:"pages"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// WikiGraphMode enumerates the graph query modes exposed to the API.
const (
	// WikiGraphModeOverview returns the top-N most-connected pages as an
	// overview of the knowledge base. Intended for the first graph open.
	WikiGraphModeOverview = "overview"
	// WikiGraphModeEgo returns the neighborhood around a center page up to a
	// configurable depth. Intended for drill-down interactions.
	WikiGraphModeEgo = "ego"
)

// WikiGraphRequest is the service-layer input for graph queries. It is
// populated by the HTTP handler from query params and passed down to the
// service, which is responsible for enforcing mode-specific semantics.
//
// Limit policy: a non-positive `Limit` means "no cap" and is reserved for
// internal callers (e.g. wiki lint) that need the full graph. The HTTP
// handler always clamps `Limit` into a safe range before calling the
// service so external traffic can never request an uncapped graph.
type WikiGraphRequest struct {
	KnowledgeBaseID string
	Mode            string   // "overview" (default) | "ego"
	Center          string   // ego mode center slug (required when Mode == "ego")
	Depth           int      // ego mode BFS depth, >= 1
	Types           []string // optional page_type filter; empty = no filter
	Limit           int      // max nodes to return; <= 0 means uncapped
}

// WikiGraphData represents the link graph structure for visualization.
type WikiGraphData struct {
	Nodes []WikiGraphNode `json:"nodes"`
	Edges []WikiGraphEdge `json:"edges"`
	Meta  WikiGraphMeta   `json:"meta"`
}

// WikiGraphMeta describes how the returned subgraph relates to the full
// knowledge base graph. The frontend uses `Truncated` to decide whether to
// surface a "showing X of Y" hint and to enable ego-expansion UI.
type WikiGraphMeta struct {
	Mode      string `json:"mode"`
	Total     int    `json:"total"`            // total node count in the KB before filtering/limit
	Returned  int    `json:"returned"`         // number of nodes actually returned
	Truncated bool   `json:"truncated"`        // true when Returned < Total (after filters)
	Center    string `json:"center,omitempty"` // populated in ego mode
	Depth     int    `json:"depth,omitempty"`  // populated in ego mode
}

// WikiGraphNode represents a node in the wiki link graph
type WikiGraphNode struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	PageType string `json:"page_type"`
	// Number of inbound + outbound links
	LinkCount int `json:"link_count"`
}

// WikiGraphEdge represents a directed edge in the wiki link graph
type WikiGraphEdge struct {
	Source string `json:"source"` // source slug
	Target string `json:"target"` // target slug
}

// WikiStats provides aggregate statistics about the wiki
type WikiStats struct {
	TotalPages    int64            `json:"total_pages"`
	PagesByType   map[string]int64 `json:"pages_by_type"`
	TotalLinks    int64            `json:"total_links"`
	OrphanCount   int64            `json:"orphan_count"`   // pages with no inbound links
	RecentUpdates []*WikiPage      `json:"recent_updates"` // last N updated pages
	PendingTasks  int64            `json:"pending_tasks"`  // number of documents waiting to be ingested
	PendingIssues int64            `json:"pending_issues"` // number of pending wiki issues
	IsActive      bool             `json:"is_active"`      // whether wiki ingestion is currently running
}

// WikiPageIssue represents an issue flagged on a specific wiki page.
// These issues are typically identified by agents or linters and stored for review.
type WikiPageIssue struct {
	ID                    string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID              uint64         `json:"tenant_id" gorm:"index"`
	KnowledgeBaseID       string         `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	Slug                  string         `json:"slug" gorm:"type:varchar(255);index"`
	IssueType             string         `json:"issue_type" gorm:"type:varchar(50)"`
	Description           string         `json:"description" gorm:"type:text"`
	SuspectedKnowledgeIDs StringArray    `json:"suspected_knowledge_ids" gorm:"type:json"`
	Status                string         `json:"status" gorm:"type:varchar(20);default:'pending';index"`
	ReportedBy            string         `json:"reported_by" gorm:"type:varchar(100)"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName specifies the database table name
func (WikiPageIssue) TableName() string {
	return "wiki_page_issues"
}

// WikiIndexEntry is a single row in the structured wiki index response.
// Only the columns needed to render a clickable directory entry are
// carried — the backend projects SELECT slug, title, summary so a 40k-
// page KB does not pay for TEXT content transport on every index open.
type WikiIndexEntry struct {
	Slug         string      `json:"slug"`
	Title        string      `json:"title"`
	Summary      string      `json:"summary"`
	ParentSlug   string      `json:"parent_slug,omitempty"`
	CategoryPath StringArray `json:"category_path,omitempty"`
	WikiPath     string      `json:"wiki_path,omitempty"`
	Depth        int         `json:"depth,omitempty"`
	SortOrder    int         `json:"sort_order,omitempty"`
}

// WikiIndexGroup bundles the entries for one page_type into a page-sized
// slice. `Total` is the full count across the KB for the type; `Items`
// holds the current paginated window starting at `NextOffset - len(Items)`.
// An empty NextCursor means the window is already at the end of the type.
type WikiIndexGroup struct {
	Type       string           `json:"type"`
	Total      int64            `json:"total"`
	Items      []WikiIndexEntry `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// WikiIndexResponse is what GET /wiki/index returns. The heavy directory
// markdown that used to sit in wiki_pages.content is gone — only the LLM-
// generated intro survives there. Everything else is assembled on demand
// from the index repo's light-column projection, keeping index reads
// O(page_size) regardless of KB size.
type WikiIndexResponse struct {
	Intro   string           `json:"intro"`
	Version int              `json:"version"`
	Groups  []WikiIndexGroup `json:"groups"`
}

// WikiPageLite is a slim projection of WikiPage carrying only the fields
// the wiki ingest pipeline reaches for during Map / Reduce. It exists so
// per-batch fetcher queries don't have to load the full multi-MB content
// column for every page they want a title or out-link from.
//
// Use cases:
//
//   - SlugTitleFetcher: resolve slug -> title for log entries and
//     cross-link injection.
//   - cleanDeadLinks: read out_links + status without pulling content.
//   - dedup pre-filter: title + aliases + page_type for the trgm /
//     surface-similarity comparisons.
//
// Aliases is included because dedup and cross-link injection both treat
// the alias surface forms as first-class match targets; OutLinks is
// included so dead-link cleanup can determine which pages reference a
// given dead slug without a second query.
type WikiPageLite struct {
	Slug     string      `json:"slug"`
	Title    string      `json:"title"`
	PageType string      `json:"page_type"`
	Status   string      `json:"status"`
	Aliases  StringArray `json:"aliases,omitempty"`
	OutLinks StringArray `json:"out_links,omitempty"`
}

// WikiReviewAction enumerates the actions that can be taken on a wiki page
// during the review workflow.
const (
	// WikiReviewActionSubmit submits a draft page for review (draft -> reviewing)
	WikiReviewActionSubmit = "submit"
	// WikiReviewActionApprove approves a page under review (reviewing -> published)
	WikiReviewActionApprove = "approve"
	// WikiReviewActionReject rejects a page under review, sending it back to draft (reviewing -> draft)
	WikiReviewActionReject = "reject"
	// WikiReviewActionDeprecate marks a published page as deprecated (published -> deprecated)
	WikiReviewActionDeprecate = "deprecate"
	// WikiReviewActionArchive archives a page (deprecated/superseded -> archived)
	WikiReviewActionArchive = "archive"
	// WikiReviewActionSupersede marks a page as superseded by another page (published -> superseded)
	WikiReviewActionSupersede = "supersede"
)

// WikiReviewRequest represents a request to perform a review action on a wiki page.
type WikiReviewRequest struct {
	// Action is the review action to perform: submit, approve, reject, deprecate, archive
	Action string `json:"action" binding:"required"`
	// Reason is an optional human-readable explanation for the action
	Reason string `json:"reason,omitempty"`
	// ReviewerID is the user performing the action (populated from auth context)
	ReviewerID string `json:"-"`
}

// WikiReviewResult represents the result of a review action.
type WikiReviewResult struct {
	Page     *WikiPage `json:"page"`
	Action   string    `json:"action"`
	From     string    `json:"from_status"`
	To       string    `json:"to_status"`
	Reason   string    `json:"reason,omitempty"`
	Approved bool      `json:"approved"`
}

// WikiSupersession represents a "page A is superseded by page B" relationship.
// When page A is superseded by page B, A's status becomes "superseded" and
// users visiting A are redirected or informed about B.
type WikiSupersession struct {
	ID              string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64         `json:"tenant_id" gorm:"index"`
	KnowledgeBaseID string         `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	// OldPageID is the page being superseded (the older / outdated page)
	OldPageID string `json:"old_page_id" gorm:"type:varchar(36);index"`
	// OldPageSlug is denormalized from old page for display convenience
	OldPageSlug string `json:"old_page_slug" gorm:"type:varchar(255);index"`
	// NewPageID is the page that supersedes the old one (the newer / replacement page)
	NewPageID string `json:"new_page_id" gorm:"type:varchar(36);index"`
	// NewPageSlug is denormalized from new page for display convenience
	NewPageSlug string `json:"new_page_slug" gorm:"type:varchar(255);index"`
	// Reason explains why the old page was superseded
	Reason string `json:"reason,omitempty" gorm:"type:text"`
	// CreatedBy is the user who created this supersession relationship
	CreatedBy string    `json:"created_by" gorm:"type:varchar(100)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName specifies the database table name
func (WikiSupersession) TableName() string {
	return "wiki_supersessions"
}

// WikiSupersedeRequest represents a request to mark a page as superseded by another page.
type WikiSupersedeRequest struct {
	// OldPageSlug is the slug of the page being superseded
	OldPageSlug string `json:"old_page_slug" binding:"required"`
	// NewPageSlug is the slug of the page that supersedes the old one
	NewPageSlug string `json:"new_page_slug" binding:"required"`
	// Reason explains why the old page is being superseded
	Reason string `json:"reason,omitempty"`
}

// --- Milestone C: Confidence, Quality & Freshness Types ---

// WikiFeedbackRequest represents a request to submit user feedback for a page
type WikiFeedbackRequest struct {
	IsPositive bool `json:"is_positive"`
}

// WikiCriticalityRequest represents a request to set a page's criticality level
type WikiCriticalityRequest struct {
	Level string `json:"level" binding:"required,oneof=P0 P1 P2 P3"`
}

// WikiExpertValidationRequest represents a request to set expert validation status
type WikiExpertValidationRequest struct {
	Validated bool `json:"validated"`
}

// WikiQualityScores contains all scoring information for a wiki page
type WikiQualityScores struct {
	// Confidence components
	ConfidenceScore     float64 `json:"confidence_score"`
	SourceAuthority     float64 `json:"source_authority"`
	EvidenceSupport     float64 `json:"evidence_support"`
	RecencyScore        float64 `json:"recency_score"`
	ConsistencyScore    float64 `json:"consistency_score"`
	ExpertValidation    float64 `json:"expert_validation"`
	UsageFeedback       float64 `json:"usage_feedback"`
	ContradictionPenalty float64 `json:"contradiction_penalty"`
	StalenessPenalty    float64 `json:"staleness_penalty"`

	// Quality components
	QualityScore        float64 `json:"quality_score"`
	ContentCompleteness float64 `json:"content_completeness"`
	SourceReliability   float64 `json:"source_reliability"`
	TimelinessScore     float64 `json:"timeliness_score"`
	ReadabilityScore    float64 `json:"readability_score"`
	CitationSufficiency float64 `json:"citation_sufficiency"`

	// Final combined score
	FinalScore float64 `json:"final_score"`

	// Freshness state
	FreshnessState  string  `json:"freshness_state"`
	RetrievalBoost  float64 `json:"retrieval_boost"`
	CriticalityLevel string `json:"criticality_level"`

	// Usage metrics
	ViewCount        int64 `json:"view_count"`
	PositiveFeedback int64 `json:"positive_feedback"`
	NegativeFeedback int64 `json:"negative_feedback"`
	ExpertValidated  bool  `json:"expert_validated"`

	// Metadata
	ScoreLastCalculatedAt time.Time `json:"score_last_calculated_at"`
}

// WikiRefreshScoresResult contains the result of a batch score refresh operation
type WikiRefreshScoresResult struct {
	PagesRefreshed int  `json:"pages_refreshed"`
	Success        bool `json:"success"`
}

// GetQualityScores extracts quality scoring information from a WikiPage
func (p *WikiPage) GetQualityScores() *WikiQualityScores {
	if p == nil {
		return nil
	}
	return &WikiQualityScores{
		ConfidenceScore:      p.ConfidenceScore,
		SourceAuthority:      p.SourceAuthority,
		EvidenceSupport:      p.EvidenceSupport,
		RecencyScore:         p.RecencyScore,
		ConsistencyScore:     p.ConsistencyScore,
		ExpertValidation:     p.ExpertValidation,
		UsageFeedback:        p.UsageFeedback,
		ContradictionPenalty: p.ContradictionPenalty,
		StalenessPenalty:     p.StalenessPenalty,
		QualityScore:         p.QualityScore,
		ContentCompleteness:  p.ContentCompleteness,
		SourceReliability:    p.SourceReliability,
		TimelinessScore:      p.TimelinessScore,
		ReadabilityScore:     p.ReadabilityScore,
		CitationSufficiency:  p.CitationSufficiency,
		FinalScore:           p.FinalScore,
		FreshnessState:       p.FreshnessState,
		RetrievalBoost:       p.RetrievalBoost,
		CriticalityLevel:     p.CriticalityLevel,
		ViewCount:            p.ViewCount,
		PositiveFeedback:     p.PositiveFeedback,
		NegativeFeedback:     p.NegativeFeedback,
		ExpertValidated:      p.ExpertValidated,
		ScoreLastCalculatedAt: p.ScoreLastCalculatedAt,
	}
}
