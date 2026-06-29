package wikiquality

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/Tencent/XinWiki/internal/types"
)

func TestClampScore(t *testing.T) {
	assert.Equal(t, 0.0, types.ClampScore(-0.5))
	assert.Equal(t, 1.0, types.ClampScore(1.5))
	assert.Equal(t, 0.5, types.ClampScore(0.5))
	assert.Equal(t, 0.0, types.ClampScore(0.0))
	assert.Equal(t, 1.0, types.ClampScore(1.0))
}

func TestNormalizeCriticalityLevel(t *testing.T) {
	assert.Equal(t, types.CriticalityP0, types.NormalizeCriticalityLevel("p0"))
	assert.Equal(t, types.CriticalityP1, types.NormalizeCriticalityLevel("P1"))
	assert.Equal(t, types.CriticalityP2, types.NormalizeCriticalityLevel(" p2 "))
	assert.Equal(t, types.CriticalityP3, types.NormalizeCriticalityLevel(""))
	assert.Equal(t, types.CriticalityP3, types.NormalizeCriticalityLevel("invalid"))
}

func TestNormalizeFreshnessState(t *testing.T) {
	assert.Equal(t, types.FreshnessActive, types.NormalizeFreshnessState("ACTIVE"))
	assert.Equal(t, types.FreshnessWarm, types.NormalizeFreshnessState("warm"))
	assert.Equal(t, types.FreshnessCold, types.NormalizeFreshnessState(" Cold "))
	assert.Equal(t, types.FreshnessArchived, types.NormalizeFreshnessState("archived"))
	assert.Equal(t, types.FreshnessActive, types.NormalizeFreshnessState(""))
	assert.Equal(t, types.FreshnessActive, types.NormalizeFreshnessState("invalid"))
}

func TestCalculateConfidenceScore(t *testing.T) {
	t.Run("nil page returns default 0.5", func(t *testing.T) {
		score := CalculateConfidenceScore(nil)
		assert.InDelta(t, 0.5, score, 0.001)
	})

	t.Run("all zeros returns minimum base", func(t *testing.T) {
		page := &types.WikiPage{}
		page.SourceAuthority = 0
		page.EvidenceSupport = 0
		page.RecencyScore = 0
		page.ConsistencyScore = 0
		page.ExpertValidation = 0
		page.UsageFeedback = 0
		score := CalculateConfidenceScore(page)
		assert.InDelta(t, 0.0, score, 0.001)
	})

	t.Run("all ones returns 1.0", func(t *testing.T) {
		page := &types.WikiPage{}
		page.SourceAuthority = 1.0
		page.EvidenceSupport = 1.0
		page.RecencyScore = 1.0
		page.ConsistencyScore = 1.0
		page.ExpertValidation = 1.0
		page.UsageFeedback = 1.0
		score := CalculateConfidenceScore(page)
		assert.InDelta(t, 1.0, score, 0.001)
	})

	t.Run("weights sum to 1.0", func(t *testing.T) {
		totalWeight := types.WeightSourceAuthority +
			types.WeightEvidenceSupport +
			types.WeightRecency +
			types.WeightConsistency +
			types.WeightExpertValidation +
			types.WeightUsageFeedback
		assert.InDelta(t, 1.0, totalWeight, 0.001)
	})

	t.Run("average page returns ~0.5", func(t *testing.T) {
		page := &types.WikiPage{}
		page.SourceAuthority = 0.5
		page.EvidenceSupport = 0.5
		page.RecencyScore = 1.0
		page.ConsistencyScore = 1.0
		page.ExpertValidation = 0.0
		page.UsageFeedback = 0.5
		score := CalculateConfidenceScore(page)
		assert.True(t, score > 0.4 && score < 0.8)
	})
}

