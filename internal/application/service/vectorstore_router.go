package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// RouterStats 路由层统计信息
type RouterStats struct {
	TotalStores    int `json:"total_stores"`
	TotalReplicas  int `json:"total_replicas"`
	HealthyNodes   int `json:"healthy_nodes"`
	UnhealthyNodes int `json:"unhealthy_nodes"`
}

// storeEntry 单个存储的路由条目
type storeEntry struct {
	mu           sync.RWMutex
	config       types.ReadWriteSeparationConfig
	master       interfaces.RWCapableEngine
	replicas     []interfaces.ReadableNode
	readBalancer LoadBalancer
	healthStates map[string]*nodeHealthState
	lsnTracker   *lsnTracker
	writeBuffer  WriteBuffer
	stopCh       chan struct{}
}

type nodeHealthState struct {
	health         *types.NodeHealth
	consecutiveFails int
	circuitState   types.CircuitBreakerState
	lastFailTime   time.Time
	lastCheckTime  time.Time
}

// VectorStoreRouter 向量存储读写分离路由器
type VectorStoreRouter struct {
	mu         sync.RWMutex
	stores     map[string]*storeEntry
	config     types.ReadWriteSeparationConfig
	globalStop chan struct{}
	wg         sync.WaitGroup
}

// NewVectorStoreRouter 创建新的向量存储路由器
func NewVectorStoreRouter(cfg types.ReadWriteSeparationConfig) *VectorStoreRouter {
	return &VectorStoreRouter{
		stores:     make(map[string]*storeEntry),
		config:     cfg,
		globalStop: make(chan struct{}),
	}
}

// GetReader 获取读接口，根据一致性级别路由到合适节点
func (r *VectorStoreRouter) GetReader(ctx context.Context, storeID string, consistency types.ConsistencyLevel, token *types.WriteToken) (interfaces.ReadableNode, error) {
	startTime := time.Now()
	r.mu.RLock()
	entry, ok := r.stores[storeID]
	r.mu.RUnlock()
	if !ok {
		logger.Errorf(ctx, "[VectorStoreRouter] GetReader failed: store %s not found", storeID)
		readRequestsTotal.WithLabelValues(storeID, "none", consistency.String(), "error").Inc()
		return nil, fmt.Errorf("store %s not found", storeID)
	}

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	var nodeType string
	rwEnabled := entry.config.Enabled
	replicaCount := len(entry.replicas)
	logger.Infof(ctx, "[VectorStoreRouter] GetReader request: storeID=%s, consistency=%s, rwEnabled=%v, replicaCount=%d, hasToken=%v",
		storeID, consistency.String(), rwEnabled, replicaCount, token != nil)

	// 未开启读写分离或没有配置副本，直接返回主节点
	if !rwEnabled || replicaCount == 0 {
		nodeType = "master"
		reason := "rw_disabled"
		if replicaCount == 0 {
			reason = "no_replicas"
		}
		logger.Infof(ctx, "[VectorStoreRouter] Route to master: reason=%s, storeID=%s", reason, storeID)
		readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "success").Inc()
		requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
		return entry.master, nil
	}

	// 强一致性：直接读主
	if consistency == types.ConsistencyLevelStrong {
		nodeType = "master"
		logger.Infof(ctx, "[VectorStoreRouter] Route to master: reason=strong_consistency, storeID=%s", storeID)
		readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "success").Inc()
		requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
		return entry.master, nil
	}

	// 会话一致性：需要确保能读到指定LSN之后的数据
	if consistency == types.ConsistencyLevelSession && token != nil {
		logger.Infof(ctx, "[VectorStoreRouter] Session consistency: requiredLSN=%d, storeID=%s", token.LSN, storeID)
		reader, err := r.selectReaderForLSN(ctx, entry, token.LSN)
		if err != nil {
			// 等待超时降级读主
			nodeType = "master"
			logger.Warnf(ctx, "[VectorStoreRouter] Fallback to master: reason=lsn_wait_timeout, requiredLSN=%d, err=%v, storeID=%s",
				token.LSN, err, storeID)
			readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "fallback").Inc()
			requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
			return entry.master, nil
		}
		nodeType = "replica"
		logger.Infof(ctx, "[VectorStoreRouter] Route to replica: reason=session_consistency_met, requiredLSN=%d, storeID=%s",
			token.LSN, storeID)
		readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "success").Inc()
		requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
		return reader, nil
	}

	// 最终一致：通过负载均衡器选一个健康副本
	healthyPools := r.getHealthyReplicas(entry)
	healthyCount := len(healthyPools)
	if healthyCount == 0 {
		// 没有健康副本，降级读主
		nodeType = "master"
		logger.Warnf(ctx, "[VectorStoreRouter] Fallback to master: reason=no_healthy_replicas, storeID=%s", storeID)
		readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "fallback").Inc()
		requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
		return entry.master, nil
	}

	logger.Infof(ctx, "[VectorStoreRouter] Healthy replicas available: count=%d, storeID=%s", healthyCount, storeID)
	selected := entry.readBalancer.SelectNode(healthyPools)
	if selected == nil {
		nodeType = "master"
		logger.Warnf(ctx, "[VectorStoreRouter] Fallback to master: reason=load_balancer_returned_nil, storeID=%s", storeID)
		readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "fallback").Inc()
		requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
		return entry.master, nil
	}
	nodeType = "replica"
	logger.Infof(ctx, "[VectorStoreRouter] Route to replica via load balancer: healthyCount=%d, storeID=%s", healthyCount, storeID)
	readRequestsTotal.WithLabelValues(storeID, nodeType, consistency.String(), "success").Inc()
	requestLatency.WithLabelValues(storeID, "get_reader", nodeType).Observe(time.Since(startTime).Seconds())
	return selected, nil
}

