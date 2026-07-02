package service

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// LoadBalancer 读副本负载均衡器接口
type LoadBalancer interface {
	// SelectNode 从健康节点池中选择一个节点
	SelectNode(healthyNodes map[string]interfaces.ReadableNode) interfaces.ReadableNode
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	counter atomic.Uint64
	mu      sync.Mutex
	nodeIDs []string
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

func (b *RoundRobinBalancer) SelectNode(healthyNodes map[string]interfaces.ReadableNode) interfaces.ReadableNode {
	if len(healthyNodes) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 构建节点ID列表，排序保证顺序一致性（map迭代顺序不确定）
	b.nodeIDs = b.nodeIDs[:0]
	for nodeID := range healthyNodes {
		b.nodeIDs = append(b.nodeIDs, nodeID)
	}
	sort.Strings(b.nodeIDs)

	// 轮询选择
	idx := b.counter.Add(1) % uint64(len(b.nodeIDs))
	selectedID := b.nodeIDs[idx]
	return healthyNodes[selectedID]
}

// LeastConnectionsBalancer 最小连接数负载均衡器（预留实现）
type LeastConnectionsBalancer struct {
	mu            sync.Mutex
	connectionMap map[string]int64
}

// NewLeastConnectionsBalancer 创建最小连接数负载均衡器
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		connectionMap: make(map[string]int64),
	}
}

func (b *LeastConnectionsBalancer) SelectNode(healthyNodes map[string]interfaces.ReadableNode) interfaces.ReadableNode {
	if len(healthyNodes) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 惰性清理：移除不在健康节点池中的条目，防止 connectionMap 无界增长
	for nodeID := range b.connectionMap {
		if _, ok := healthyNodes[nodeID]; !ok {
			delete(b.connectionMap, nodeID)
		}
	}

	var selectedNode interfaces.ReadableNode
	var minConns int64 = -1
	var selectedID string

	for nodeID, node := range healthyNodes {
		conns := b.connectionMap[nodeID]
		if minConns == -1 || conns < minConns {
			minConns = conns
			selectedNode = node
			selectedID = nodeID
		}
	}

	if selectedNode != nil {
		b.connectionMap[selectedID]++
	}
	return selectedNode
}

// Compile-time interface checks
var _ LoadBalancer = (*RoundRobinBalancer)(nil)
var _ LoadBalancer = (*LeastConnectionsBalancer)(nil)