func TestCalculateContradictionPenalty(t *testing.T) {
	t.Run("no contradictions returns 1.0", func(t *testing.T) {
		penalty := CalculateContradictionPenalty(0)
		assert.InDelta(t, 1.0, penalty, 0.001)
	})

	t.Run("one contradiction reduces by 0.1", func(t *testing.T) {
		penalty := CalculateContradictionPenalty(1)
		assert.InDelta(t, 0.9, penalty, 0.001)
	})

	t.Run("five contradictions floors at 0.5", func(t *testing.T) {
		penalty := CalculateContradictionPenalty(5)
		assert.InDelta(t, 0.5, penalty, 0.001)
	})

	t.Run("ten contradictions stays floored at 0.5", func(t *testing.T) {
		penalty := CalculateContradictionPenalty(10)
		assert.InDelta(t, 0.5, penalty, 0.001)
	})
}

func TestCalculateStalenessPenalty(t *testing.T) {
	now := time.Now()

	t.Run("just updated returns 1.0", func(t *testing.T) {
		penalty := CalculateStalenessPenalty(now, now)
		assert.InDelta(t, 1.0, penalty, 0.001)
	})

	t.Run("100 days old returns penalty", func(t *testing.T) {
		oldTime := now.AddDate(0, 0, -100)
		penalty := CalculateStalenessPenalty(oldTime, now)
		expected := 1.0 - 100*types.StalenessPenaltyFactor
		assert.InDelta(t, expected, penalty, 0.001)
	})

	t.Run("2 years old floors at 0.3", func(t *testing.T) {
		oldTime := now.AddDate(-2, 0, 0)
		penalty := CalculateStalenessPenalty(oldTime, now)
		assert.InDelta(t, 0.3, penalty, 0.001)
	})

	t.Run("zero time returns 1.0", func(t *testing.T) {
		penalty := CalculateStalenessPenalty(time.Time{}, now)
		assert.InDelta(t, 1.0, penalty, 0.001)
	})
}

func TestCalculateRecencyScore(t *testing.T) {
	now := time.Now()

	t.Run("just updated returns 1.0", func(t *testing.T) {
		score := CalculateRecencyScore(now, now)
		assert.InDelta(t, 1.0, score, 0.001)
	})

	t.Run("1 week old returns 1.0", func(t *testing.T) {
		oldTime := now.AddDate(0, 0, -7)
		score := CalculateRecencyScore(oldTime, now)
		assert.InDelta(t, 1.0, score, 0.001)
	})

	t.Run("1 month old returns ~0.77+", func(t *testing.T) {
		oldTime := now.AddDate(0, 0, -30)
		score := CalculateRecencyScore(oldTime, now)
		assert.True(t, score >= 0.7)
	})

	t.Run("over 1 year old returns ~0.2", func(t *testing.T) {
		oldTime := now.AddDate(-2, 0, 0)
		score := CalculateRecencyScore(oldTime, now)
		assert.InDelta(t, 0.2, score, 0.01)
	})

	t.Run("zero time returns 0.5", func(t *testing.T) {
		score := CalculateRecencyScore(time.Time{}, now)
		assert.InDelta(t, 0.5, score, 0.001)
	})
}

func TestCalculateFinalScore(t *testing.T) {
	now := time.Now()

	t.Run("nil page returns 0.5", func(t *testing.T) {
		score := CalculateFinalScore(nil, now)
		assert.InDelta(t, 0.5, score, 0.001)
	})

	t.Run("perfect page returns 1.0", func(t *testing.T) {
		page := &types.WikiPage{
			SourceAuthority:     1.0,
			EvidenceSupport:     1.0,
			RecencyScore:        1.0,
			ConsistencyScore:    1.0,
			ExpertValidation:    1.0,
			UsageFeedback:       1.0,
			ContradictionCount:  0,
			UpdatedAt:           now,
		}
		score := CalculateFinalScore(page, now)
		assert.InDelta(t, 1.0, score, 0.001)
	})

	t.Run("contradictions reduce final score", func(t *testing.T) {
		page := &types.WikiPage{
			SourceAuthority:     1.0,
			EvidenceSupport:     1.0,
			RecencyScore:        1.0,
			ConsistencyScore:    1.0,
			ExpertValidation:    1.0,
			UsageFeedback:       1.0,
			ContradictionCount:  3,
			UpdatedAt:           now,
		}
		score := CalculateFinalScore(page, now)
		expected := 1.0 * 0.7 * 1.0
		assert.InDelta(t, expected, score, 0.001)
	})
}