// GetWriter 获取写接口，路由到主节点，返回原接口保证完全兼容
func (r *VectorStoreRouter) GetWriter(ctx context.Context, storeID string) (interfaces.RetrieveEngineService, error) {
	startTime := time.Now()
	r.mu.RLock()
	entry, ok := r.stores[storeID]
	r.mu.RUnlock()
	if !ok {
		logger.Errorf(ctx, "[VectorStoreRouter] GetWriter failed: store %s not found", storeID)
		writeRequestsTotal.WithLabelValues(storeID, "get_writer", "error").Inc()
		return nil, fmt.Errorf("store %s not found", storeID)
	}

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	masterState := entry.healthStates["master"]
	if masterState.circuitState == types.CircuitBreakerOpen {
		logger.Warnf(ctx, "[VectorStoreRouter] Master circuit breaker is OPEN for store %s, write requests may fail", storeID)
	}

	logger.Infof(ctx, "[VectorStoreRouter] GetWriter: storeID=%s, masterCircuitState=%d", storeID, masterState.circuitState)
	writeRequestsTotal.WithLabelValues(storeID, "get_writer", "success").Inc()
	requestLatency.WithLabelValues(storeID, "get_writer", "master").Observe(time.Since(startTime).Seconds())
	return entry.master, nil
}

// RegisterEngine 注册一个存储引擎（使用全局默认配置，master 需为已包装的 RWCapableEngine）
func (r *VectorStoreRouter) RegisterEngine(storeID string, master interfaces.RWCapableEngine, replicas []interfaces.ReadableNode) error {
	return r.registerEngineInternal(storeID, master, nil, r.config, replicas)
}

// RegisterEngineWithConfig 注册一个存储引擎（使用独立配置）
// master 为原始 RetrieveEngineService，内部自动包装为 RWCapableEngine
func (r *VectorStoreRouter) RegisterEngineWithConfig(storeID string, master interfaces.RetrieveEngineService, storeCfg types.ReadWriteSeparationConfig, replicas []interfaces.ReadableNode) error {
	rwMaster := WrapEngineWithRWCapabilities(storeID, master)
	return r.registerEngineInternal(storeID, rwMaster, master, storeCfg, replicas)
}

