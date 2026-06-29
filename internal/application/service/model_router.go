package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// ModelRouterServiceImpl implements model routing with cost/latency optimization
type ModelRouterServiceImpl struct {
	modelRepo       interfaces.ModelRepository
	costService     interfaces.CostTrackingService
	mu              sync.RWMutex
	policyCache     map[uint64]*types.ModelRoutingPolicy
	perfStatsCache  map[string]*types.ModelPerformanceStats
}

// NewModelRouterService creates a new model router service
func NewModelRouterService(
	modelRepo interfaces.ModelRepository,
	costService interfaces.CostTrackingService,
) interfaces.ModelRouterService {
	return &ModelRouterServiceImpl{
		modelRepo:      modelRepo,
		costService:    costService,
		policyCache:    make(map[uint64]*types.ModelRoutingPolicy),
		perfStatsCache: make(map[string]*types.ModelPerformanceStats),
	}
}

// SelectModel selects the most appropriate model based on criteria
func (s *ModelRouterServiceImpl) SelectModel(
	ctx context.Context,
	req *types.ModelSelectRequest,
) (*types.ModelSelectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	logger.Infof(ctx, "[ModelRouter] Selecting model for tenant=%d task=%s security=%s",
		req.TenantID, req.TaskType, req.SecurityLevel)

	// Step 1: Get or create default routing policy
	policy, err := s.getOrCreatePolicy(ctx, req.TenantID)
	if err != nil {
		logger.Warnf(ctx, "[ModelRouter] Failed to get policy, using defaults: %v", err)
		policy = s.getDefaultPolicy(req.TenantID)
	}

	// Step 2: Security enforcement - high security tasks only use internal models
	if req.SecurityLevel == types.SecurityLevelHigh || req.SecurityLevel == types.SecurityLevelCritical {
		if forcedModel, ok := policy.SecurityForcedModels[req.SecurityLevel]; ok && forcedModel != "" {
			return s.buildResult(ctx, forcedModel, types.RoutingStrategySecurityForced,
				fmt.Sprintf("Security level %s forced model", req.SecurityLevel), false, "")
		}
		if !policy.AllowExternalModels {
			internalModel, err := s.selectInternalModel(ctx, req, policy)
			if err == nil && internalModel != "" {
				return s.buildResult(ctx, internalModel, types.RoutingStrategySecurityForced,
					"High security task using internal model only", false, "")
			}
		}
	}

	// Step 3: Check budget and apply cost/latency strategy
	budgetExceeded, currentSpend, budgetLimit, err := s.CheckBudgetExceeded(ctx, req.TenantID)
	if err != nil {
		logger.Warnf(ctx, "[ModelRouter] Budget check failed: %v", err)
	}

	// Step 4: Apply user-specified preference if valid
	if req.PreferModelID != "" {
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, req.PreferModelID)
		if err == nil && model != nil && model.Status == types.ModelStatusActive {
			canUse, reason := s.canUseModel(ctx, model, req, budgetExceeded)
			if canUse {
				return s.buildResult(ctx, req.PreferModelID, types.RoutingStrategyDefault,
					"User specified model preference", budgetExceeded, "")
			}
			logger.Infof(ctx, "[ModelRouter] Preferred model %s not usable: %s", req.PreferModelID, reason)
		}
	}

	// Step 5: Select based on strategy
	strategy := policy.Strategy
	maxLatency := req.MaxLatencyMs
	if maxLatency <= 0 {
		maxLatency = policy.MaxLatencyMs
	}

	availableModels, err := s.getAvailableModels(ctx, req)
	if err != nil || len(availableModels) == 0 {
		logger.Warnf(ctx, "[ModelRouter] Failed to get available models, using default: %v", err)
		return s.buildResult(ctx, policy.DefaultModelID, types.RoutingStrategyDefault,
			"Using default model - no available models found", budgetExceeded, "")
	}

	// Step 6: If budget exceeded, fall back to cheapest model
	if budgetExceeded {
		logger.Infof(ctx, "[ModelRouter] Budget exceeded: current=%.4f limit=%.4f, selecting cheapest model",
			currentSpend, budgetLimit)
		cheapest := s.selectCheapestModel(ctx, availableModels, req)
		if cheapest != "" {
			return s.buildResult(ctx, cheapest, types.RoutingStrategyFallback,
				fmt.Sprintf("Budget exceeded (%.4f/%.4f), using cheapest model", currentSpend, budgetLimit),
				true, "")
		}
	}

	// Step 7: Apply routing strategy
	var selectedModel string
	var reason string

	switch strategy {
	case types.RoutingStrategyCostOptimized:
		selectedModel = s.selectCostOptimized(ctx, availableModels, req, maxLatency)
		reason = "Cost optimized routing within latency constraint"
	case types.RoutingStrategyLatencyOptimized:
		selectedModel = s.selectLatencyOptimized(ctx, availableModels, req, maxLatency)
		reason = "Latency optimized routing within cost constraint"
	case types.RoutingStrategyQualityFirst:
		selectedModel = s.selectHighestQuality(ctx, availableModels, req)
		reason = "Quality first routing"
	default:
		selectedModel = s.selectBalanced(ctx, availableModels, req, maxLatency)
		reason = "Balanced routing (cost/latency/quality)"
	}

	// Fallback to default if selection failed
	if selectedModel == "" {
		selectedModel = policy.DefaultModelID
		strategy = types.RoutingStrategyDefault
		reason = "Falling back to default model"
		if selectedModel == "" {
			selectedModel = availableModels[0].ID
		}
	}

	return s.buildResult(ctx, selectedModel, strategy, reason, budgetExceeded, "")
}

