package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

type conflictDetectionService struct {
	conflictRepo interfaces.ConflictRepository
	chunkRepo    interfaces.ChunkRepository
	knowledgeRepo interfaces.KnowledgeRepository
}

func NewConflictDetectionService(
	conflictRepo interfaces.ConflictRepository,
	chunkRepo interfaces.ChunkRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
) interfaces.ConflictDetectionService {
	return &conflictDetectionService{
		conflictRepo: conflictRepo,
		chunkRepo:    chunkRepo,
		knowledgeRepo: knowledgeRepo,
	}
}

func (s *conflictDetectionService) DetectConflicts(
	ctx context.Context,
	req *types.ConflictDetectionRequest,
) (*types.ConflictDetectionResult, error) {
	startTime := time.Now()

	if req.MinConfidence <= 0 {
		req.MinConfidence = 0.7
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 100
	}

	logger.Infof(ctx, "[ConflictDetection] Starting detection for tenant=%d kb=%s docs=%v",
		req.TenantID, req.KBID, req.DocIDs)

	result := &types.ConflictDetectionResult{
		RunAt: startTime,
	}

	var chunks []*types.Chunk
	var err error

	if len(req.DocIDs) > 0 {
		for _, docID := range req.DocIDs {
			docChunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, req.TenantID, docID)
			if err != nil {
				logger.Warnf(ctx, "[ConflictDetection] Failed to get chunks for doc %s: %v", docID, err)
				continue
			}
			chunks = append(chunks, docChunks...)
		}
	} else {
		chunks, err = s.listAllChunksForKB(ctx, req.TenantID, req.KBID)
		if err != nil {
			return nil, fmt.Errorf("failed to list chunks: %w", err)
		}
	}

	result.TotalScanned = len(chunks)

	detectTypes := req.ConflictTypes
	if len(detectTypes) == 0 {
		detectTypes = []types.ConflictType{
			types.ConflictTypeAttributeValue,
			types.ConflictTypeParameterDef,
			types.ConflictTypeTemporal,
			types.ConflictTypeNumerical,
		}
	}

	var detectedConflicts []*types.Conflict

	for _, ctype := range detectTypes {
		var conflicts []*types.Conflict
		var err error

		switch ctype {
		case types.ConflictTypeAttributeValue:
			conflicts, err = s.detectAttributeValueConflicts(ctx, req.TenantID, req.KBID, chunks)
		case types.ConflictTypeParameterDef:
			conflicts, err = s.detectParameterDefConflicts(ctx, req.TenantID, req.KBID, chunks)
		case types.ConflictTypeTemporal:
			conflicts, err = s.detectTemporalConflictsFromChunks(ctx, req.TenantID, req.KBID, chunks)
		case types.ConflictTypeNumerical:
			conflicts, err = s.detectNumericalConflicts(ctx, req.TenantID, req.KBID, chunks)
		default:
			logger.Warnf(ctx, "[ConflictDetection] Unsupported conflict type: %s", ctype)
			continue
		}

		if err != nil {
			logger.Warnf(ctx, "[ConflictDetection] Failed to detect %s conflicts: %v", ctype, err)
			continue
		}
		detectedConflicts = append(detectedConflicts, conflicts...)
	}

	for _, conflict := range detectedConflicts {
		existing, err := s.conflictRepo.FindExisting(
			ctx,
			req.TenantID,
			req.KBID,
			conflict.Type,
			conflict.EntityType,
			conflict.Attribute,
		)
		if err == nil && existing != nil {
			result.ExistingConflicts++
			continue
		}

		if err := s.conflictRepo.Create(ctx, conflict); err != nil {
			logger.Warnf(ctx, "[ConflictDetection] Failed to save conflict: %v", err)
			continue
		}
		result.NewConflicts++
	}

	result.Conflicts = detectedConflicts
	result.DurationMs = time.Since(startTime).Milliseconds()

	logger.Infof(ctx, "[ConflictDetection] Detection completed: scanned=%d new=%d existing=%d duration=%dms",
		result.TotalScanned, result.NewConflicts, result.ExistingConflicts, result.DurationMs)

	return result, nil
}