func (r *VectorStoreRouter) registerEngineInternal(storeID string, rwMaster interfaces.RWCapableEngine, rawMaster interfaces.RetrieveEngineService, storeCfg types.ReadWriteSeparationConfig, replicas []interfaces.ReadableNode) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stores[storeID]; exists {
		logger.Errorf(context.Background(), "[VectorStoreRouter] RegisterEngine failed: store %s already registered", storeID)
		return fmt.Errorf("store %s already registered", storeID)
	}

	storeCfg.WriteEndpoint = storeID

	logger.Infof(context.Background(), "[VectorStoreRouter] Registering engine: storeID=%s, rwEnabled=%v, replicaCount=%d, healthCheckInterval=%v, circuitBreakerThreshold=%d",
		storeID, storeCfg.Enabled, len(replicas), storeCfg.HealthCheckInterval, storeCfg.CircuitBreakerThreshold)

	entry := &storeEntry{
		config:       storeCfg,
		master:       rwMaster,
		replicas:     replicas,
		healthStates: make(map[string]*nodeHealthState),
		lsnTracker:   newLSNTracker(),
		stopCh:       make(chan struct{}),
	}

	// 初始化负载均衡器
	entry.readBalancer = NewRoundRobinBalancer()

	// 初始化主节点健康状态
	entry.healthStates["master"] = &nodeHealthState{
		circuitState: types.CircuitBreakerClosed,
		lastCheckTime: time.Now(),
	}
	for i := range replicas {
		nodeID := fmt.Sprintf("replica-%d", i)
		entry.healthStates[nodeID] = &nodeHealthState{
			circuitState: types.CircuitBreakerClosed,
			lastCheckTime: time.Now(),
		}
		logger.Infof(context.Background(), "[VectorStoreRouter] Registered replica: nodeID=%s, storeID=%s", nodeID, storeID)
	}

	r.stores[storeID] = entry

	// 启动健康检查协程
	if storeCfg.Enabled {
		r.wg.Add(1)
		go r.runHealthCheck(entry)
		logger.Infof(context.Background(), "[VectorStoreRouter] Health check goroutine started for storeID=%s", storeID)
	}

	logger.Infof(context.Background(), "[VectorStoreRouter] Engine registered successfully: storeID=%s", storeID)
	return nil
}

// UnregisterEngine 注销一个存储引擎
func (r *VectorStoreRouter) UnregisterEngine(storeID string) {
	r.mu.Lock()
	entry, ok := r.stores[storeID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.stores, storeID)
	r.mu.Unlock()

	close(entry.stopCh)
	if entry.writeBuffer != nil {
		_ = entry.writeBuffer.Close()
	}
}

// UpdateReplicas 动态更新副本列表（热更新）
func (r *VectorStoreRouter) UpdateReplicas(storeID string, replicas []interfaces.ReadableNode) error {
	r.mu.RLock()
	entry, ok := r.stores[storeID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("store %s not found", storeID)
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.replicas = replicas
	entry.healthStates = make(map[string]*nodeHealthState)
	entry.healthStates["master"] = &nodeHealthState{
		circuitState: types.CircuitBreakerClosed,
		lastCheckTime: time.Now(),
	}
	for i := range replicas {
		nodeID := fmt.Sprintf("replica-%d", i)
		entry.healthStates[nodeID] = &nodeHealthState{
			circuitState: types.CircuitBreakerClosed,
			lastCheckTime: time.Now(),
		}
	}

	return nil
}

// GetRouterStats 获取路由层统计信息
func (r *VectorStoreRouter) GetRouterStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RouterStats{
		TotalStores: len(r.stores),
	}

	for _, entry := range r.stores {
		entry.mu.RLock()
		stats.TotalReplicas += len(entry.replicas)
		for _, state := range entry.healthStates {
			if state.health != nil && state.health.Healthy {
				stats.HealthyNodes++
			} else {
				stats.UnhealthyNodes++
			}
		}
		entry.mu.RUnlock()
	}

	return stats
}

// Shutdown 优雅关闭
func (r *VectorStoreRouter) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 停止所有健康检查
	for _, entry := range r.stores {
		close(entry.stopCh)
		if entry.writeBuffer != nil {
			_ = entry.writeBuffer.FlushAll(ctx)
			_ = entry.writeBuffer.Close()
		}
	}

	// 等待所有协程退出
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runHealthCheck 运行周期性健康检查
func (r *VectorStoreRouter) runHealthCheck(entry *storeEntry) {
	defer r.wg.Done()
	ticker := time.NewTicker(entry.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-entry.stopCh:
			return
		case <-r.globalStop:
			return
		case <-ticker.C:
			r.checkAllNodesHealth(entry)
		}
	}
}