// GetRoutingPolicy retrieves routing policy for a tenant
func (s *ModelRouterServiceImpl) GetRoutingPolicy(ctx context.Context, tenantID uint64) (*types.ModelRoutingPolicy, error) {
	s.mu.RLock()
	if cached, ok := s.policyCache[tenantID]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	policy := s.getDefaultPolicy(tenantID)
	s.mu.Lock()
	s.policyCache[tenantID] = policy
	s.mu.Unlock()
	return policy, nil
}

// UpdateRoutingPolicy updates the routing policy
func (s *ModelRouterServiceImpl) UpdateRoutingPolicy(ctx context.Context, policy *types.ModelRoutingPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}
	if policy.TenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	if policy.DefaultModelID == "" {
		return fmt.Errorf("default_model_id is required")
	}

	policy.UpdatedAt = time.Now()
	s.mu.Lock()
	s.policyCache[policy.TenantID] = policy
	s.mu.Unlock()

	logger.Infof(ctx, "[ModelRouter] Updated routing policy for tenant=%d strategy=%s",
		policy.TenantID, policy.Strategy)
	return nil
}

// RecordModelPerformance records performance metrics
func (s *ModelRouterServiceImpl) RecordModelPerformance(ctx context.Context, stats *types.ModelPerformanceStats) error {
	if stats == nil || stats.ModelID == "" {
		return fmt.Errorf("invalid stats")
	}
	stats.LastUpdated = time.Now()
	s.mu.Lock()
	s.perfStatsCache[stats.ModelID] = stats
	s.mu.Unlock()
	return nil
}

// CheckBudgetExceeded checks if tenant has exceeded their budget
func (s *ModelRouterServiceImpl) CheckBudgetExceeded(ctx context.Context, tenantID uint64) (bool, float64, float64, error) {
	policy, err := s.GetRoutingPolicy(ctx, tenantID)
	if err != nil {
		return false, 0, 0, err
	}

	// If no budget set, never exceed
	if policy.DailyBudgetUSD <= 0 && policy.MonthlyBudgetUSD <= 0 {
		return false, 0, 0, nil
	}

	// Check daily budget
	if policy.DailyBudgetUSD > 0 {
		today := time.Now().Truncate(24 * time.Hour)
		dailySpend, err := s.getSpendSince(ctx, tenantID, today)
		if err != nil {
			logger.Warnf(ctx, "[ModelRouter] Failed to get daily spend: %v", err)
		} else if dailySpend >= policy.DailyBudgetUSD {
			return true, dailySpend, policy.DailyBudgetUSD, nil
		}
	}

	// Check monthly budget
	if policy.MonthlyBudgetUSD > 0 {
		monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())
		monthlySpend, err := s.getSpendSince(ctx, tenantID, monthStart)
		if err != nil {
			logger.Warnf(ctx, "[ModelRouter] Failed to get monthly spend: %v", err)
		} else if monthlySpend >= policy.MonthlyBudgetUSD {
			return true, monthlySpend, policy.MonthlyBudgetUSD, nil
		}
	}

	return false, 0, 0, nil
}

// Helper functions

func (s *ModelRouterServiceImpl) getOrCreatePolicy(ctx context.Context, tenantID uint64) (*types.ModelRoutingPolicy, error) {
	s.mu.RLock()
	if cached, ok := s.policyCache[tenantID]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()
	return s.getDefaultPolicy(tenantID), nil
}