func (s *conflictDetectionService) listAllChunksForKB(
	ctx context.Context,
	tenantID uint64,
	kbID string,
) ([]*types.Chunk, error) {
	var allChunks []*types.Chunk
	page := 1
	pageSize := 200

	for {
		pagination := &types.Pagination{
			Page:     page,
			PageSize: pageSize,
		}
		chunks, total, err := s.chunkRepo.ListPagedChunksByKnowledgeID(
			ctx,
			tenantID,
			kbID,
			pagination,
			nil,
			"",
			"",
			"",
			"desc",
			"manual",
		)
		if err != nil {
			return nil, err
		}

		allChunks = append(allChunks, chunks...)

		if int64(len(allChunks)) >= total || len(chunks) == 0 {
			break
		}
		page++
	}

	return allChunks, nil
}

func (s *conflictDetectionService) GetConflict(
	ctx context.Context,
	tenantID uint64,
	conflictID string,
) (*types.Conflict, error) {
	return s.conflictRepo.GetByID(ctx, tenantID, conflictID)
}

func (s *conflictDetectionService) ListConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	status types.ConflictStatus,
	severity types.ConflictSeverity,
	conflictType types.ConflictType,
	page, pageSize int,
) ([]*types.Conflict, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.conflictRepo.List(ctx, tenantID, kbID, status, severity, conflictType, page, pageSize)
}

func (s *conflictDetectionService) ResolveConflict(
	ctx context.Context,
	tenantID uint64,
	req *types.ConflictResolutionRequest,
) (*types.Conflict, error) {
	if tenantID == 0 {
		return nil, fmt.Errorf("tenant_id is required")
	}

	conflict, err := s.conflictRepo.GetByID(ctx, tenantID, req.ConflictID)
	if err != nil {
		return nil, fmt.Errorf("conflict not found: %w", err)
	}

	conflict.ReviewerID = req.ReviewerID
	conflict.Resolution = req.Resolution
	conflict.ResolvedValue = req.ResolvedValue

	switch req.Action {
	case "resolve":
		conflict.Status = types.ConflictStatusResolved
	case "dismiss":
		conflict.Status = types.ConflictStatusDismissed
	default:
		return nil, fmt.Errorf("invalid action: %s", req.Action)
	}

	if err := s.conflictRepo.Update(ctx, conflict); err != nil {
		return nil, fmt.Errorf("failed to update conflict: %w", err)
	}

	return conflict, nil
}

func (s *conflictDetectionService) GetConflictSummary(
	ctx context.Context,
	tenantID uint64,
	kbID string,
) (*types.ConflictSummary, error) {
	byStatus, err := s.conflictRepo.CountByStatus(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}

	bySeverity, err := s.conflictRepo.CountBySeverity(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}

	byType, err := s.conflictRepo.CountByType(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}

	total := 0
	openConflicts := 0
	for status, count := range byStatus {
		total += count
		if status == types.ConflictStatusDetected || status == types.ConflictStatusReviewing {
			openConflicts += count
		}
	}

	criticalConflicts := bySeverity[types.ConflictSeverityCritical]

	recentConflicts, _, err := s.conflictRepo.List(ctx, tenantID, kbID, types.ConflictStatus(""), types.ConflictSeverity(""), types.ConflictType(""), 1, 5)
	if err != nil {
		logger.Warnf(ctx, "[ConflictDetection] Failed to get recent conflicts: %v", err)
	}

	return &types.ConflictSummary{
		Total:             total,
		ByStatus:          byStatus,
		BySeverity:        bySeverity,
		ByType:            byType,
		OpenConflicts:     openConflicts,
		CriticalConflicts: criticalConflicts,
		RecentConflicts:   recentConflicts,
	}, nil
}