// checkAllNodesHealth 检查所有节点健康状态
func (r *VectorStoreRouter) checkAllNodesHealth(entry *storeEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), entry.config.HealthCheckTimeout)
	defer cancel()

	storeID := entry.config.WriteEndpoint
	startTime := time.Now()
	var healthyMasters, healthyReplicas float64

	// 检查主节点
	masterHealth, err := entry.master.HealthCheck(ctx)
	latency := time.Since(startTime).Seconds()
	entry.mu.Lock()
	masterState := entry.healthStates["master"]
	prevMasterState := masterState.circuitState
	if err != nil || !masterHealth.Healthy {
		masterState.consecutiveFails++
		logger.Warnf(ctx, "[VectorStoreRouter] Master health check failed: storeID=%s, consecutiveFails=%d/%d, err=%v",
			storeID, masterState.consecutiveFails, entry.config.CircuitBreakerThreshold, err)
		if masterState.consecutiveFails >= entry.config.CircuitBreakerThreshold {
			masterState.circuitState = types.CircuitBreakerOpen
			if prevMasterState != types.CircuitBreakerOpen {
				logger.Errorf(ctx, "[VectorStoreRouter] *** CIRCUIT BREAKER OPENED for MASTER *** storeID=%s, consecutiveFails=%d",
					storeID, masterState.consecutiveFails)
			}
		}
		nodeHealthCheckLatency.WithLabelValues(storeID, "master", "error").Observe(latency)
	} else {
		masterState.health = masterHealth
		masterState.consecutiveFails = 0
		masterState.circuitState = types.CircuitBreakerClosed
		if prevMasterState == types.CircuitBreakerOpen {
			logger.Infof(ctx, "[VectorStoreRouter] *** CIRCUIT BREAKER CLOSED for MASTER *** storeID=%s, master is healthy again (LSN=%d, latency=%dms)",
				storeID, masterHealth.LSN, masterHealth.LatencyMs)
		}
		entry.lsnTracker.UpdateMasterLSN(masterHealth.LSN)
		healthyMasters = 1
		nodeHealthCheckLatency.WithLabelValues(storeID, "master", "success").Observe(latency)
	}
	circuitBreakerState.WithLabelValues(storeID, "master").Set(float64(masterState.circuitState))
	masterState.lastCheckTime = time.Now()
	entry.mu.Unlock()
	healthyNodesGauge.WithLabelValues(storeID, "master").Set(healthyMasters)

	// 检查所有副本
	entry.mu.RLock()
	replicas := make([]interfaces.ReadableNode, len(entry.replicas))
	copy(replicas, entry.replicas)
	entry.mu.RUnlock()

	masterLSN := entry.lsnTracker.GetMasterLSN()
	for i, replica := range replicas {
		nodeID := fmt.Sprintf("replica-%d", i)
		replicaStart := time.Now()
		health, err := replica.HealthCheck(ctx)
		replicaLatency := time.Since(replicaStart).Seconds()
		entry.mu.Lock()
		state := entry.healthStates[nodeID]
		prevState := state.circuitState
		if err != nil || !health.Healthy {
			state.consecutiveFails++
			logger.Warnf(ctx, "[VectorStoreRouter] Replica health check failed: nodeID=%s, storeID=%s, consecutiveFails=%d/%d, err=%v",
				nodeID, storeID, state.consecutiveFails, entry.config.CircuitBreakerThreshold, err)
			if state.consecutiveFails >= entry.config.CircuitBreakerThreshold {
				state.circuitState = types.CircuitBreakerOpen
				if prevState != types.CircuitBreakerOpen {
					logger.Errorf(ctx, "[VectorStoreRouter] *** CIRCUIT BREAKER OPENED for replica *** nodeID=%s, storeID=%s",
						nodeID, storeID)
				}
			}
			nodeHealthCheckLatency.WithLabelValues(storeID, nodeID, "error").Observe(replicaLatency)
		} else {
			state.health = health
			state.consecutiveFails = 0
			state.circuitState = types.CircuitBreakerClosed
			lsnLag := masterLSN - health.LSN
			if prevState == types.CircuitBreakerOpen {
				logger.Infof(ctx, "[VectorStoreRouter] *** CIRCUIT BREAKER CLOSED for replica *** nodeID=%s, storeID=%s, lsnLag=%d",
					nodeID, storeID, lsnLag)
			}
			if health.ReplicationLag > entry.config.MaxReplicationLag {
				logger.Warnf(ctx, "[VectorStoreRouter] Replica replication lag exceeds threshold: nodeID=%s, storeID=%s, lag=%v, maxAllowed=%v",
					nodeID, storeID, health.ReplicationLag, entry.config.MaxReplicationLag)
			}
			entry.lsnTracker.UpdateReplicaLSN(nodeID, health.LSN)
			healthyReplicas++
			replicaLSNLag.WithLabelValues(storeID, nodeID).Set(float64(lsnLag))
			nodeHealthCheckLatency.WithLabelValues(storeID, nodeID, "success").Observe(replicaLatency)
		}
		circuitBreakerState.WithLabelValues(storeID, nodeID).Set(float64(state.circuitState))
		state.lastCheckTime = time.Now()
		entry.mu.Unlock()
	}
	healthyNodesGauge.WithLabelValues(storeID, "replica").Set(healthyReplicas)
}