func TestCalculateQualityScore(t *testing.T) {
	t.Run("nil page returns 0.5", func(t *testing.T) {
		score := CalculateQualityScore(nil)
		assert.InDelta(t, 0.5, score, 0.001)
	})

	t.Run("all ones returns 1.0", func(t *testing.T) {
		page := &types.WikiPage{
			ContentCompleteness: 1.0,
			SourceReliability:   1.0,
			TimelinessScore:     1.0,
			ReadabilityScore:    1.0,
			CitationSufficiency: 1.0,
			PositiveFeedback:    10,
			NegativeFeedback:    0,
		}
		score := CalculateQualityScore(page)
		assert.InDelta(t, 1.0, score, 0.001)
	})

	t.Run("weights sum to 1.0", func(t *testing.T) {
		totalWeight := types.WeightContentCompleteness +
			types.WeightSourceReliability +
			types.WeightTimeliness +
			types.WeightReadability +
			types.WeightCitationSufficiency +
			types.WeightQualityFeedback
		assert.InDelta(t, 1.0, totalWeight, 0.001)
	})

	t.Run("no feedback uses 0.5 default", func(t *testing.T) {
		page := &types.WikiPage{
			ContentCompleteness: 1.0,
			SourceReliability:   1.0,
			TimelinessScore:     1.0,
			ReadabilityScore:    1.0,
			CitationSufficiency: 1.0,
			PositiveFeedback:    0,
			NegativeFeedback:    0,
		}
		score := CalculateQualityScore(page)
		// 5 dimensions at 1.0 (0.85 total), plus 0.5 for feedback (0.075)
		expected := 0.85 + 0.075
		assert.InDelta(t, expected, score, 0.001)
	})
}

func TestCalculateFreshnessState(t *testing.T) {
	now := time.Now()

	t.Run("nil page returns active", func(t *testing.T) {
		state := CalculateFreshnessState(nil, now)
		assert.Equal(t, types.FreshnessActive, state)
	})

	t.Run("archived status returns archived", func(t *testing.T) {
		page := &types.WikiPage{
			Status: types.WikiPageStatusArchived,
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessArchived, state)
	})

	t.Run("deprecated status returns cold", func(t *testing.T) {
		page := &types.WikiPage{
			Status: types.WikiPageStatusDeprecated,
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessCold, state)
	})

	t.Run("recently accessed returns active", func(t *testing.T) {
		page := &types.WikiPage{
			Status:       types.WikiPageStatusPublished,
			LastAccessedAt: now.AddDate(0, 0, -10),
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessActive, state)
	})

	t.Run("2 months old returns warm", func(t *testing.T) {
		page := &types.WikiPage{
			Status:         types.WikiPageStatusPublished,
			LastAccessedAt: now.AddDate(0, 0, -60),
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessWarm, state)
	})

	t.Run("4 months old returns cold", func(t *testing.T) {
		page := &types.WikiPage{
			Status:         types.WikiPageStatusPublished,
			LastAccessedAt: now.AddDate(0, 0, -120),
			CriticalityLevel: types.CriticalityP3,
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessCold, state)
	})

	t.Run("1 year old P3 returns archived", func(t *testing.T) {
		page := &types.WikiPage{
			Status:         types.WikiPageStatusPublished,
			LastAccessedAt: now.AddDate(-1, 0, 0),
			CriticalityLevel: types.CriticalityP3,
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessArchived, state)
	})

	t.Run("1 year old P0 stays cold (never auto-archive)", func(t *testing.T) {
		page := &types.WikiPage{
			Status:         types.WikiPageStatusPublished,
			LastAccessedAt: now.AddDate(-1, 0, 0),
			CriticalityLevel: types.CriticalityP0,
		}
		state := CalculateFreshnessState(page, now)
		assert.Equal(t, types.FreshnessCold, state)
	})
}