func (s *conflictDetectionService) GetGovernanceSuggestion(
	ctx context.Context,
	tenantID uint64,
	conflictID string,
) (*types.ConflictGovernanceSuggestion, error) {
	conflict, err := s.conflictRepo.GetByID(ctx, tenantID, conflictID)
	if err != nil {
		return nil, err
	}

	suggestion := &types.ConflictGovernanceSuggestion{
		ConflictID: conflict.ID,
	}

	switch conflict.Type {
	case types.ConflictTypeAttributeValue:
		suggestion.SuggestionType = "value_reconciliation"
		suggestion.Title = fmt.Sprintf("Resolve conflicting values for %s.%s", conflict.EntityType, conflict.Attribute)
		suggestion.Description = conflict.Description
		suggestion.Options = s.generateValueOptions(conflict)
		suggestion.Recommended = s.recommendValue(conflict)
		suggestion.Rationale = "Recommended value selected based on source recency and document authority"

	case types.ConflictTypeParameterDef:
		suggestion.SuggestionType = "parameter_alignment"
		suggestion.Title = fmt.Sprintf("Align parameter definition for %s", conflict.Attribute)
		suggestion.Description = conflict.Description
		suggestion.Options = []string{"Use latest document version", "Use most authoritative source", "Merge definitions", "Mark for manual review"}
		suggestion.Recommended = "Use latest document version"
		suggestion.Rationale = "Parameter definitions should follow the latest documentation"

	case types.ConflictTypeTemporal:
		suggestion.SuggestionType = "temporal_resolution"
		suggestion.Title = "Resolve temporal information conflict"
		suggestion.Description = conflict.Description
		suggestion.Options = []string{"Use most recent information", "Keep historical record", "Add validity period", "Archive outdated information"}
		suggestion.Recommended = "Use most recent information"
		suggestion.Rationale = "Temporal conflicts should resolve to the most recent accurate data"

	default:
		suggestion.SuggestionType = "manual_review"
		suggestion.Title = "Review detected conflict"
		suggestion.Description = conflict.Description
		suggestion.Options = []string{"Mark as resolved", "Dismiss conflict", "Assign to reviewer"}
		suggestion.Recommended = "Assign to reviewer"
		suggestion.Rationale = "Complex conflicts require human review"
	}

	return suggestion, nil
}

func (s *conflictDetectionService) DetectPairwiseConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	docID1, docID2 string,
) ([]*types.Conflict, error) {
	req := &types.ConflictDetectionRequest{
		TenantID: tenantID,
		KBID:     kbID,
		DocIDs:   []string{docID1, docID2},
	}

	result, err := s.DetectConflicts(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Conflicts, nil
}

func (s *conflictDetectionService) DetectAttributeConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	entityType, attribute string,
) ([]*types.Conflict, error) {
	chunks, err := s.listAllChunksForKB(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}

	return s.detectSpecificAttributeConflicts(ctx, tenantID, kbID, chunks, entityType, attribute)
}

