package types

import (
	"context"
	"time"
)

// ConsistencyLevel 一致性级别
type ConsistencyLevel int

const (
	// ConsistencyLevelEventual 默认：最终一致，路由到任意可用副本，最低延迟
	ConsistencyLevelEventual ConsistencyLevel = iota
	// ConsistencyLevelSession 会话一致：保证能读到指定WriteToken之后的写入
	ConsistencyLevelSession
	// ConsistencyLevelStrong 强一致：直接读主节点，保证读到最新数据（性能较差）
	ConsistencyLevelStrong
)

func (c ConsistencyLevel) String() string {
	switch c {
	case ConsistencyLevelEventual:
		return "eventual"
	case ConsistencyLevelSession:
		return "session"
	case ConsistencyLevelStrong:
		return "strong"
	default:
		return "unknown"
	}
}

// WriteToken 写入令牌，用于会话一致读
type WriteToken struct {
	StoreID   string `json:"store_id"`
	LSN       int64  `json:"lsn"`        // 日志序列号，单调递增
	Timestamp int64  `json:"timestamp"`  // 写入时间戳（Unix毫秒）
	TenantID  uint64 `json:"tenant_id"`
}

// NodeHealth 节点健康状态
type NodeHealth struct {
	NodeID         string        `json:"node_id"`
	Endpoint       string        `json:"endpoint"`
	IsMaster       bool          `json:"is_master"`
	Healthy        bool          `json:"healthy"`
	LatencyMs      int64         `json:"latency_ms"`
	LSN            int64         `json:"lsn"`             // 当前节点最新LSN
	ReplicationLag time.Duration `json:"replication_lag"` // 复制延迟
	Connections    int           `json:"connections"`     // 当前连接数
	LastChecked    time.Time     `json:"last_checked"`
}

// LoadBalanceStrategy 负载均衡策略类型
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin       LoadBalanceStrategy = "round_robin"        // 轮询
	LoadBalanceLeastConnections LoadBalanceStrategy = "least_connections"  // 最小连接数
	LoadBalanceLatencyWeighted  LoadBalanceStrategy = "latency_weighted"   // 延迟加权
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	CircuitBreakerClosed   CircuitBreakerState = iota // 关闭（正常）
	CircuitBreakerOpen                                // 打开（熔断）
	CircuitBreakerHalfOpen                            // 半开（探测中）
)

// WriteTaskStatus 写入任务状态
type WriteTaskStatus string

const (
	WriteTaskPending    WriteTaskStatus = "pending"    // 排队中
	WriteTaskProcessing WriteTaskStatus = "processing" // 写入中
	WriteTaskCompleted  WriteTaskStatus = "completed"  // 写入完成
	WriteTaskFailed     WriteTaskStatus = "failed"     // 写入失败
)

// ReadWriteSeparationConfig 读写分离配置
type ReadWriteSeparationConfig struct {
	// 开关（关闭则所有请求走主节点，行为与升级前一致）
	Enabled bool `json:"enabled" default:"false"`

	// 主节点端点（写端点）
	WriteEndpoint string `json:"write_endpoint"`

	// 读副本端点列表
	ReadEndpoints []string `json:"read_endpoints"`

	// 负载均衡策略
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy" default:"round_robin"`

	// 健康检查配置
	HealthCheckInterval time.Duration `json:"health_check_interval" default:"5s"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout" default:"2s"`

	// 熔断配置
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold" default:"5"`  // 连续失败N次触发熔断
	CircuitBreakerResetWait time.Duration `json:"circuit_breaker_reset_wait" default:"30s"` // 熔断后半开探测等待时间

	// 一致性配置
	MaxReplicationLag  time.Duration    `json:"max_replication_lag" default:"1s"`      // 超过该延迟的副本不参与路由
	WaitForLSNTimeout  time.Duration    `json:"wait_for_lsn_timeout" default:"500ms"`  // Session级别一致性等待副本追平超时
	DefaultConsistency ConsistencyLevel `json:"default_consistency" default:"eventual"`

	// 写入缓冲配置
	WriteBufferSizePerKB int           `json:"write_buffer_size_per_kb" default:"1000"` // 每个KB缓冲区大小（条目数）
	WriteMaxBatchSize    int           `json:"write_max_batch_size" default:"1000"`      // 最大写入批次
	WriteMaxWaitTime     time.Duration `json:"write_max_wait_time" default:"100ms"`       // 最大攒批等待时间
	WriteConcurrency     int           `json:"write_concurrency" default:"4"`              // 并发写入协程数

	// 连接池配置
	PoolMaxOpenConns    int           `json:"pool_max_open_conns" default:"10"`
	PoolMaxIdleConns    int           `json:"pool_max_idle_conns" default:"5"`
	PoolConnMaxLifetime time.Duration `json:"pool_conn_max_lifetime" default:"10m"`
}

// DefaultReadWriteSeparationConfig 返回默认配置（关闭读写分离）
func DefaultReadWriteSeparationConfig() ReadWriteSeparationConfig {
	return ReadWriteSeparationConfig{
		Enabled:                 false,
		LoadBalanceStrategy:     LoadBalanceRoundRobin,
		HealthCheckInterval:     5 * time.Second,
		HealthCheckTimeout:      2 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerResetWait: 30 * time.Second,
		MaxReplicationLag:       1 * time.Second,
		WaitForLSNTimeout:       500 * time.Millisecond,
		DefaultConsistency:      ConsistencyLevelEventual,
		WriteBufferSizePerKB:    1000,
		WriteMaxBatchSize:       1000,
		WriteMaxWaitTime:        100 * time.Millisecond,
		WriteConcurrency:        4,
		PoolMaxOpenConns:        10,
		PoolMaxIdleConns:        5,
		PoolConnMaxLifetime:     10 * time.Minute,
	}
}

// NodeHealthChecker 节点健康检查接口
type NodeHealthChecker interface {
	CheckHealth(ctx context.Context, endpoint string) (*NodeHealth, error)
}
