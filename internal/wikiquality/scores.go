package wikiquality

import (
	"math"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// CalculateConfidenceScore computes the weighted confidence score
// Formula: 0.30*SourceAuthority + 0.20*EvidenceSupport + 0.20*Recency + 0.15*Consistency + 0.10*ExpertValidation + 0.05*UsageFeedback
func CalculateConfidenceScore(page *types.WikiPage) float64 {
	if page == nil {
		return types.ClampScore(0.5)
	}

	score := types.WeightSourceAuthority*page.SourceAuthority +
		types.WeightEvidenceSupport*page.EvidenceSupport +
		types.WeightRecency*page.RecencyScore +
		types.WeightConsistency*page.ConsistencyScore +
		types.WeightExpertValidation*page.ExpertValidation +
		types.WeightUsageFeedback*page.UsageFeedback

	return types.ClampScore(score)
}

// CalculateContradictionPenalty computes penalty based on contradiction count
// Penalty = max(0.5, 1.0 - count*0.1)
func CalculateContradictionPenalty(contradictionCount int) float64 {
	penalty := 1.0 - float64(contradictionCount)*types.ContradictionPenaltyPerItem
	if penalty < types.MaxContradictionPenalty {
		penalty = types.MaxContradictionPenalty
	}
	return types.ClampScore(penalty)
}

// CalculateStalenessPenalty computes penalty based on days since last update
// Formula: 1.0 - days*0.003, floor at 0.3
func CalculateStalenessPenalty(lastUpdated time.Time, now time.Time) float64 {
	if lastUpdated.IsZero() {
		return types.ClampScore(1.0)
	}

	days := now.Sub(lastUpdated).Hours() / 24.0
	penalty := 1.0 - days*types.StalenessPenaltyFactor
	if penalty < 0.3 {
		penalty = 0.3
	}
	return types.ClampScore(penalty)
}

// CalculateRecencyScore computes a recency score from 0.0 to 1.0
func CalculateRecencyScore(lastUpdated time.Time, now time.Time) float64 {
	if lastUpdated.IsZero() {
		return types.ClampScore(0.5)
	}

	days := now.Sub(lastUpdated).Hours() / 24.0
	if days <= 7 {
		return 1.0
	}
	if days <= 30 {
		return types.ClampScore(1.0 - (days-7)*0.01)
	}
	if days <= 90 {
		return types.ClampScore(0.77 - (days-30)*0.005)
	}
	if days <= 365 {
		return types.ClampScore(0.47 - (days-90)*0.001)
	}
	return types.ClampScore(0.2)
}

// CalculateFinalScore computes the final score with penalties applied
// FinalScore = ConfidenceScore * ContradictionPenalty * StalenessPenalty
func CalculateFinalScore(page *types.WikiPage, now time.Time) float64 {
	if page == nil {
		return types.ClampScore(0.5)
	}

	confidence := CalculateConfidenceScore(page)
	contradictionPenalty := CalculateContradictionPenalty(page.ContradictionCount)
	stalenessPenalty := CalculateStalenessPenalty(page.UpdatedAt, now)

	final := confidence * contradictionPenalty * stalenessPenalty
	return types.ClampScore(final)
}

// CalculateQualityScore computes the overall quality score from 6 dimensions
// Formula: 0.20*ContentCompleteness + 0.20*SourceReliability + 0.15*Timeliness + 0.15*Readability + 0.15*CitationSufficiency + 0.15*UsageFeedback
func CalculateQualityScore(page *types.WikiPage) float64 {
	if page == nil {
		return types.ClampScore(0.5)
	}

	usageFeedback := calculateUsageFeedbackScore(page.PositiveFeedback, page.NegativeFeedback)

	score := types.WeightContentCompleteness*page.ContentCompleteness +
		types.WeightSourceReliability*page.SourceReliability +
		types.WeightTimeliness*page.TimelinessScore +
		types.WeightReadability*page.ReadabilityScore +
		types.WeightCitationSufficiency*page.CitationSufficiency +
		types.WeightQualityFeedback*usageFeedback

	return types.ClampScore(score)
}

func calculateUsageFeedbackScore(positive, negative int64) float64 {
	total := positive + negative
	if total == 0 {
		return 0.5
	}
	ratio := float64(positive) / float64(total)
	// Confidence in feedback increases with sample size
	weight := math.Min(1.0, float64(total)/10.0)
	return types.ClampScore(0.5 + (ratio-0.5)*weight)
}

// CalculateSourceAuthority computes source authority based on number and quality of sources
func CalculateSourceAuthority(sourceCount int, hasAuthoritativeSource bool, averageSourceTrust float64) float64 {
	base := 0.3
	if sourceCount >= 1 {
		base += 0.2
	}
	if sourceCount >= 3 {
		base += 0.15
	}
	if sourceCount >= 5 {
		base += 0.1
	}
	if hasAuthoritativeSource {
		base += 0.15
	}
	base += averageSourceTrust * 0.1
	return types.ClampScore(base)
}

// CalculateEvidenceSupport computes evidence support based on citation count and chunk references
func CalculateEvidenceSupport(chunkRefCount int, citationDensity float64) float64 {
	base := 0.3
	if chunkRefCount >= 1 {
		base += 0.2
	}
	if chunkRefCount >= 3 {
		base += 0.15
	}
	if chunkRefCount >= 5 {
		base += 0.1
	}
	if citationDensity >= 0.5 {
		base += 0.1
	}
	if citationDensity >= 1.0 {
		base += 0.15
	}
	return types.ClampScore(base)
}

// CalculateContentCompleteness scores content based on length, structure, and sections
func CalculateContentCompleteness(content string, hasTitle bool, hasSummary bool, sectionCount int) float64 {
	base := 0.2
	words := len(strings.Fields(content))

	if words >= 50 {
		base += 0.2
	}
	if words >= 100 {
		base += 0.15
	}
	if words >= 300 {
		base += 0.15
	}
	if words >= 500 {
		base += 0.1
	}
	if hasTitle {
		base += 0.1
	}
	if hasSummary {
		base += 0.1
	}
	if sectionCount >= 2 {
		base += 0.1
	}
	if sectionCount >= 4 {
		base += 0.1
	}
	return types.ClampScore(base)
}

// CalculateReadabilityScore computes a basic readability score based on sentence length and structure
func CalculateReadabilityScore(content string) float64 {
	if len(strings.TrimSpace(content)) == 0 {
		return 0.2
	}

	sentences := strings.Count(content, ".") + strings.Count(content, "!") + strings.Count(content, "?")
	words := len(strings.Fields(content))
	paragraphs := strings.Count(content, "\n\n") + 1

	if sentences == 0 {
		sentences = 1
	}
	avgWordsPerSentence := float64(words) / float64(sentences)

	score := 0.5
	// Ideal: 15-20 words per sentence
	if avgWordsPerSentence >= 10 && avgWordsPerSentence <= 25 {
		score += 0.2
	} else if avgWordsPerSentence >= 8 && avgWordsPerSentence <= 30 {
		score += 0.1
	}
	// Paragraph structure
	if paragraphs >= 2 && words/paragraphs >= 50 {
		score += 0.15
	}
	// Has formatting
	if strings.Contains(content, "**") || strings.Contains(content, "#") || strings.Contains(content, "-") {
		score += 0.15
	}
	return types.ClampScore(score)
}

// CalculateCitationSufficiency scores how well-cited the content is
func CalculateCitationSufficiency(chunkRefCount int, wordCount int) float64 {
	words := float64(wordCount)
	citations := float64(chunkRefCount)

	if words == 0 {
		return 0.3
	}

	ratio := citations / (words / 200.0) // Expect ~1 citation per 200 words
	score := 0.3
	if ratio >= 0.5 {
		score += 0.2
	}
	if ratio >= 1.0 {
		score += 0.2
	}
	if ratio >= 1.5 {
		score += 0.2
	}
	if citations >= 1 {
		score += 0.1
	}
	return types.ClampScore(score)
}

// CalculateFreshnessState determines the freshness state based on last access and criticality
func CalculateFreshnessState(page *types.WikiPage, now time.Time) string {
	if page == nil {
		return types.FreshnessActive
	}

	// Respect page status first
	switch page.Status {
	case types.WikiPageStatusArchived:
		return types.FreshnessArchived
	case types.WikiPageStatusDeprecated:
		return types.FreshnessCold
	case types.WikiPageStatusDraft:
		return types.FreshnessActive
	}

	lastAccess := page.LastAccessedAt
	if lastAccess.IsZero() {
		lastAccess = page.UpdatedAt
	}
	if lastAccess.IsZero() {
		lastAccess = page.CreatedAt
	}

	daysSinceAccess := now.Sub(lastAccess).Hours() / 24.0

	// P0/P1 pages never go to archived automatically
	criticality := types.NormalizeCriticalityLevel(page.CriticalityLevel)
	isHighCriticality := criticality == types.CriticalityP0 || criticality == types.CriticalityP1

	switch {
	case daysSinceAccess <= types.FreshnessActiveThresholdDays:
		return types.FreshnessActive
	case daysSinceAccess <= types.FreshnessWarmThresholdDays:
		return types.FreshnessWarm
	case daysSinceAccess <= types.FreshnessColdThresholdDays:
		return types.FreshnessCold
	default:
		if isHighCriticality {
			return types.FreshnessCold
		}
		return types.FreshnessArchived
	}
}

// CalculateRetrievalBoost returns the retrieval multiplier for a given freshness state and status
func CalculateRetrievalBoost(freshnessState string, pageStatus string) float64 {
	// Status-based boosts take precedence for deprecated/superseded
	switch pageStatus {
	case types.WikiPageStatusDeprecated:
		return types.RetrievalBoostDeprecated
	case types.WikiPageStatusSuperseded:
		return types.RetrievalBoostSuperseded
	case types.WikiPageStatusArchived:
		return types.RetrievalBoostArchived
	case types.WikiPageStatusDraft:
		return 0.7
	}

	// Freshness-based boost
	switch types.NormalizeFreshnessState(freshnessState) {
	case types.FreshnessActive:
		return types.RetrievalBoostActive
	case types.FreshnessWarm:
		return types.RetrievalBoostWarm
	case types.FreshnessCold:
		return types.RetrievalBoostCold
	case types.FreshnessArchived:
		return types.RetrievalBoostArchived
	default:
		return types.RetrievalBoostActive
	}
}

// ShouldAutoArchive determines if a page should be automatically archived
// P0/P1 pages are never auto-archived
func ShouldAutoArchive(page *types.WikiPage, now time.Time) bool {
	if page == nil {
		return false
	}

	// Don't archive draft or published pages that are still being accessed
	if page.Status == types.WikiPageStatusDraft {
		return false
	}

	criticality := types.NormalizeCriticalityLevel(page.CriticalityLevel)
	if criticality == types.CriticalityP0 || criticality == types.CriticalityP1 {
		return false
	}

	// Already archived
	if page.Status == types.WikiPageStatusArchived {
		return false
	}

	freshness := CalculateFreshnessState(page, now)
	return freshness == types.FreshnessArchived
}

// UpdateAllScores recalculates all scores for a wiki page and updates the fields
func UpdateAllScores(page *types.WikiPage, now time.Time) {
	if page == nil {
		return
	}

	// Normalize defaults
	page.CriticalityLevel = types.NormalizeCriticalityLevel(page.CriticalityLevel)
	page.FreshnessState = types.NormalizeFreshnessState(page.FreshnessState)

	// Calculate recency
	page.RecencyScore = CalculateRecencyScore(page.UpdatedAt, now)
	page.TimelinessScore = page.RecencyScore // Timeliness uses same recency base

	// Calculate penalties
	page.ContradictionPenalty = CalculateContradictionPenalty(page.ContradictionCount)
	page.StalenessPenalty = CalculateStalenessPenalty(page.UpdatedAt, now)

	// Calculate confidence
	page.ConfidenceScore = CalculateConfidenceScore(page)

	// Calculate quality
	page.QualityScore = CalculateQualityScore(page)

	// Calculate final score
	page.FinalScore = CalculateFinalScore(page, now)

	// Calculate freshness and retrieval boost
	page.FreshnessState = CalculateFreshnessState(page, now)
	page.RetrievalBoost = CalculateRetrievalBoost(page.FreshnessState, page.Status)

	// Update timestamp
	page.ScoreLastCalculatedAt = now
}

// RecordPageAccess records a page view/access and updates last accessed time
func RecordPageAccess(page *types.WikiPage, now time.Time) {
	if page == nil {
		return
	}
	page.ViewCount++
	page.LastAccessedAt = now
}

// RecordFeedback records user feedback (positive or negative)
func RecordFeedback(page *types.WikiPage, isPositive bool) {
	if page == nil {
		return
	}
	if isPositive {
		page.PositiveFeedback++
	} else {
		page.NegativeFeedback++
	}
}