func (s *conflictDetectionService) DetectTemporalConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	maxAge time.Duration,
) ([]*types.Conflict, error) {
	chunks, err := s.listAllChunksForKB(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	return s.detectTemporalConflictsFromChunks(ctx, tenantID, kbID, chunks)
}

func (s *conflictDetectionService) detectAttributeValueConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	chunks []*types.Chunk,
) ([]*types.Conflict, error) {
	entityMap := make(map[string]map[string][]types.EntityAttribute)

	for _, chunk := range chunks {
		entities := s.extractEntities(chunk.Content)
		for i := range entities {
			entities[i].ChunkID = chunk.ID
			entities[i].SourceDocID = chunk.KnowledgeID
			attr := entities[i]
			if entityMap[attr.EntityType] == nil {
				entityMap[attr.EntityType] = make(map[string][]types.EntityAttribute)
			}
			entityMap[attr.EntityType][attr.Attribute] = append(entityMap[attr.EntityType][attr.Attribute], attr)
		}
	}

	var conflicts []*types.Conflict

	for entityType, attrs := range entityMap {
		for attrName, values := range attrs {
			if len(values) < 2 {
				continue
			}

			conflict := s.checkValueConflict(entityType, attrName, values)
			if conflict != nil {
				conflict.TenantID = tenantID
				conflict.KBID = kbID
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts, nil
}

func (s *conflictDetectionService) detectSpecificAttributeConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	chunks []*types.Chunk,
	entityType, attribute string,
) ([]*types.Conflict, error) {
	var values []types.EntityAttribute

	for _, chunk := range chunks {
		entities := s.extractEntities(chunk.Content)
		for i := range entities {
			if strings.EqualFold(entities[i].EntityType, entityType) && strings.EqualFold(entities[i].Attribute, attribute) {
				entities[i].ChunkID = chunk.ID
				entities[i].SourceDocID = chunk.KnowledgeID
				values = append(values, entities[i])
			}
		}
	}

	if len(values) < 2 {
		return nil, nil
	}

	conflict := s.checkValueConflict(entityType, attribute, values)
	if conflict == nil {
		return nil, nil
	}

	conflict.TenantID = tenantID
	conflict.KBID = kbID
	return []*types.Conflict{conflict}, nil
}

func (s *conflictDetectionService) checkValueConflict(entityType, attrName string, values []types.EntityAttribute) *types.Conflict {
	valueCounts := make(map[string][]types.EntityAttribute)
	for _, v := range values {
		normalized := strings.ToLower(strings.TrimSpace(v.Value))
		valueCounts[normalized] = append(valueCounts[normalized], v)
	}

	if len(valueCounts) <= 1 {
		return nil
	}

	conflictingValues := make([]string, 0, len(valueCounts))
	sourceChunks := make([]string, 0)
	sourceDocs := make([]string, 0)
	conflictAttrs := make([]types.EntityAttribute, 0, len(valueCounts))
	seenDocs := make(map[string]bool)
	seenChunks := make(map[string]bool)

	for val, attrs := range valueCounts {
		conflictingValues = append(conflictingValues, val)
		rep := attrs[0]
		rep.Attribute = attrName
		rep.Value = val
		conflictAttrs = append(conflictAttrs, rep)
		for _, attr := range attrs {
			if attr.ChunkID != "" && !seenChunks[attr.ChunkID] {
				sourceChunks = append(sourceChunks, attr.ChunkID)
				seenChunks[attr.ChunkID] = true
			}
			if attr.SourceDocID != "" && !seenDocs[attr.SourceDocID] {
				sourceDocs = append(sourceDocs, attr.SourceDocID)
				seenDocs[attr.SourceDocID] = true
			}
		}
	}

	severity := types.ConflictSeverityMedium
	if len(conflictingValues) > 3 {
		severity = types.ConflictSeverityHigh
	}

	var evidence []string
	for _, v := range values {
		evidence = append(evidence, fmt.Sprintf("[%s]: %s = %s", v.SourceDocID, attrName, v.Value))
	}

	return &types.Conflict{
		ID:          fmt.Sprintf("attr-%s-%s-%d", entityType, attrName, time.Now().UnixNano()),
		Type:        types.ConflictTypeAttributeValue,
		Severity:    severity,
		Status:      types.ConflictStatusDetected,
		EntityType:  entityType,
		Attribute:   attrName,
		Values:      conflictAttrs,
		AffectedDocs: sourceDocs,
		Description: fmt.Sprintf("Conflicting values for %s.%s: %v", entityType, attrName, conflictingValues),
		Suggestion:  "Review conflicting attribute values and select the correct one",
		DetectedBy:  "heuristic",
		Metadata: map[string]interface{}{
			"conflicting_values": conflictingValues,
			"source_chunks":      sourceChunks,
			"evidence":           evidence,
			"confidence":         0.75,
		},
	}
}

var attributePattern = regexp.MustCompile(`(?i)(?:the\s+)?(\w[\w\s]{0,30}?)\s+(?:is|are|was|were|equals|=|:)\s+["']?([^"'\n.;]{1,100})["']?`)
var knownEntities = []string{"version", "price", "limit", "threshold", "timeout", "size", "count", "number", "rate", "ratio", "percentage", "status", "state", "type", "mode", "level", "value", "setting", "config", "parameter", "option", "property", "field"}

func (s *conflictDetectionService) extractEntities(content string) []types.EntityAttribute {
	var entities []types.EntityAttribute

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		matches := attributePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			attrName := strings.ToLower(strings.TrimSpace(match[1]))
			value := strings.TrimSpace(match[2])

			for _, known := range knownEntities {
				if strings.Contains(attrName, known) {
					entityType := "document"
					parts := strings.Fields(attrName)
					if len(parts) >= 2 {
						entityType = strings.Join(parts[:len(parts)-1], "_")
						attrName = parts[len(parts)-1]
					}

					entities = append(entities, types.EntityAttribute{
						EntityType: entityType,
						Attribute:  attrName,
						Value:      value,
						Confidence: 0.6,
					})
					break
				}
			}
		}
	}

	return entities
}