func (s *ModelRouterServiceImpl) getDefaultPolicy(tenantID uint64) *types.ModelRoutingPolicy {
	return &types.ModelRoutingPolicy{
		TenantID:            tenantID,
		DefaultModelID:      "doubao-pro-32k",
		Strategy:            types.RoutingStrategyCostOptimized,
		MaxLatencyMs:        30000,
		AllowExternalModels: true,
		SecurityForcedModels: map[types.SecurityLevel]string{
			types.SecurityLevelCritical: "",
			types.SecurityLevelHigh:     "",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *ModelRouterServiceImpl) getAvailableModels(ctx context.Context, req *types.ModelSelectRequest) ([]*types.Model, error) {
	models, err := s.modelRepo.List(ctx, req.TenantID, types.ModelTypeKnowledgeQA, "")
	if err != nil {
		// Fallback to system models
		models, err = s.modelRepo.List(ctx, 0, types.ModelTypeKnowledgeQA, "")
		if err != nil {
			return nil, err
		}
	}

	var available []*types.Model
	for _, m := range models {
		if m.Status == types.ModelStatusActive {
			available = append(available, m)
		}
	}
	return available, nil
}

func (s *ModelRouterServiceImpl) canUseModel(ctx context.Context, model *types.Model, req *types.ModelSelectRequest, budgetExceeded bool) (bool, string) {
	if model == nil || model.Status != types.ModelStatusActive {
		return false, "model disabled"
	}
	if req.SecurityLevel == types.SecurityLevelHigh || req.SecurityLevel == types.SecurityLevelCritical {
		// Check if model is external (non-local)
		isExternal := model.Source != types.ModelSourceLocal && model.Source != ""
		if isExternal {
			policy, _ := s.GetRoutingPolicy(ctx, req.TenantID)
			if policy != nil && !policy.AllowExternalModels {
				return false, "external models not allowed for high security"
			}
		}
	}
	if budgetExceeded {
		return true, ""
	}
	return true, ""
}

func (s *ModelRouterServiceImpl) selectInternalModel(ctx context.Context, req *types.ModelSelectRequest, policy *types.ModelRoutingPolicy) (string, error) {
	models, err := s.getAvailableModels(ctx, req)
	if err != nil {
		return "", err
	}
	for _, m := range models {
		if m.Source == types.ModelSourceLocal || m.Source == "" {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("no internal models available")
}

func (s *ModelRouterServiceImpl) getModelPerf(modelID string) *types.ModelPerformanceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if stats, ok := s.perfStatsCache[modelID]; ok {
		return stats
	}
	return nil
}

func (s *ModelRouterServiceImpl) getModelCostEstimate(model *types.Model, inputTokens int) float64 {
	if model == nil {
		return 0
	}
	usage := &types.TokenUsage{
		PromptTokens:     inputTokens,
		CompletionTokens: inputTokens / 2,
	}
	return model.CalculateCost(usage)
}

func (s *ModelRouterServiceImpl) getModelLatencyEstimate(model *types.Model) int {
	if model == nil {
		return 30000
	}
	stats := s.getModelPerf(model.ID)
	if stats != nil && stats.AvgLatencyMs > 0 {
		return stats.AvgLatencyMs
	}
	switch getModelTier(model) {
	case types.ModelTierBasic:
		return 5000
	case types.ModelTierStandard:
		return 10000
	case types.ModelTierAdvanced:
		return 20000
	case types.ModelTierPremium:
		return 30000
	default:
		return 15000
	}
}

func (s *ModelRouterServiceImpl) selectCheapestModel(ctx context.Context, models []*types.Model, req *types.ModelSelectRequest) string {
	sort.Slice(models, func(i, j int) bool {
		costI := s.getModelCostEstimate(models[i], req.EstimatedInputTokens)
		costJ := s.getModelCostEstimate(models[j], req.EstimatedInputTokens)
		return costI < costJ
	})
	for _, m := range models {
		canUse, _ := s.canUseModel(ctx, m, req, true)
		if canUse {
			return m.ID
		}
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}

func (s *ModelRouterServiceImpl) selectCostOptimized(ctx context.Context, models []*types.Model, req *types.ModelSelectRequest, maxLatencyMs int) string {
	type candidate struct {
		model  *types.Model
		cost   float64
		latency int
	}
	var candidates []candidate
	for _, m := range models {
		canUse, _ := s.canUseModel(ctx, m, req, false)
		if !canUse {
			continue
		}
		latency := s.getModelLatencyEstimate(m)
		if maxLatencyMs > 0 && latency > maxLatencyMs {
			continue
		}
		candidates = append(candidates, candidate{
			model:  m,
			cost:   s.getModelCostEstimate(m, req.EstimatedInputTokens),
			latency: latency,
		})
	}
	if len(candidates) == 0 {
		return s.selectCheapestModel(ctx, models, req)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})
	return candidates[0].model.ID
}

func (s *ModelRouterServiceImpl) selectLatencyOptimized(ctx context.Context, models []*types.Model, req *types.ModelSelectRequest, maxLatencyMs int) string {
	type candidate struct {
		model   *types.Model
		cost    float64
		latency int
	}
	var candidates []candidate
	maxCost := req.MaxCostPerCall
	for _, m := range models {
		canUse, _ := s.canUseModel(ctx, m, req, false)
		if !canUse {
			continue
		}
		cost := s.getModelCostEstimate(m, req.EstimatedInputTokens)
		if maxCost > 0 && cost > maxCost {
			continue
		}
		candidates = append(candidates, candidate{
			model:   m,
			cost:    cost,
			latency: s.getModelLatencyEstimate(m),
		})
	}
	if len(candidates) == 0 {
		return s.selectCheapestModel(ctx, models, req)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].latency < candidates[j].latency
	})
	return candidates[0].model.ID
}

func (s *ModelRouterServiceImpl) selectHighestQuality(ctx context.Context, models []*types.Model, req *types.ModelSelectRequest) string {
	tierOrder := map[types.ModelTier]int{
		types.ModelTierBasic:    1,
		types.ModelTierStandard: 2,
		types.ModelTierAdvanced: 3,
		types.ModelTierPremium:  4,
	}
	sort.Slice(models, func(i, j int) bool {
		tierI := tierOrder[getModelTier(models[i])]
		tierJ := tierOrder[getModelTier(models[j])]
		return tierI > tierJ
	})
	for _, m := range models {
		canUse, _ := s.canUseModel(ctx, m, req, false)
		if canUse {
			return m.ID
		}
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}

func (s *ModelRouterServiceImpl) selectBalanced(ctx context.Context, models []*types.Model, req *types.ModelSelectRequest, maxLatencyMs int) string {
	type candidate struct {
		model  *types.Model
		score  float64
	}
	var candidates []candidate
	for _, m := range models {
		canUse, _ := s.canUseModel(ctx, m, req, false)
		if !canUse {
			continue
		}
		cost := s.getModelCostEstimate(m, req.EstimatedInputTokens)
		latency := s.getModelLatencyEstimate(m)
		tierScore := float64(0)
		switch getModelTier(m) {
		case types.ModelTierBasic:
			tierScore = 1
		case types.ModelTierStandard:
			tierScore = 2
		case types.ModelTierAdvanced:
			tierScore = 3
		case types.ModelTierPremium:
			tierScore = 4
		}
		// Normalize and score: lower cost and latency better, higher tier better
		normCost := 1.0 / (cost + 0.0001)
		normLatency := 1.0 / float64(latency+100)
		score := normCost*0.4 + normLatency*0.3 + tierScore*0.3
		if maxLatencyMs > 0 && latency > maxLatencyMs {
			score *= 0.5 // Penalize over latency
		}
		candidates = append(candidates, candidate{model: m, score: score})
	}
	if len(candidates) == 0 {
		return s.selectCheapestModel(ctx, models, req)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].model.ID
}

func (s *ModelRouterServiceImpl) buildResult(
	ctx context.Context,
	modelID string,
	strategy types.ModelRoutingStrategy,
	reason string,
	budgetExceeded bool,
	fallbackFrom string,
) (*types.ModelSelectResult, error) {
	model, err := s.modelRepo.GetByIDAnyTenant(ctx, modelID)
	modelName := modelID
	if err == nil && model != nil {
		if model.DisplayName != "" {
			modelName = model.DisplayName
		} else {
			modelName = model.Name
		}
	}

	return &types.ModelSelectResult{
		ModelID:   modelID,
		ModelName: modelName,
		RouteDecision: types.RouteDecision{
			Strategy:       strategy,
			SelectedModel:  modelID,
			Reason:         reason,
			FallbackFrom:   fallbackFrom,
			BudgetExceeded: budgetExceeded,
		},
		EstimatedCost:      s.getModelCostEstimate(model, 1000),
		EstimatedLatencyMs: s.getModelLatencyEstimate(model),
	}, nil
}

func (s *ModelRouterServiceImpl) getSpendSince(ctx context.Context, tenantID uint64, since time.Time) (float64, error) {
	end := time.Now()
	days := int(math.Ceil(end.Sub(since).Hours() / 24))
	if days <= 0 {
		days = 1
	}
	dashboard, err := s.costService.GetCostDashboard(ctx, tenantID, days)
	if err != nil {
		return 0, err
	}
	return dashboard.TotalCost, nil
}

func getModelTier(model *types.Model) types.ModelTier {
	if model == nil {
		return types.ModelTierStandard
	}
	if model.IsBuiltin {
		return types.ModelTierPremium
	}
	if model.IsDefault {
		return types.ModelTierAdvanced
	}
	price := model.InputPricePerMillion + model.OutputPricePerMillion
	if price <= 0 {
		return types.ModelTierStandard
	}
	if price < 1.0 {
		return types.ModelTierBasic
	}
	if price < 10.0 {
		return types.ModelTierStandard
	}
	if price < 50.0 {
		return types.ModelTierAdvanced
	}
	return types.ModelTierPremium
}