func TestCalculateRetrievalBoost(t *testing.T) {
	t.Run("active page returns 1.0", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessActive, types.WikiPageStatusPublished)
		assert.InDelta(t, 1.0, boost, 0.001)
	})

	t.Run("warm page returns 0.8", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessWarm, types.WikiPageStatusPublished)
		assert.InDelta(t, 0.8, boost, 0.001)
	})

	t.Run("cold page returns 0.5", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessCold, types.WikiPageStatusPublished)
		assert.InDelta(t, 0.5, boost, 0.001)
	})

	t.Run("archived page returns 0.0", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessArchived, types.WikiPageStatusPublished)
		assert.InDelta(t, 0.0, boost, 0.001)
	})

	t.Run("deprecated page returns 0.3 regardless of freshness", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessActive, types.WikiPageStatusDeprecated)
		assert.InDelta(t, 0.3, boost, 0.001)
	})

	t.Run("superseded page returns 0.1", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessActive, types.WikiPageStatusSuperseded)
		assert.InDelta(t, 0.1, boost, 0.001)
	})

	t.Run("draft returns 0.7", func(t *testing.T) {
		boost := CalculateRetrievalBoost(types.FreshnessActive, types.WikiPageStatusDraft)
		assert.InDelta(t, 0.7, boost, 0.001)
	})
}

func TestShouldAutoArchive(t *testing.T) {
	now := time.Now()

	t.Run("nil page returns false", func(t *testing.T) {
		result := ShouldAutoArchive(nil, now)
		assert.False(t, result)
	})

	t.Run("draft page never auto-archives", func(t *testing.T) {
		page := &types.WikiPage{
			Status: types.WikiPageStatusDraft,
		}
		result := ShouldAutoArchive(page, now)
		assert.False(t, result)
	})

	t.Run("P0 page never auto-archives", func(t *testing.T) {
		page := &types.WikiPage{
			Status:           types.WikiPageStatusPublished,
			CriticalityLevel: types.CriticalityP0,
			LastAccessedAt:   now.AddDate(-5, 0, 0),
		}
		result := ShouldAutoArchive(page, now)
		assert.False(t, result)
	})

	t.Run("already archived returns false", func(t *testing.T) {
		page := &types.WikiPage{
			Status: types.WikiPageStatusArchived,
		}
		result := ShouldAutoArchive(page, now)
		assert.False(t, result)
	})

	t.Run("old P3 page returns true", func(t *testing.T) {
		page := &types.WikiPage{
			Status:           types.WikiPageStatusPublished,
			CriticalityLevel: types.CriticalityP3,
			LastAccessedAt:   now.AddDate(-1, 0, 0),
		}
		result := ShouldAutoArchive(page, now)
		assert.True(t, result)
	})
}

func TestUpdateAllScores(t *testing.T) {
	now := time.Now()

	t.Run("nil page does not panic", func(t *testing.T) {
		UpdateAllScores(nil, now)
	})

	t.Run("updates all score fields", func(t *testing.T) {
		page := &types.WikiPage{
			ID:                "test-id",
			Status:            types.WikiPageStatusPublished,
			CriticalityLevel:  types.CriticalityP2,
			SourceAuthority:   0.7,
			EvidenceSupport:   0.6,
			ConsistencyScore:  0.9,
			ExpertValidation:  0.0,
			UsageFeedback:     0.6,
			ContradictionCount: 1,
			ContentCompleteness: 0.7,
			SourceReliability: 0.7,
			ReadabilityScore:  0.6,
			CitationSufficiency: 0.5,
			CreatedAt:         now.AddDate(0, 0, -60),
			UpdatedAt:         now.AddDate(0, 0, -10),
			LastAccessedAt:    now.AddDate(0, 0, -5),
		}

		UpdateAllScores(page, now)

		assert.True(t, page.ConfidenceScore > 0)
		assert.True(t, page.ConfidenceScore <= 1.0)
		assert.True(t, page.QualityScore > 0)
		assert.True(t, page.QualityScore <= 1.0)
		assert.True(t, page.FinalScore > 0)
		assert.True(t, page.FinalScore <= 1.0)
		assert.NotEqual(t, time.Time{}, page.ScoreLastCalculatedAt)
		assert.True(t, page.RecencyScore > 0)
		assert.True(t, page.ContradictionPenalty < 1.0)
		assert.True(t, page.StalenessPenalty <= 1.0)
		assert.NotEmpty(t, page.FreshnessState)
		assert.True(t, page.RetrievalBoost > 0)
		assert.True(t, page.RetrievalBoost <= 1.0)
	})
}