func (s *conflictDetectionService) detectParameterDefConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	chunks []*types.Chunk,
) ([]*types.Conflict, error) {
	paramMap := make(map[string][]types.EntityAttribute)

	paramPattern := regexp.MustCompile(`(?i)(?:parameter|param|config|setting|flag)\s+["']?(\w+)["']?\s*(?:is|are|=|:|defaults? to|means)\s*["']?([^"'\n.;]{1,100})`)

	for _, chunk := range chunks {
		matches := paramPattern.FindAllStringSubmatch(chunk.Content, -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			paramName := strings.ToLower(strings.TrimSpace(match[1]))
			paramValue := strings.TrimSpace(match[2])

			paramMap[paramName] = append(paramMap[paramName], types.EntityAttribute{
				EntityType:  "parameter",
				Attribute:   paramName,
				Value:       paramValue,
				ChunkID:     chunk.ID,
				SourceDocID: chunk.KnowledgeID,
				Confidence:  0.85,
			})
		}
	}

	var conflicts []*types.Conflict

	for paramName, defs := range paramMap {
		if len(defs) < 2 {
			continue
		}

		valueSet := make(map[string]bool)
		var values []string
		var sourceChunks, sourceDocs []string
		var evidence []string
		seenDocs := make(map[string]bool)
		seenChunks := make(map[string]bool)
		conflictAttrs := make([]types.EntityAttribute, 0)

		for _, def := range defs {
			normalized := strings.ToLower(def.Value)
			if !valueSet[normalized] {
				valueSet[normalized] = true
				values = append(values, def.Value)
				conflictAttrs = append(conflictAttrs, def)
			}
			if def.ChunkID != "" && !seenChunks[def.ChunkID] {
				sourceChunks = append(sourceChunks, def.ChunkID)
				seenChunks[def.ChunkID] = true
			}
			if def.SourceDocID != "" && !seenDocs[def.SourceDocID] {
				sourceDocs = append(sourceDocs, def.SourceDocID)
				seenDocs[def.SourceDocID] = true
			}
			evidence = append(evidence, fmt.Sprintf("[%s]: %s = %s", def.SourceDocID, paramName, def.Value))
		}

		if len(valueSet) > 1 {
			severity := types.ConflictSeverityHigh
			conflicts = append(conflicts, &types.Conflict{
				ID:           fmt.Sprintf("param-%s-%d", paramName, time.Now().UnixNano()),
				Type:         types.ConflictTypeParameterDef,
				Severity:     severity,
				Status:       types.ConflictStatusDetected,
				EntityType:   "parameter",
				Attribute:    paramName,
				Values:       conflictAttrs,
				AffectedDocs: sourceDocs,
				Description:  fmt.Sprintf("Parameter '%s' has conflicting definitions: %v", paramName, values),
				Suggestion:   "Align parameter definition across documents using the latest version",
				DetectedBy:   "heuristic",
				TenantID:     tenantID,
				KBID:         kbID,
				Metadata: map[string]interface{}{
					"conflicting_values": values,
					"source_chunks":      sourceChunks,
					"evidence":           evidence,
					"confidence":         0.85,
				},
			})
		}
	}

	return conflicts, nil
}

