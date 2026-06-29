package service

import (
	"testing"

	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestRoundRobinBalancer_SelectNode_Empty(t *testing.T) {
	b := NewRoundRobinBalancer()
	result := b.SelectNode(map[string]interfaces.ReadableNode{})
	assert.Nil(t, result)
}

func TestRoundRobinBalancer_SelectNode_Single(t *testing.T) {
	b := NewRoundRobinBalancer()
	engine := newMockEngine()
	nodes := map[string]interfaces.ReadableNode{"replica-0": engine}

	result := b.SelectNode(nodes)
	assert.NotNil(t, result)
}

func TestRoundRobinBalancer_SelectNode_RoundRobin(t *testing.T) {
	b := NewRoundRobinBalancer()
	engine1 := newMockEngine()
	engine2 := newMockEngine()
	engine3 := newMockEngine()
	nodes := map[string]interfaces.ReadableNode{
		"replica-0": engine1,
		"replica-1": engine2,
		"replica-2": engine3,
	}

	// 轮询应该分配到不同节点（map 顺序随机，只验证所有节点都被选中）
	seen := make(map[interfaces.ReadableNode]int)
	for i := 0; i < 30; i++ {
		selected := b.SelectNode(nodes)
		assert.NotNil(t, selected)
		seen[selected]++
	}

	// 3个节点都应被选中过
	assert.Len(t, seen, 3, "all 3 nodes should be selected")
	for node, count := range seen {
		assert.Greater(t, count, 0, "node %v should be selected at least once", node)
	}
}

func TestLeastConnectionsBalancer_SelectNode_Empty(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	result := b.SelectNode(map[string]interfaces.ReadableNode{})
	assert.Nil(t, result)
}

func TestLeastConnectionsBalancer_SelectNode_PicksLeastConnected(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	engine1 := newMockEngine()
	engine2 := newMockEngine()
	nodes := map[string]interfaces.ReadableNode{
		"replica-0": engine1,
		"replica-1": engine2,
	}

	// 多轮选择后，两个节点都应被选中（least connections 会均衡分配）
	seen := make(map[interfaces.ReadableNode]int)
	for i := 0; i < 10; i++ {
		selected := b.SelectNode(nodes)
		assert.NotNil(t, selected)
		seen[selected]++
	}

	// 两个节点都应被选中过
	assert.Len(t, seen, 2, "both nodes should be selected")
	for _, count := range seen {
		assert.Greater(t, count, 0, "each node should be selected at least once")
	}
}