// getHealthyReplicas 获取所有健康的副本节点
func (r *VectorStoreRouter) getHealthyReplicas(entry *storeEntry) map[string]interfaces.ReadableNode {
	healthy := make(map[string]interfaces.ReadableNode)
	entry.mu.RLock()
	defer entry.mu.RUnlock()

	for i, replica := range entry.replicas {
		nodeID := fmt.Sprintf("replica-%d", i)
		state := entry.healthStates[nodeID]
		if state.circuitState == types.CircuitBreakerClosed && state.health != nil && state.health.Healthy {
			// 检查复制延迟是否在可接受范围内
			if state.health.ReplicationLag <= entry.config.MaxReplicationLag {
				healthy[nodeID] = replica
			}
		}
	}
	return healthy
}

// selectReaderForLSN 选择满足LSN要求的副本节点
func (r *VectorStoreRouter) selectReaderForLSN(ctx context.Context, entry *storeEntry, requiredLSN int64) (interfaces.ReadableNode, error) {
	// 先找已追平的健康副本
	healthy := r.getHealthyReplicas(entry)
	for nodeID, replica := range healthy {
		state := entry.healthStates[nodeID]
		if state.health != nil && state.health.LSN >= requiredLSN {
			return replica, nil
		}
	}

	// 等待副本追平，超时则返回错误
	timeout := entry.config.WaitForLSNTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := entry.lsnTracker.WaitForLSN(requiredLSN)
	select {
	case nodeID := <-ch:
		entry.mu.RLock()
		defer entry.mu.RUnlock()
		// 找到对应的replica
		for i, replica := range entry.replicas {
			if fmt.Sprintf("replica-%d", i) == nodeID {
				return replica, nil
			}
		}
		return entry.master, nil
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
}

// lsnTracker LSN水位追踪器，用于会话一致性
type lsnTracker struct {
	mu           sync.RWMutex
	masterLSN    int64
	replicaLSNs  map[string]int64
	waiters      map[int64][]chan string
}

func newLSNTracker() *lsnTracker {
	return &lsnTracker{
		replicaLSNs: make(map[string]int64),
		waiters:     make(map[int64][]chan string),
	}
}

func (t *lsnTracker) UpdateMasterLSN(lsn int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if lsn > t.masterLSN {
		t.masterLSN = lsn
	}
}

func (t *lsnTracker) GetMasterLSN() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.masterLSN
}

func (t *lsnTracker) UpdateReplicaLSN(nodeID string, lsn int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	oldLSN := t.replicaLSNs[nodeID]
	t.replicaLSNs[nodeID] = lsn

	// 通知等待的waiter
	if lsn > oldLSN {
		for requiredLSN, chs := range t.waiters {
			if lsn >= requiredLSN {
				for _, ch := range chs {
					select {
					case ch <- nodeID:
					default:
					}
				}
				delete(t.waiters, requiredLSN)
			}
		}
	}
}

func (t *lsnTracker) WaitForLSN(lsn int64) <-chan string {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan string, 1)

	// 检查现在是否有副本已经满足
	for nodeID, replicaLSN := range t.replicaLSNs {
		if replicaLSN >= lsn {
			ch <- nodeID
			return ch
		}
	}

	// 加入等待队列
	t.waiters[lsn] = append(t.waiters[lsn], ch)
	return ch
}