func (s *conflictDetectionService) detectTemporalConflictsFromChunks(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	chunks []*types.Chunk,
) ([]*types.Conflict, error) {
	var conflicts []*types.Conflict

	datePattern := regexp.MustCompile(`\b(20\d{2}|19\d{2})[-/](0?[1-9]|1[0-2])[-/](0?[1-9]|[12]\d|3[01])\b|\b(?:January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+(20\d{2}|19\d{2})\b|\bas of\s+(\w+\s+\d{4}|Q[1-4]\s+\d{4}|\d{4})`)

	temporalMentions := make(map[string][]time.Time)
	chunkMap := make(map[string][]*types.Chunk)

	for _, chunk := range chunks {
		matches := datePattern.FindAllString(chunk.Content, -1)
		for _, match := range matches {
			t, err := parseDateFlexible(match)
			if err != nil {
				continue
			}

			key := s.extractTopic(chunk.Content, match)
			if key == "" {
				continue
			}

			temporalMentions[key] = append(temporalMentions[key], t)
			chunkMap[key] = append(chunkMap[key], chunk)
		}
	}

	now := time.Now()

	for topic, dates := range temporalMentions {
		if len(dates) < 2 {
			var latest time.Time
			for _, d := range dates {
				if d.After(latest) {
					latest = d
				}
			}

			age := now.Sub(latest)
			if age > 18*30*24*time.Hour {
				chunks := chunkMap[topic]
				if len(chunks) > 0 {
					conflicts = append(conflicts, &types.Conflict{
						ID:           fmt.Sprintf("temporal-%s-%d", sanitizeKey(topic), time.Now().UnixNano()),
						Type:         types.ConflictTypeTemporal,
						Severity:     types.ConflictSeverityMedium,
						Status:       types.ConflictStatusDetected,
						EntityType:   "temporal",
						Attribute:    topic,
						Description:  fmt.Sprintf("Potentially outdated information: '%s' last dated %v (%.0f months old)", topic, latest.Format("2006-01-02"), age.Hours()/24/30),
						Values:       []types.EntityAttribute{{EntityType: "temporal", Attribute: topic, Value: latest.Format("2006-01-02"), ChunkID: chunks[0].ID, SourceDocID: chunks[0].KnowledgeID, Confidence: 0.6}},
						AffectedDocs: []string{chunks[0].KnowledgeID},
						Suggestion:   "Review outdated information and update if necessary",
						DetectedBy:   "temporal_analysis",
						TenantID:     tenantID,
						KBID:         kbID,
						Metadata: map[string]interface{}{
							"source_chunks": []string{chunks[0].ID},
							"evidence":      []string{fmt.Sprintf("Mention date: %v in chunk %s", latest.Format("2006-01-02"), chunks[0].ID)},
							"confidence":    0.6,
						},
					})
				}
			}
			continue
		}

		var minDate, maxDate time.Time
		for i, d := range dates {
			if i == 0 || d.Before(minDate) {
				minDate = d
			}
			if i == 0 || d.After(maxDate) {
				maxDate = d
			}
		}

		dateRange := maxDate.Sub(minDate)
		if dateRange > 90*24*time.Hour {
			chunks := chunkMap[topic]
			sourceChunks := make([]string, len(chunks))
			sourceDocs := make([]string, 0)
			evidence := make([]string, len(chunks))
			seenDocs := make(map[string]bool)
			conflictAttrs := make([]types.EntityAttribute, 0)
			for i, c := range chunks {
				sourceChunks[i] = c.ID
				if c.KnowledgeID != "" && !seenDocs[c.KnowledgeID] {
					sourceDocs = append(sourceDocs, c.KnowledgeID)
					seenDocs[c.KnowledgeID] = true
				}
				evidence[i] = fmt.Sprintf("[%s] dated mention in chunk %s", c.KnowledgeID, c.ID)
			}
			conflictAttrs = append(conflictAttrs,
				types.EntityAttribute{EntityType: "temporal", Attribute: topic, Value: minDate.Format("2006-01-02"), Confidence: 0.5},
				types.EntityAttribute{EntityType: "temporal", Attribute: topic, Value: maxDate.Format("2006-01-02"), Confidence: 0.5},
			)
			conflicts = append(conflicts, &types.Conflict{
				ID:           fmt.Sprintf("temporal-range-%s-%d", sanitizeKey(topic), time.Now().UnixNano()),
				Type:         types.ConflictTypeTemporal,
				Severity:     types.ConflictSeverityLow,
				Status:       types.ConflictStatusDetected,
				EntityType:   "temporal",
				Attribute:    topic,
				Description:  fmt.Sprintf("Multiple date references for '%s': range from %v to %v (%.0f days)", topic, minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"), dateRange.Hours()/24),
				Values:       conflictAttrs,
				AffectedDocs: sourceDocs,
				Suggestion:   "Review date references and use the most recent information",
				DetectedBy:   "temporal_analysis",
				TenantID:     tenantID,
				KBID:         kbID,
				Metadata: map[string]interface{}{
					"conflicting_values": []string{minDate.Format("2006-01-02"), maxDate.Format("2006-01-02")},
					"source_chunks":      sourceChunks,
					"evidence":           evidence,
					"confidence":         0.5,
				},
			})
		}
	}

	return conflicts, nil
}

func (s *conflictDetectionService) detectNumericalConflicts(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	chunks []*types.Chunk,
) ([]*types.Conflict, error) {
	var conflicts []*types.Conflict

	numberPattern := regexp.MustCompile(`(?i)(\w[\w\s]{0,30}?)\s*(?:is|=|:|equals|amounts to|reaches)\s*["']?(\d+(?:\.\d+)?)\s*(%|percent|ms|seconds?|minutes?|hours?|days?|MB|GB|TB|KB|bytes?|units?|requests?|times?)?`)

	numMap := make(map[string][]struct {
		value  float64
		unit   string
		chunk  *types.Chunk
	})

	for _, chunk := range chunks {
		matches := numberPattern.FindAllStringSubmatch(chunk.Content, -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			attrName := strings.ToLower(strings.TrimSpace(match[1]))
			numStr := match[2]
			unit := ""
			if len(match) > 3 {
				unit = strings.ToLower(match[3])
			}

			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				continue
			}

			if val > 0 && (strings.Contains(attrName, "limit") || strings.Contains(attrName, "max") ||
				strings.Contains(attrName, "threshold") || strings.Contains(attrName, "timeout") ||
				strings.Contains(attrName, "size") || strings.Contains(attrName, "rate") ||
				strings.Contains(attrName, "count") || strings.Contains(attrName, "number") ||
				strings.Contains(attrName, "value") || strings.Contains(attrName, "ratio") ||
				strings.Contains(attrName, "percent")) {

				key := attrName + "|" + unit
				numMap[key] = append(numMap[key], struct {
					value  float64
					unit   string
					chunk  *types.Chunk
				}{val, unit, chunk})
			}
		}
	}

	for key, entries := range numMap {
		if len(entries) < 2 {
			continue
		}

		parts := strings.SplitN(key, "|", 2)
		attrName := parts[0]
		unit := parts[1]

		var minVal, maxVal float64
		for i, e := range entries {
			if i == 0 || e.value < minVal {
				minVal = e.value
			}
			if i == 0 || e.value > maxVal {
				maxVal = e.value
			}
		}

		ratio := 1.0
		if minVal > 0 {
			ratio = maxVal / minVal
		}

		if ratio > 1.5 {
			var values []string
			var sourceChunks, sourceDocs []string
			var evidence []string
			seenDocs := make(map[string]bool)
			var conflictAttrs []types.EntityAttribute

			for _, e := range entries {
				valStr := fmt.Sprintf("%v", e.value)
				if unit != "" {
					valStr += " " + unit
				}
				values = append(values, valStr)
				sourceChunks = append(sourceChunks, e.chunk.ID)
				if e.chunk.KnowledgeID != "" && !seenDocs[e.chunk.KnowledgeID] {
					sourceDocs = append(sourceDocs, e.chunk.KnowledgeID)
					seenDocs[e.chunk.KnowledgeID] = true
				}
				evidence = append(evidence, fmt.Sprintf("[%s]: %s = %s", e.chunk.KnowledgeID, attrName, valStr))
				conflictAttrs = append(conflictAttrs, types.EntityAttribute{
					EntityType:  "numerical",
					Attribute:   attrName,
					Value:       valStr,
					ChunkID:     e.chunk.ID,
					SourceDocID: e.chunk.KnowledgeID,
					Confidence:  0.7,
				})
			}

			severity := types.ConflictSeverityMedium
			if ratio > 3 {
				severity = types.ConflictSeverityHigh
			}

			conflicts = append(conflicts, &types.Conflict{
				ID:           fmt.Sprintf("num-%s-%d", sanitizeKey(attrName), time.Now().UnixNano()),
				Type:         types.ConflictTypeNumerical,
				Severity:     severity,
				Status:       types.ConflictStatusDetected,
				EntityType:   "numerical",
				Attribute:    attrName,
				Values:       conflictAttrs,
				AffectedDocs: sourceDocs,
				Description:  fmt.Sprintf("Numerical value discrepancy for '%s': values range from %v to %v%s (%.1fx difference)", attrName, minVal, maxVal, " "+unit, ratio),
				Suggestion:   "Review numerical values and verify the correct figure",
				DetectedBy:   "numerical_analysis",
				TenantID:     tenantID,
				KBID:         kbID,
				Metadata: map[string]interface{}{
					"conflicting_values": values,
					"source_chunks":      sourceChunks,
					"evidence":           evidence,
					"confidence":         0.7,
					"ratio":              ratio,
				},
			})
		}
	}

	return conflicts, nil
}