func TestRecordPageAccess(t *testing.T) {
	now := time.Now()

	t.Run("nil page does not panic", func(t *testing.T) {
		RecordPageAccess(nil, now)
	})

	t.Run("increments view count and updates last accessed", func(t *testing.T) {
		page := &types.WikiPage{
			ViewCount: 5,
		}
		RecordPageAccess(page, now)
		assert.Equal(t, int64(6), page.ViewCount)
		assert.Equal(t, now, page.LastAccessedAt)
	})
}

func TestRecordFeedback(t *testing.T) {
	t.Run("nil page does not panic", func(t *testing.T) {
		RecordFeedback(nil, true)
	})

	t.Run("positive feedback increments count", func(t *testing.T) {
		page := &types.WikiPage{
			PositiveFeedback: 3,
			NegativeFeedback: 1,
		}
		RecordFeedback(page, true)
		assert.Equal(t, int64(4), page.PositiveFeedback)
		assert.Equal(t, int64(1), page.NegativeFeedback)
	})

	t.Run("negative feedback increments count", func(t *testing.T) {
		page := &types.WikiPage{
			PositiveFeedback: 3,
			NegativeFeedback: 1,
		}
		RecordFeedback(page, false)
		assert.Equal(t, int64(3), page.PositiveFeedback)
		assert.Equal(t, int64(2), page.NegativeFeedback)
	})
}

func TestCalculateSourceAuthority(t *testing.T) {
	t.Run("no sources returns low score", func(t *testing.T) {
		score := CalculateSourceAuthority(0, false, 0)
		assert.InDelta(t, 0.3, score, 0.001)
	})

	t.Run("many authoritative sources returns high score", func(t *testing.T) {
		score := CalculateSourceAuthority(5, true, 0.8)
		assert.True(t, score >= 0.8)
	})
}

func TestCalculateContentCompleteness(t *testing.T) {
	t.Run("empty content returns low score", func(t *testing.T) {
		score := CalculateContentCompleteness("", false, false, 0)
		assert.InDelta(t, 0.2, score, 0.001)
	})

	t.Run("comprehensive content returns high score", func(t *testing.T) {
		longContent := `# Introduction

This is the introduction section. It provides context for the topic and outlines what will be covered in this document. This is a well-structured paragraph with good sentence length.

## Background

This section covers background information. It provides necessary context for understanding the main concepts. There are multiple paragraphs here that build upon each other to provide comprehensive coverage.

## Key Concepts

- First key concept with detailed explanation
- Second key concept with examples and supporting evidence
- Third key concept that ties everything together
- Fourth concept that discusses edge cases and limitations

## Conclusion

This final section summarizes the key points and provides actionable recommendations. It wraps up the discussion nicely.

Additional supporting information goes here to provide more depth.`
		score := CalculateContentCompleteness(longContent, true, true, strings.Count(longContent, "## ")+strings.Count(longContent, "# ")-1)
		assert.True(t, score >= 0.7)
	})
}

func TestCalculateReadabilityScore(t *testing.T) {
	t.Run("empty content returns low score", func(t *testing.T) {
		score := CalculateReadabilityScore("")
		assert.InDelta(t, 0.2, score, 0.001)
	})

	t.Run("well-formatted content returns good score", func(t *testing.T) {
		content := `# Introduction

This is a well-structured paragraph with good sentence length. It contains multiple sentences that are easy to read.

## Key Points

- First point is clear
- Second point is also clear
- Third point provides good detail

This makes the content more readable and engaging.`
		score := CalculateReadabilityScore(content)
		assert.True(t, score >= 0.7)
	})
}

func TestCalculateCitationSufficiency(t *testing.T) {
	t.Run("no words returns base score", func(t *testing.T) {
		score := CalculateCitationSufficiency(0, 0)
		assert.InDelta(t, 0.3, score, 0.001)
	})

	t.Run("well-cited content returns high score", func(t *testing.T) {
		// 600 words, 5 citations = 1.67 ratio
		score := CalculateCitationSufficiency(5, 600)
		assert.True(t, score >= 0.8)
	})
}
