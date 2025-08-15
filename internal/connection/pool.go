package connection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gather/internal/config"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
)

// ConnectionPool 以太坊连接池
type ConnectionPool struct {
	nodes       []*config.NodeConfig
	pools       map[string]*NodePool
	logger      *logrus.Logger
	mu          sync.RWMutex
	healthCheck time.Duration
	maxRetries  int
}

// NodePool 单个节点的连接池
type NodePool struct {
	nodeConfig *config.NodeConfig
	clients    chan *ethclient.Client
	maxSize    int
	current    int
	logger     *logrus.Logger
	mu         sync.Mutex
	isHealthy  bool
	lastCheck  time.Time
}

// NewConnectionPool 创建连接池
func NewConnectionPool(nodes []*config.NodeConfig, logger *logrus.Logger) *ConnectionPool {
	return &ConnectionPool{
		nodes:       nodes,
		pools:       make(map[string]*NodePool),
		logger:      logger,
		healthCheck: 30 * time.Second,
		maxRetries:  3,
	}
}

// Initialize 初始化连接池
func (cp *ConnectionPool) Initialize() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, node := range cp.nodes {
		pool, err := NewNodePool(node, 10, cp.logger) // 每个节点最多10个连接
		if err != nil {
			cp.logger.Warnf("初始化节点 %s 连接池失败: %v", node.Name, err)
			continue
		}
		
		cp.pools[node.Name] = pool
		cp.logger.Infof("节点 %s 连接池已初始化", node.Name)
	}

	if len(cp.pools) == 0 {
		return fmt.Errorf("没有可用的节点连接池")
	}

	// 启动健康检查
	go cp.healthChecker()

	return nil
}

// NewNodePool 创建节点连接池
func NewNodePool(nodeConfig *config.NodeConfig, maxSize int, logger *logrus.Logger) (*NodePool, error) {
	pool := &NodePool{
		nodeConfig: nodeConfig,
		clients:    make(chan *ethclient.Client, maxSize),
		maxSize:    maxSize,
		logger:     logger,
		isHealthy:  true,
	}

	// 预创建一些连接
	initialSize := maxSize / 2
	if initialSize < 1 {
		initialSize = 1
	}

	for i := 0; i < initialSize; i++ {
		client, err := pool.createClient()
		if err != nil {
			logger.Warnf("预创建连接失败: %v", err)
			break
		}
		
		select {
		case pool.clients <- client:
			pool.current++
		default:
			client.Close()
			break
		}
	}

	return pool, nil
}

// createClient 创建新的以太坊客户端
func (np *NodePool) createClient() (*ethclient.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, np.nodeConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("连接节点失败: %w", err)
	}

	// 测试连接
	_, err = client.ChainID(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("测试连接失败: %w", err)
	}

	return client, nil
}

// GetClient 获取客户端连接
func (cp *ConnectionPool) GetClient() (*ethclient.Client, string, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// 按优先级排序，选择可用的节点
	var availablePools []*NodePool
	var availableNames []string
	
	for name, pool := range cp.pools {
		if pool.IsHealthy() {
			availablePools = append(availablePools, pool)
			availableNames = append(availableNames, name)
		}
	}

	if len(availablePools) == 0 {
		return nil, "", fmt.Errorf("没有可用的健康节点")
	}

	// 轮询选择节点（简单的负载均衡）
	for i, pool := range availablePools {
		client, err := pool.GetClient()
		if err != nil {
			cp.logger.Debugf("从节点 %s 获取连接失败: %v", availableNames[i], err)
			continue
		}
		return client, availableNames[i], nil
	}

	return nil, "", fmt.Errorf("所有节点都无法提供连接")
}

// GetClient 从节点池获取客户端
func (np *NodePool) GetClient() (*ethclient.Client, error) {
	// 首先尝试从池中获取现有连接
	select {
	case client := <-np.clients:
		// 检查连接是否仍然有效
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		_, err := client.ChainID(ctx)
		if err != nil {
			client.Close()
			np.mu.Lock()
			np.current--
			np.mu.Unlock()
			// 连接无效，尝试创建新连接
			return np.createNewClient()
		}
		
		return client, nil
	default:
		// 池中没有可用连接，创建新连接
		return np.createNewClient()
	}
}