func (s *conflictDetectionService) extractTopic(content, dateStr string) string {
	idx := strings.Index(content, dateStr)
	if idx < 0 {
		return ""
	}

	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := idx + len(dateStr) + 20
	if end > len(content) {
		end = len(content)
	}

	context := content[start:end]
	stopWords := map[string]bool{"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true, "in": true, "on": true, "at": true, "to": true, "for": true, "of": true, "and": true, "or": true, "as": true, "by": true, "with": true}

	words := strings.Fields(strings.ToLower(context))
	var topicWords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len(w) > 3 && !stopWords[w] {
			topicWords = append(topicWords, w)
		}
		if len(topicWords) >= 3 {
			break
		}
	}

	if len(topicWords) == 0 {
		return ""
	}

	return strings.Join(topicWords, "_")
}

func (s *conflictDetectionService) generateValueOptions(conflict *types.Conflict) []string {
	valueStrs := make([]string, 0, len(conflict.Values)+2)
	for _, v := range conflict.Values {
		valueStrs = append(valueStrs, v.Value)
	}
	if raw, ok := conflict.Metadata["conflicting_values"]; ok {
		if arr, ok := raw.([]string); ok {
			valueStrs = arr
		}
	}
	options := make([]string, 0, len(valueStrs)+2)
	options = append(options, valueStrs...)
	options = append(options, "Merge values", "Mark for manual review")
	return options
}

func (s *conflictDetectionService) recommendValue(conflict *types.Conflict) string {
	if len(conflict.Values) > 0 {
		return conflict.Values[len(conflict.Values)-1].Value
	}
	if raw, ok := conflict.Metadata["conflicting_values"]; ok {
		if arr, ok := raw.([]string); ok && len(arr) > 0 {
			return arr[len(arr)-1]
		}
	}
	return ""
}

func parseDateFlexible(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02", "2006/1/2", "2006/01/02",
		"January 2, 2006", "Jan 2, 2006", "January 2006", "Jan 2006",
		"2006",
	}
	s = strings.TrimSpace(s)
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	if strings.HasPrefix(strings.ToLower(s), "q") {
		if len(s) >= 6 {
			yearStr := s[len(s)-4:]
			year, err := strconv.Atoi(yearStr)
			if err == nil {
				quarter := s[1]
				month := 1
				switch quarter {
				case '2':
					month = 4
				case '3':
					month = 7
				case '4':
					month = 10
				}
				return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
			}
		}
	}

	if strings.HasPrefix(strings.ToLower(s), "as of") {
		rest := strings.TrimPrefix(strings.ToLower(s), "as of")
		rest = strings.TrimSpace(rest)
		return parseDateFlexible(rest)
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", s)
}

func sanitizeKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