// createNewClient 创建新客户端连接
func (np *NodePool) createNewClient() (*ethclient.Client, error) {
	np.mu.Lock()
	defer np.mu.Unlock()

	// 检查是否达到最大连接数
	if np.current >= np.maxSize {
		return nil, fmt.Errorf("连接池已满")
	}

	client, err := np.createClient()
	if err != nil {
		np.isHealthy = false
		return nil, err
	}

	np.current++
	return client, nil
}

// ReturnClient 归还客户端到池中
func (cp *ConnectionPool) ReturnClient(client *ethclient.Client, nodeName string) {
	if client == nil {
		return
	}

	cp.mu.RLock()
	pool, exists := cp.pools[nodeName]
	cp.mu.RUnlock()

	if !exists {
		client.Close()
		return
	}

	pool.ReturnClient(client)
}

// ReturnClient 归还客户端到节点池
func (np *NodePool) ReturnClient(client *ethclient.Client) {
	if client == nil {
		return
	}

	select {
	case np.clients <- client:
		// 成功归还到池中
	default:
		// 池已满，关闭连接
		client.Close()
		np.mu.Lock()
		np.current--
		np.mu.Unlock()
	}
}

// IsHealthy 检查节点是否健康
func (np *NodePool) IsHealthy() bool {
	np.mu.Lock()
	defer np.mu.Unlock()
	
	// 如果最近检查过且是健康的，直接返回
	if time.Since(np.lastCheck) < 30*time.Second && np.isHealthy {
		return np.isHealthy
	}

	// 执行健康检查
	client, err := np.createClient()
	if err != nil {
		np.isHealthy = false
		np.lastCheck = time.Now()
		return false
	}

	// 测试基本功能
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.ChainID(ctx)
	client.Close()

	np.isHealthy = (err == nil)
	np.lastCheck = time.Now()

	return np.isHealthy
}

// healthChecker 健康检查器
func (cp *ConnectionPool) healthChecker() {
	ticker := time.NewTicker(cp.healthCheck)
	defer ticker.Stop()

	for range ticker.C {
		cp.mu.RLock()
		pools := make(map[string]*NodePool)
		for name, pool := range cp.pools {
			pools[name] = pool
		}
		cp.mu.RUnlock()

		for name, pool := range pools {
			isHealthy := pool.IsHealthy()
			if isHealthy {
				cp.logger.Debugf("节点 %s 健康检查通过", name)
			} else {
				cp.logger.Warnf("节点 %s 健康检查失败", name)
			}
		}
	}
}

// GetStats 获取连接池统计信息
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := make(map[string]interface{})
	
	for name, pool := range cp.pools {
		poolStats := map[string]interface{}{
			"max_size":     pool.maxSize,
			"current_size": pool.current,
			"available":    len(pool.clients),
			"is_healthy":   pool.IsHealthy(),
			"last_check":   pool.lastCheck.Format(time.RFC3339),
		}
		stats[name] = poolStats
	}

	return stats
}

// Close 关闭连接池
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var errors []error
	
	for name, pool := range cp.pools {
		if err := pool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭节点 %s 连接池失败: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("关闭连接池时发生错误: %v", errors)
	}

	cp.logger.Info("连接池已关闭")
	return nil
}

// Close 关闭节点连接池
func (np *NodePool) Close() error {
	np.mu.Lock()
	defer np.mu.Unlock()

	// 关闭所有连接
	close(np.clients)
	for client := range np.clients {
		client.Close()
	}

	np.current = 0
	return nil
}

// ConnectionWrapper 连接包装器，自动管理连接的获取和归还
type ConnectionWrapper struct {
	client   *ethclient.Client
	nodeName string
	pool     *ConnectionPool
}

// NewConnectionWrapper 创建连接包装器
func (cp *ConnectionPool) NewConnectionWrapper() (*ConnectionWrapper, error) {
	client, nodeName, err := cp.GetClient()
	if err != nil {
		return nil, err
	}

	return &ConnectionWrapper{
		client:   client,
		nodeName: nodeName,
		pool:     cp,
	}, nil
}

// Client 获取以太坊客户端
func (cw *ConnectionWrapper) Client() *ethclient.Client {
	return cw.client
}

// NodeName 获取节点名称
func (cw *ConnectionWrapper) NodeName() string {
	return cw.nodeName
}

// Close 关闭连接包装器，自动归还连接
func (cw *ConnectionWrapper) Close() {
	if cw.client != nil {
		cw.pool.ReturnClient(cw.client, cw.nodeName)
		cw.client = nil
	}
}