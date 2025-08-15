package collector

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"

	"gather/internal/config"
	"gather/internal/decoder"
	"gather/internal/logging"
	"gather/internal/output"
	"gather/internal/progress"
	"gather/internal/retry"
	"gather/internal/shutdown"
	"gather/pkg/models"
)

// BlockResult 区块结果
type BlockResult struct {
	BlockNumber       uint64
	Block             *models.Block
	Transactions      []*models.Transaction
	Logs              []*models.TransactionLog
	InternalTxs       []*models.InternalTransaction
	StateChanges      []*models.StateChange
	ContractCreations []*models.ContractCreation
	Withdrawals       []*models.Withdrawal
	Error             error
}

// BatchResult 批量采集结果
type BatchResult struct {
	StartBlock            uint64
	EndBlock              uint64
	ProcessedBlocks       uint64
	TotalBlocks           uint64
	TotalTransactions     uint64
	TotalLogs             uint64
	StartTime             time.Time
	EndTime               time.Time
	Duration              time.Duration
	BlocksPerSecond       float64
	TransactionsPerSecond float64
	Errors                []error
}

// 采集器常量
const (
	// 网络和超时设置
	DefaultStreamPollInterval = 15 * time.Second // 流式采集轮询间隔
	DefaultWorkerCount        = 10               // 默认工作协程数
	DefaultBatchSize          = 100              // 默认批处理大小
	DefaultRetryLimit         = 3                // 默认重试次数

	// 数据处理设置
	MaxBlocksPerBatch    = 1000 // 每批最大区块数
	MaxConcurrentWorkers = 50   // 最大并发工作协程数

	// 调试和日志
	DebugLogLevel = logrus.DebugLevel // 调试日志级别
)

// 自定义错误类型
type CollectorError struct {
	Type    string
	Message string
	Err     error
}

func (e *CollectorError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func (e *CollectorError) Unwrap() error {
	return e.Err
}

// 错误类型常量
const (
	ErrTypeConnection = "CONNECTION"
	ErrTypeValidation = "VALIDATION"
	ErrTypeTimeout    = "TIMEOUT"
	ErrTypeData       = "DATA"
	ErrTypeNetwork    = "NETWORK"
	ErrTypeRateLimit  = "RATE_LIMIT" // 429错误
)

// 便利的错误创建函数
func NewConnectionError(msg string, err error) *CollectorError {
	return &CollectorError{Type: ErrTypeConnection, Message: msg, Err: err}
}

func NewValidationError(msg string) *CollectorError {
	return &CollectorError{Type: ErrTypeValidation, Message: msg}
}

func NewTimeoutError(msg string, err error) *CollectorError {
	return &CollectorError{Type: ErrTypeTimeout, Message: msg, Err: err}
}

func NewDataError(msg string, err error) *CollectorError {
	return &CollectorError{Type: ErrTypeData, Message: msg, Err: err}
}

func NewRateLimitError(nodeName string, err error) *CollectorError {
	return &CollectorError{Type: ErrTypeRateLimit, Message: fmt.Sprintf("节点 %s 达到速率限制", nodeName), Err: err}
}

// isRateLimitError 检测是否为429错误
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 检查常见的429错误模式
	return containsAny(errStr, []string{
		"429", "Too Many Requests", "rate limit", "Rate limit",
		"quota exceeded", "request limit", "requests per second",
		"API rate limit exceeded", "exceed rate limit",
	})
}

// containsAny 检查字符串是否包含任意一个子字符串
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// NodeClient 节点客户端
type NodeClient struct {
	Name         string
	URL          string
	Type         string
	RateLimit    int
	Priority     int
	Client       *ethclient.Client
	Available    bool
	LastUsed     time.Time
	RateLimited  bool      // 是否被速率限制
	RateLimitEnd time.Time // 速率限制结束时间
	ErrorCount   int       // 错误计数
	mu           sync.RWMutex
}

// Collector 数据采集器
type Collector struct {
	nodes            []*NodeClient
	blockchainConfig *config.BlockchainConfig
	collectorConfig  *config.CollectorConfig
	outputter        output.Output
	logger           *logrus.Logger
	mu               sync.RWMutex
	lastSynced       uint64
	currentNodeIndex int
	blockHashCache   map[uint64]string          // 缓存区块哈希用于重组检测
	inputDecoder     *decoder.InputDecoder      // 输入数据解码器
	progressManager  *progress.Manager          // 进度管理器
	retrier          *retry.Retrier             // 重试器
	gracefulShutdown *shutdown.GracefulShutdown // 优雅停机管理器
	structuredLogger *logging.StructuredLogger  // 结构化日志器
}

// validateConfig 验证配置参数
func validateConfig(cfg *config.BlockchainConfig, out output.Output, logger *logrus.Logger) error {
	if cfg == nil {
		return fmt.Errorf("区块链配置不能为空")
	}
	if out == nil {
		return fmt.Errorf("输出器不能为空")
	}
	if logger == nil {
		return fmt.Errorf("日志器不能为空")
	}
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("至少需要配置一个区块链节点")
	}

	// 验证节点配置
	for i, node := range cfg.Nodes {
		if node.Name == "" {
			return fmt.Errorf("节点 %d 的名称不能为空", i)
		}
		if node.URL == "" {
			return fmt.Errorf("节点 %s 的URL不能为空", node.Name)
		}
	}

	return nil
}

// NewCollector 创建新的采集器
func NewCollector(cfg *config.BlockchainConfig, out output.Output, logger *logrus.Logger) *Collector {
	return NewCollectorWithLogging(cfg, out, logger, nil)
}

// NewCollectorWithLogging 创建带结构化日志的采集器
func NewCollectorWithLogging(cfg *config.BlockchainConfig, out output.Output, logger *logrus.Logger, logConfig *logging.LogConfig) *Collector {
	// 验证输入参数
	if err := validateConfig(cfg, out, logger); err != nil {
		logger.Fatalf("配置验证失败: %v", NewValidationError(err.Error()))
	}
	// 初始化所有节点客户端
	var nodes []*NodeClient

	for _, nodeConfig := range cfg.Nodes {
		client, err := ethclient.Dial(nodeConfig.URL)
		if err != nil {
			connErr := NewConnectionError(fmt.Sprintf("连接节点失败 %s", nodeConfig.Name), err)
			logger.Warnf("%v", connErr)
			continue
		}

		// 测试节点连接
		_, err = client.BlockNumber(context.Background())
		if err != nil {
			networkErr := NewConnectionError(fmt.Sprintf("节点 %s 不可用", nodeConfig.Name), err)
			logger.Warnf("%v", networkErr)
			client.Close()
			continue
		}

		node := &NodeClient{
			Name:      nodeConfig.Name,
			URL:       nodeConfig.URL,
			Type:      nodeConfig.Type,
			RateLimit: nodeConfig.RateLimit,
			Priority:  nodeConfig.Priority,
			Client:    client,
			Available: true,
			LastUsed:  time.Now(),
		}

		nodes = append(nodes, node)
		logger.Infof("成功连接到节点: %s", nodeConfig.Name)
	}

	if len(nodes) == 0 {
		logger.Fatal("无法连接到任何区块链节点")
	}

	// 按优先级排序节点（优先级数字越小越优先）
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Priority < nodes[j].Priority
	})

	// 初始化进度管理器
	progressManager, err := progress.NewManager("", logger)
	if err != nil {
		logger.Warnf("初始化进度管理器失败: %v，将不支持断点续传", err)
	}

	// 初始化结构化日志器
	var structuredLogger *logging.StructuredLogger
	if logConfig != nil {
		var err error
		structuredLogger, err = logging.NewStructuredLogger(logConfig)
		if err != nil {
			logger.Warnf("初始化结构化日志器失败: %v，将使用默认日志", err)
		}
	}

	// 初始化优雅停机管理器
	gracefulShutdown := shutdown.NewGracefulShutdown(30*time.Second, logger)

	collector := &Collector{
		nodes:            nodes,
		blockchainConfig: cfg,
		collectorConfig:  nil, // 暂时设为nil，稍后更新
		outputter:        out,
		logger:           logger,
		currentNodeIndex: 0,
		blockHashCache:   make(map[uint64]string),
		inputDecoder:     decoder.NewInputDecoder(logger, &config.DecoderConfig{
			FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
			APITimeout:     "5s",
			EnableCache:    true,
			CacheSize:      10000,
			EnableAPI:      true,
		}),
		progressManager:  progressManager,
		retrier:          retry.NewRetrier(retry.NetworkRetryConfig, logger),
		gracefulShutdown: gracefulShutdown,
		structuredLogger: structuredLogger,
	}

	// 注册停机处理函数
	collector.registerShutdownHandlers()

	return collector
}

// getNextAvailableNode 获取下一个可用节点
func (c *Collector) getNextAvailableNode() *NodeClient {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// 尝试从当前索引开始查找可用节点
	for i := 0; i < len(c.nodes); i++ {
		index := (c.currentNodeIndex + i) % len(c.nodes)
		node := c.nodes[index]

		node.mu.RLock()
		available := node.Available
		rateLimited := node.RateLimited
		rateLimitEnd := node.RateLimitEnd
		node.mu.RUnlock()

		// 检查速率限制是否已过期
		if rateLimited && now.After(rateLimitEnd) {
			node.mu.Lock()
			node.RateLimited = false
			node.ErrorCount = 0
			c.logger.Infof("节点 %s 速率限制已解除", node.Name)
			node.mu.Unlock()
			rateLimited = false
		}

		if available && !rateLimited {
			c.currentNodeIndex = index
			return node
		}
	}

	// 如果没有可用节点，检查是否所有节点都被速率限制
	allRateLimited := true
	for _, node := range c.nodes {
		node.mu.RLock()
		if !node.RateLimited || now.After(node.RateLimitEnd) {
			allRateLimited = false
		}
		node.mu.RUnlock()
		if !allRateLimited {
			break
		}
	}

	if allRateLimited {
		c.logger.Warn("所有节点都被速率限制，等待限制解除...")
		return nil
	}

	// 如果没有可用节点，重置所有节点状态
	c.logger.Warn("所有节点都不可用，尝试重新连接...")
	for _, node := range c.nodes {
		node.mu.Lock()
		if !node.RateLimited {
			node.Available = true
		}
		node.mu.Unlock()
	}

	if len(c.nodes) > 0 {
		return c.nodes[0]
	}

	return nil
}

// SetCollectorConfig 设置采集器配置
func (c *Collector) SetCollectorConfig(cfg *config.CollectorConfig) {
	c.collectorConfig = cfg
}

// markNodeRateLimited 标记节点为速率限制状态
func (c *Collector) markNodeRateLimited(nodeName string, err error) {
	for _, node := range c.nodes {
		if node.Name == nodeName {
			node.mu.Lock()
			node.RateLimited = true
			node.RateLimitEnd = time.Now().Add(5 * time.Minute) // 5分钟后重试
			node.ErrorCount++
			node.mu.Unlock()

			rateLimitErr := NewRateLimitError(nodeName, err)
			c.logger.Errorf("🚫 %v - 将在5分钟后重试", rateLimitErr)
			break
		}
	}
}

// handleNodeError 处理节点错误
func (c *Collector) handleNodeError(nodeName string, err error) {
	if isRateLimitError(err) {
		c.markNodeRateLimited(nodeName, err)
	} else {
		// 其他错误，增加错误计数
		for _, node := range c.nodes {
			if node.Name == nodeName {
				node.mu.Lock()
				node.ErrorCount++
				if node.ErrorCount >= 3 {
					node.Available = false
					c.logger.Warnf("节点 %s 错误次数过多，暂时禁用", nodeName)
				}
				node.mu.Unlock()
				break
			}
		}
	}
}

// getClient 获取可用的客户端
func (c *Collector) getClient() *ethclient.Client {
	node := c.getNextAvailableNode()
	if node == nil {
		return nil
	}
	return node.Client
}

// GetClient 获取可用的客户端（公开方法）
func (c *Collector) GetClient() *ethclient.Client {
	return c.getClient()
}

// CollectBlock 采集单个区块（公开方法）
func (c *Collector) CollectBlock(ctx context.Context, blockNumber uint64) (*BlockResult, error) {
	return c.collectBlock(ctx, blockNumber)
}

// getClientWithNodeName 获取可用的客户端和节点名称
func (c *Collector) getClientWithNodeName() (*ethclient.Client, string) {
	node := c.getNextAvailableNode()
	if node == nil {
		return nil, ""
	}
	return node.Client, node.Name
}

// validateBatchParams 验证批量采集参数
func validateBatchParams(startBlock, endBlock uint64, workers, batchSize int) error {
	if startBlock > endBlock {
		return fmt.Errorf("起始区块号(%d)不能大于结束区块号(%d)", startBlock, endBlock)
	}
	if workers <= 0 || workers > MaxConcurrentWorkers {
		return fmt.Errorf("工作协程数必须在1-%d之间，当前值: %d", MaxConcurrentWorkers, workers)
	}
	if batchSize <= 0 || batchSize > MaxBlocksPerBatch {
		return fmt.Errorf("批处理大小必须在1-%d之间，当前值: %d", MaxBlocksPerBatch, batchSize)
	}
	if endBlock-startBlock+1 > 1000000 { // 防止过大的范围
		return fmt.Errorf("区块范围过大，最大支持100万个区块")
	}
	return nil
}

// CollectBatch 批量采集（支持断点续传）
func (c *Collector) CollectBatch(ctx context.Context, startBlock, endBlock uint64, workers, batchSize int) (*BatchResult, error) {
	// 验证参数
	if err := validateBatchParams(startBlock, endBlock, workers, batchSize); err != nil {
		return nil, NewValidationError(err.Error())
	}

	// 检查断点续传
	actualStartBlock := c.getResumeBlock(startBlock)
	if actualStartBlock != startBlock {
		c.logger.Infof("检测到断点续传，从区块 %d 开始（原计划 %d）", actualStartBlock, startBlock)
		startBlock = actualStartBlock
	}

	c.logger.Infof("开始并发批量采集区块 %d - %d，使用 %d 个工作者", startBlock, endBlock, workers)

	result := &BatchResult{
		StartBlock: startBlock,
		EndBlock:   endBlock,
		StartTime:  time.Now(),
	}

	// 创建工作池
	taskChan := make(chan uint64, workers*2)
	resultChan := make(chan *BlockResult, workers*2)
	errorChan := make(chan error, workers)

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go c.worker(ctx, taskChan, resultChan, errorChan, &wg)
	}

	// 发送任务
	go func() {
		defer close(taskChan)
		for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
			select {
			case taskChan <- blockNum:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// 处理结果
	for {
		select {
		case blockResult, ok := <-resultChan:
			if !ok {
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(result.StartTime)
				result.BlocksPerSecond = float64(result.ProcessedBlocks) / result.Duration.Seconds()
				result.TransactionsPerSecond = float64(result.TotalTransactions) / result.Duration.Seconds()
				return result, nil
			}

			// 处理并输出数据
			if err := c.processAndOutput(blockResult); err != nil {
				c.logger.Errorf("处理区块 %d 失败: %v", blockResult.BlockNumber, err)
			} else {
				// 更新进度
				transactionCount := 0
				if blockResult.Transactions != nil {
					transactionCount = len(blockResult.Transactions)
				}
				c.updateProcessingProgress(blockResult.BlockNumber, transactionCount)
			}

			if blockResult.Block != nil {
				result.TotalBlocks++
			}
			if blockResult.Transactions != nil {
				result.TotalTransactions += uint64(len(blockResult.Transactions))
			}
			if blockResult.Logs != nil {
				result.TotalLogs += uint64(len(blockResult.Logs))
			}

			result.ProcessedBlocks++
			c.logger.Infof("已处理区块 %d", blockResult.BlockNumber)

		case err := <-errorChan:
			if err != nil {
				c.logger.Errorf("采集过程中发生错误: %v", err)
				result.Errors = append(result.Errors, err)
			}
			// 空错误信息是正常的，当工作协程正常完成时会发生
			// 不需要记录警告信息

		case <-ctx.Done():
			c.logger.Warn("采集被取消")
			return result, ctx.Err()
		}
	}
}

// CollectStream 流式采集
func (c *Collector) CollectStream(ctx context.Context) error {
	c.logger.Info("开始实时流处理")

	// 获取最新区块号
	client := c.getClient()
	if client == nil {
		return fmt.Errorf("没有可用的节点")
	}

	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("获取最新区块号失败: %w", err)
	}

	// 设置起始区块
	c.mu.Lock()
	if c.lastSynced == 0 {
		c.lastSynced = latestBlock
	}
	c.mu.Unlock()

	c.logger.Infof("开始监听新区块，当前区块: %d", c.lastSynced)

	// 使用轮询方式替代WebSocket订阅
	ticker := time.NewTicker(DefaultStreamPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取最新区块号
			currentLatest, err := client.BlockNumber(ctx)
			if err != nil {
				c.logger.Errorf("获取最新区块号失败: %v", err)
				continue
			}

			c.mu.Lock()
			lastSynced := c.lastSynced
			c.mu.Unlock()

			// 检查是否有新区块
			if currentLatest <= lastSynced {
				continue
			}

			// 处理新区块
			for blockNumber := lastSynced + 1; blockNumber <= currentLatest; blockNumber++ {
				select {
				case <-ctx.Done():
					c.logger.Info("流处理已停止")
					return ctx.Err()
				default:
				}

				// 采集新区块
				result, err := c.collectBlock(ctx, blockNumber)
				if err != nil {
					c.logger.Errorf("采集区块 %d 失败: %v", blockNumber, err)
					continue
				}

				// 处理数据
				if err := c.processAndOutput(result); err != nil {
					c.logger.Errorf("处理区块 %d 失败: %v", blockNumber, err)
					continue
				}

				c.mu.Lock()
				c.lastSynced = blockNumber
				c.mu.Unlock()

				c.logger.Infof("已处理新区块: %d", blockNumber)
			}

		case <-ctx.Done():
			c.logger.Info("流处理已停止")
			return ctx.Err()
		}
	}
}

// worker 工作协程
func (c *Collector) worker(ctx context.Context, taskChan <-chan uint64, resultChan chan<- *BlockResult, errorChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for blockNum := range taskChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := c.collectBlock(ctx, blockNum)
		if err != nil {
			errorChan <- fmt.Errorf("区块 %d 采集失败: %w", blockNum, err)
			continue
		}

		select {
		case resultChan <- result:
		case <-ctx.Done():
			return
		}
	}
}

// fallbackBlockResult 创建fallback区块结果（仅包含区块头信息）
func (c *Collector) fallbackBlockResult(ctx context.Context, client *ethclient.Client, blockNumber uint64) (*BlockResult, error) {
	header, err := client.HeaderByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, NewConnectionError("获取区块头失败", err)
	}

	c.logger.Warnf("只能获取区块 %d 的头信息，跳过详细交易数据", blockNumber)

	result := &BlockResult{
		BlockNumber: blockNumber,
	}

	// 创建基本的区块模型
	blockModel := &models.Block{
		Number:     blockNumber,
		Hash:       header.Hash().Hex(),
		ParentHash: header.ParentHash.Hex(),
		Timestamp:  time.Unix(int64(header.Time), 0),
		Miner:      header.Coinbase.Hex(),
		GasLimit:   header.GasLimit,
		GasUsed:    0, // 无法获取
		Difficulty: header.Difficulty,
		Nonce:      uint64(header.Nonce.Uint64()),
	}

	result.Block = blockModel
	result.Transactions = []*models.Transaction{}
	result.Logs = []*models.TransactionLog{}

	return result, nil
}

// processTransactions 处理区块中的所有交易
func (c *Collector) processTransactions(ctx context.Context, client *ethclient.Client, block *types.Block) (
	[]*models.Transaction,
	[]*models.TransactionLog,
	[]*models.InternalTransaction,
	[]*models.StateChange,
	[]*models.ContractCreation,
	error) {

	blockNumber := block.NumberU64()
	blockTime := block.Time()

	transactions := make([]*models.Transaction, 0, len(block.Transactions()))
	logs := make([]*models.TransactionLog, 0)
	internalTxs := make([]*models.InternalTransaction, 0)
	stateChanges := make([]*models.StateChange, 0)
	contractCreations := make([]*models.ContractCreation, 0)

	for _, tx := range block.Transactions() {
		// 获取交易收据（使用重试机制）
		var receipt *types.Receipt
		err := c.retrier.Execute(ctx, fmt.Sprintf("获取交易收据%s", tx.Hash().Hex()), func() error {
			var receiptErr error
			receipt, receiptErr = client.TransactionReceipt(ctx, tx.Hash())
			return receiptErr
		})

		if err != nil {
			c.logger.Warnf("获取交易收据失败 %s: %v", tx.Hash().Hex(), err)
			// 继续处理，但不包含日志
		}

		// 转换交易数据
		txModel := &models.Transaction{}
		txModel.FromEthereumTransaction(tx, receipt, blockTime)

		// 解码交易输入数据
		c.decodeTransactionInput(txModel)

		transactions = append(transactions, txModel)

		// 处理日志
		if receipt != nil {
			for _, log := range receipt.Logs {
				logModel := &models.TransactionLog{}
				logModel.FromEthereumLog(log, tx.Hash().Hex(), blockNumber, blockTime)
				logs = append(logs, logModel)
			}
		}

		// 获取内部交易（如果启用追踪且节点支持）
		if c.collectorConfig != nil && c.collectorConfig.EnableTrace {
			if traceResult, err := c.traceTransaction(ctx, tx.Hash(), blockNumber, blockTime); err == nil {
				internalTxs = append(internalTxs, traceResult.InternalTxs...)
			} else {
				// 如果节点不支持追踪，记录但不中断处理
				c.logger.Debugf("节点不支持交易追踪，跳过内部交易获取: %v", err)
			}
		}

		// 获取状态变更
		if scs, err := c.getStateChanges(ctx, tx.Hash(), blockNumber, blockTime); err == nil {
			stateChanges = append(stateChanges, scs...)
		}

		// 获取合约创建详情
		if cc, err := c.getContractCreation(ctx, tx.Hash(), blockNumber, blockTime); err == nil && cc != nil {
			contractCreations = append(contractCreations, cc)
		}
	}

	return transactions, logs, internalTxs, stateChanges, contractCreations, nil
}

// collectBlock 采集单个区块
func (c *Collector) collectBlock(ctx context.Context, blockNumber uint64) (*BlockResult, error) {
	// 获取可用客户端和节点名称
	client, nodeName := c.getClientWithNodeName()
	if client == nil {
		return nil, NewConnectionError("没有可用的节点", nil)
	}

	// 获取区块（使用重试机制）
	var block *types.Block
	err := c.retrier.Execute(ctx, fmt.Sprintf("获取区块%d", blockNumber), func() error {
		var blockErr error
		block, blockErr = client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		return blockErr
	})

	if err != nil {
		// 处理节点错误（包括429检测）
		c.handleNodeError(nodeName, err)
		c.logger.Warnf("节点 %s 获取区块 %d 失败: %v", nodeName, blockNumber, err)
		// 尝试fallback到仅获取区块头
		return c.fallbackBlockResult(ctx, client, blockNumber)
	}

	if block == nil {
		return nil, NewDataError(fmt.Sprintf("区块 %d 不存在", blockNumber), nil)
	}

	// 检测链重组（对于非创世区块）
	if blockNumber > 0 {
		if reorgNotification := c.detectReorg(ctx, block); reorgNotification != nil {
			// 发送重组通知
			if err := c.outputter.WriteReorgNotification(reorgNotification); err != nil {
				c.logger.Errorf("发送重组通知失败: %v", err)
			} else {
				c.logger.Warnf("检测到链重组: 区块 %d, 回滚到区块 %d",
					reorgNotification.DetectedBlockNumber, reorgNotification.RollbackToBlock)
			}
		}
	}

	result := &BlockResult{
		BlockNumber: blockNumber,
	}

	// 转换区块数据
	blockModel := &models.Block{}
	blockModel.FromEthereumBlock(block)

	result.Block = blockModel

	// 只在调试模式下记录详细信息
	if c.logger.Level <= DebugLogLevel {
		c.logger.Debugf("区块 %d 的 Block 模型已设置，StateRoot: %s", blockNumber, blockModel.StateRoot)
	}

	// 处理交易数据
	transactions, logs, internalTxs, stateChanges, contractCreations, err := c.processTransactions(ctx, client, block)
	if err != nil {
		return nil, NewDataError("处理交易数据失败", err)
	}

	// 批量获取内部交易（如果启用追踪且支持trace_block）
	if c.collectorConfig != nil && c.collectorConfig.EnableTrace {
		if blockInternalTxs, err := c.traceBlock(ctx, blockNumber); err == nil {
			// 合并trace_block的结果
			internalTxs = append(internalTxs, blockInternalTxs...)
			c.logger.Debugf("通过trace_block采集到 %d 个内部交易", len(blockInternalTxs))
		} else {
			c.logger.Debugf("trace_block调用失败，回退到逐个交易追踪: %v", err)
		}
	}

	// 采集提款数据 (Shanghai升级后)
	withdrawals := make([]*models.Withdrawal, 0)
	if blockNumber >= models.SHANGHAI_CAPELLA_HEIGHT {
		if blockWithdrawals := block.Withdrawals(); blockWithdrawals != nil {
			for _, w := range blockWithdrawals {
				withdrawalModel := &models.Withdrawal{}
				withdrawalModel.FromEthereumWithdrawal(w, blockNumber, block.Hash().Hex(), time.Unix(int64(block.Time()), 0))
				withdrawals = append(withdrawals, withdrawalModel)
			}
			c.logger.Infof("采集到 %d 个提款记录", len(withdrawals))
		}
	}

	result.Transactions = transactions
	result.Logs = logs
	result.InternalTxs = internalTxs
	result.StateChanges = stateChanges
	result.ContractCreations = contractCreations
	result.Withdrawals = withdrawals

	// 更新区块哈希缓存
	c.updateBlockHashCache(blockNumber, block.Hash().Hex())

	return result, nil
}

// getMapKeys 获取map的所有键（用于调试）
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// traceTransaction 追踪交易内部调用
func (c *Collector) traceTransaction(ctx context.Context, txHash common.Hash, blockNumber uint64, blockTime uint64) (*models.TraceResult, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的节点")
	}

	// 使用 debug_traceTransaction 获取交易追踪
	var result map[string]interface{}
	err := client.Client().CallContext(ctx, &result, "debug_traceTransaction", txHash.Hex(), map[string]interface{}{
		"tracer": "callTracer",
		"tracerConfig": map[string]interface{}{
			"onlyTopCall": false,
			"withLog":     true,
		},
	})

	if err != nil {
		c.logger.Warnf("交易追踪失败 %s: %v", txHash.Hex(), err)
		return nil, err
	}

	// 调试：记录trace结果的结构
	if c.logger.Level >= logrus.DebugLevel {
		c.logger.Debugf("Debug trace result for %s: keys=%v", txHash.Hex(), getMapKeys(result))
		if resultStr, ok := result["result"]; ok {
			c.logger.Debugf("Result field type: %T", resultStr)
		}
	}

	// 解析追踪结果
	traceResult := &models.TraceResult{
		TransactionHash: txHash.Hex(),
		BlockNumber:     blockNumber,
		Timestamp:       time.Unix(int64(blockTime), 0),
	}

	// 解析内部交易 - 递归处理所有调用
	internalTxs := c.parseInternalTransactionsRecursive(result, txHash.Hex(), blockNumber, blockTime, []int{})
	traceResult.InternalTxs = internalTxs

	// 调试：记录实际解析出的内部交易数量
	c.logger.Infof("Debug: Transaction %s parsed %d internal transactions", txHash.Hex(), len(internalTxs))

	// 设置状态
	if errorStr, ok := result["error"].(string); ok && errorStr != "" {
		traceResult.Status = "failed"
	} else {
		traceResult.Status = "success"
	}

	return traceResult, nil
}

// parseInternalTransactionsRecursive 递归解析内部交易
func (c *Collector) parseInternalTransactionsRecursive(traceData map[string]interface{}, txHash string, blockNumber uint64, blockTime uint64, traceAddress []int) []*models.InternalTransaction {
	var internalTxs []*models.InternalTransaction

	// 检查是否是调用
	if _, ok := traceData["from"].(string); ok {
		// 这是一个有效的调用，创建内部交易记录
		internalTx := c.parseInternalTransaction(traceData, txHash, blockNumber, blockTime, traceAddress)
		if internalTx != nil {
			internalTxs = append(internalTxs, internalTx)
		}
	}

	// 递归处理嵌套调用
	if calls, ok := traceData["calls"].([]interface{}); ok {
		for i, call := range calls {
			if callMap, ok := call.(map[string]interface{}); ok {
				// 为嵌套调用创建新的trace地址
				nestedTraceAddress := make([]int, len(traceAddress)+1)
				copy(nestedTraceAddress, traceAddress)
				nestedTraceAddress[len(traceAddress)] = i

				// 递归处理
				nestedTxs := c.parseInternalTransactionsRecursive(callMap, txHash, blockNumber, blockTime, nestedTraceAddress)
				internalTxs = append(internalTxs, nestedTxs...)
			}
		}
	}

	return internalTxs
}

// parseInternalTransaction 解析内部交易
func (c *Collector) parseInternalTransaction(call map[string]interface{}, txHash string, blockNumber uint64, blockTime uint64, traceAddress []int) *models.InternalTransaction {
	internalTx := &models.InternalTransaction{
		TransactionHash: txHash,
		BlockNumber:     blockNumber,
		Timestamp:       time.Unix(int64(blockTime), 0),
	}

	// 解析基本信息
	if from, ok := call["from"].(string); ok {
		internalTx.From = from
	}
	if to, ok := call["to"].(string); ok {
		internalTx.To = to
	}
	if callType, ok := call["type"].(string); ok {
		internalTx.CallType = callType
	}
	if value, ok := call["value"].(string); ok {
		if bigValue, ok := new(big.Int).SetString(value[2:], 16); ok { // 移除 "0x" 前缀
			internalTx.Value = bigValue
		}
	}
	if gas, ok := call["gas"].(string); ok {
		if gasUint, ok := new(big.Int).SetString(gas[2:], 16); ok {
			internalTx.Gas = gasUint.Uint64()
		}
	}
	if gasUsed, ok := call["gasUsed"].(string); ok {
		if gasUsedUint, ok := new(big.Int).SetString(gasUsed[2:], 16); ok {
			internalTx.GasUsed = gasUsedUint.Uint64()
		}
	}
	if input, ok := call["input"].(string); ok {
		internalTx.Input = input
	}
	if output, ok := call["output"].(string); ok {
		internalTx.Output = output
	}
	if errorStr, ok := call["error"].(string); ok {
		internalTx.Error = errorStr
	}

	// 设置追踪地址（从参数传入，而不是从调用数据中解析）
	internalTx.TraceAddress = make([]int, len(traceAddress))
	copy(internalTx.TraceAddress, traceAddress)

	// 设置类型
	switch internalTx.CallType {
	case "CREATE":
		internalTx.Type = "create"
	case "SUICIDE":
		internalTx.Type = "suicide"
	default:
		internalTx.Type = "call"
	}

	return internalTx
}

// getStateChanges 获取状态变更
func (c *Collector) getStateChanges(ctx context.Context, txHash common.Hash, blockNumber uint64, blockTime uint64) ([]*models.StateChange, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的节点")
	}

	var stateChanges []*models.StateChange

	// 获取交易收据以了解涉及的合约
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// 检查是否有合约创建
	if receipt.ContractAddress != (common.Address{}) {
		// 获取合约代码
		code, err := client.CodeAt(ctx, receipt.ContractAddress, big.NewInt(int64(blockNumber)))
		if err == nil && len(code) > 0 {
			stateChange := &models.StateChange{
				TransactionHash: txHash.Hex(),
				BlockNumber:     blockNumber,
				Address:         receipt.ContractAddress.Hex(),
				Type:            "code",
				OldValue:        "",
				NewValue:        common.Bytes2Hex(code),
				Timestamp:       time.Unix(int64(blockTime), 0),
			}
			stateChanges = append(stateChanges, stateChange)
		}
	}

	// 检查日志中的状态变更
	for _, log := range receipt.Logs {
		// 这里可以添加更详细的状态变更检测逻辑
		// 例如：检查特定的事件来推断状态变更
		if len(log.Topics) > 0 {
			// 检查是否是存储变更事件
			stateChange := &models.StateChange{
				TransactionHash: txHash.Hex(),
				BlockNumber:     blockNumber,
				Address:         log.Address.Hex(),
				Type:            "storage",
				OldValue:        "",
				NewValue:        common.Bytes2Hex(log.Data),
				Timestamp:       time.Unix(int64(blockTime), 0),
			}
			stateChanges = append(stateChanges, stateChange)
		}
	}

	return stateChanges, nil
}

// getContractCreation 获取合约创建详情
func (c *Collector) getContractCreation(ctx context.Context, txHash common.Hash, blockNumber uint64, blockTime uint64) (*models.ContractCreation, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的节点")
	}

	// 获取交易收据
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// 检查是否有合约创建
	if receipt.ContractAddress == (common.Address{}) {
		return nil, nil // 不是合约创建交易
	}

	// 获取合约代码
	code, err := client.CodeAt(ctx, receipt.ContractAddress, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	// 获取交易详情
	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// 获取发送方地址
	var from common.Address
	switch tx.Type() {
	case types.LegacyTxType:
		from, err = types.Sender(types.NewEIP155Signer(tx.ChainId()), tx)
	case types.AccessListTxType:
		from, err = types.Sender(types.NewEIP2930Signer(tx.ChainId()), tx)
	case types.DynamicFeeTxType:
		from, err = types.Sender(types.NewLondonSigner(tx.ChainId()), tx)
	default:
		from, err = types.Sender(types.NewLondonSigner(tx.ChainId()), tx)
	}

	if err != nil {
		return nil, err
	}

	// 获取区块信息以设置BlockHash
	block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	contractCreation := &models.ContractCreation{
		TransactionHash: txHash.Hex(),
		BlockNumber:     blockNumber,
		BlockHash:       block.Hash().Hex(),
		ContractAddress: receipt.ContractAddress.Hex(),
		Creator:         from.Hex(),
		Code:            common.Bytes2Hex(code),
		CodeSize:        len(code),
		RuntimeCode:     common.Bytes2Hex(code), // 运行时代码通常与部署代码相同
		RuntimeCodeSize: len(code),
		ConstructorArgs: common.Bytes2Hex(tx.Data()),
		GasUsed:         receipt.GasUsed,
		Timestamp:       time.Unix(int64(blockTime), 0),
	}

	// 分析合约类型
	c.analyzeContractType(ctx, contractCreation, receipt.ContractAddress)

	return contractCreation, nil
}

// processAndOutput 处理并输出数据
func (c *Collector) processAndOutput(result *BlockResult) error {
	// 调试信息只在调试模式下输出
	if c.logger.Level <= DebugLogLevel {
		c.logger.Debugf("processAndOutput 被调用，区块 %d，Block是否为nil: %v", result.BlockNumber, result.Block == nil)
	}

	if result.Block != nil {
		c.logger.Infof("开始处理区块 %d 的数据", result.BlockNumber)

		// 输出区块数据
		if err := c.outputter.WriteBlock(result.Block); err != nil {
			return fmt.Errorf("输出区块数据失败: %w", err)
		}
		c.logger.Infof("区块 %d 数据输出成功", result.BlockNumber)
	}

	if len(result.Transactions) > 0 {
		for _, tx := range result.Transactions {
			if err := c.outputter.WriteTransaction(tx); err != nil {
				return fmt.Errorf("输出交易数据失败: %w", err)
			}
		}
		c.logger.Infof("输出了 %d 个交易", len(result.Transactions))
	}

	// 批量输出各类数据
	if err := c.outputDataBatch(result); err != nil {
		return err
	}

	c.logger.Infof("区块 %d 处理完成: %d 交易, %d 日志, %d 内部交易, %d 状态变更, %d 合约创建, %d 提款",
		result.BlockNumber, len(result.Transactions), len(result.Logs),
		len(result.InternalTxs), len(result.StateChanges), len(result.ContractCreations), len(result.Withdrawals))

	return nil
}

// outputDataBatch 批量输出各类数据
func (c *Collector) outputDataBatch(result *BlockResult) error {
	// 输出日志
	if len(result.Logs) > 0 {
		for _, log := range result.Logs {
			if err := c.outputter.WriteLog(log); err != nil {
				return fmt.Errorf("输出日志数据失败: %w", err)
			}
		}
		c.logger.Debugf("输出了 %d 个日志", len(result.Logs))
	}

	// 输出内部交易
	if len(result.InternalTxs) > 0 {
		for _, itx := range result.InternalTxs {
			if err := c.outputter.WriteInternalTransaction(itx); err != nil {
				return fmt.Errorf("输出内部交易数据失败: %w", err)
			}
		}
		c.logger.Debugf("输出了 %d 个内部交易", len(result.InternalTxs))
	}

	// 输出状态变更
	if len(result.StateChanges) > 0 {
		for _, sc := range result.StateChanges {
			if err := c.outputter.WriteStateChange(sc); err != nil {
				return fmt.Errorf("输出状态变更数据失败: %w", err)
			}
		}
		c.logger.Debugf("输出了 %d 个状态变更", len(result.StateChanges))
	}

	// 输出合约创建
	if len(result.ContractCreations) > 0 {
		for _, cc := range result.ContractCreations {
			if err := c.outputter.WriteContractCreation(cc); err != nil {
				return fmt.Errorf("输出合约创建数据失败: %w", err)
			}
		}
		c.logger.Debugf("输出了 %d 个合约创建", len(result.ContractCreations))
	}

	// 输出提款数据
	if len(result.Withdrawals) > 0 {
		for _, w := range result.Withdrawals {
			if err := c.outputter.WriteWithdrawal(w); err != nil {
				return fmt.Errorf("输出提款数据失败: %w", err)
			}
		}
		c.logger.Debugf("输出了 %d 个提款记录", len(result.Withdrawals))
	}

	return nil
}

// GetNodeStatus 获取所有节点的状态信息
func (c *Collector) GetNodeStatus() map[string]interface{} {
	status := make(map[string]interface{})
	nodes := make([]map[string]interface{}, 0, len(c.nodes))

	now := time.Now()
	for _, node := range c.nodes {
		node.mu.RLock()
		nodeInfo := map[string]interface{}{
			"name":         node.Name,
			"url":          node.URL,
			"type":         node.Type,
			"priority":     node.Priority,
			"available":    node.Available,
			"rate_limited": node.RateLimited,
			"error_count":  node.ErrorCount,
		}

		if node.RateLimited {
			remainingTime := node.RateLimitEnd.Sub(now)
			if remainingTime > 0 {
				nodeInfo["rate_limit_remaining"] = remainingTime.String()
			} else {
				nodeInfo["rate_limit_remaining"] = "已过期"
			}
		}

		node.mu.RUnlock()
		nodes = append(nodes, nodeInfo)
	}

	status["nodes"] = nodes
	status["total_nodes"] = len(c.nodes)

	// 统计可用节点数
	availableCount := 0
	rateLimitedCount := 0
	for _, node := range c.nodes {
		node.mu.RLock()
		if node.Available && !node.RateLimited {
			availableCount++
		}
		if node.RateLimited && now.Before(node.RateLimitEnd) {
			rateLimitedCount++
		}
		node.mu.RUnlock()
	}

	status["available_nodes"] = availableCount
	status["rate_limited_nodes"] = rateLimitedCount

	return status
}

// PrintNodeStatus 打印节点状态（用于调试）
func (c *Collector) PrintNodeStatus() {
	status := c.GetNodeStatus()
	fmt.Printf("\n📊 节点状态报告:\n")
	fmt.Printf("总节点数: %d | 可用: %d | 被限制: %d\n",
		status["total_nodes"], status["available_nodes"], status["rate_limited_nodes"])
	fmt.Printf("%s\n", strings.Repeat("=", 50))

	nodes := status["nodes"].([]map[string]interface{})
	for _, nodeInfo := range nodes {
		name := nodeInfo["name"].(string)
		nodeType := nodeInfo["type"].(string)
		available := nodeInfo["available"].(bool)
		rateLimited := nodeInfo["rate_limited"].(bool)
		errorCount := nodeInfo["error_count"].(int)

		statusIcon := "✅"
		statusText := "正常"

		if rateLimited {
			statusIcon = "🚫"
			remaining := nodeInfo["rate_limit_remaining"].(string)
			statusText = fmt.Sprintf("速率限制 (剩余: %s)", remaining)
		} else if !available {
			statusIcon = "❌"
			statusText = "不可用"
		}

		fmt.Printf("%s %s (%s) - %s [错误: %d]\n",
			statusIcon, name, nodeType, statusText, errorCount)
	}
	fmt.Println()
}

// Close 关闭采集器
func (c *Collector) Close() {
	// 触发优雅停机
	if c.gracefulShutdown != nil {
		c.gracefulShutdown.Shutdown()
	}

	// 手动关闭资源（如果优雅停机失败）
	c.closeResources()
}

// closeResources 关闭资源
func (c *Collector) closeResources() {
	// 关闭节点连接
	for _, node := range c.nodes {
		if node.Client != nil {
			node.Client.Close()
		}
	}

	// 关闭进度管理器
	if c.progressManager != nil {
		if err := c.progressManager.Close(); err != nil {
			c.logger.Errorf("关闭进度管理器失败: %v", err)
		}
	}

	// 关闭输出器
	if c.outputter != nil {
		if err := c.outputter.Close(); err != nil {
			c.logger.Errorf("关闭输出器失败: %v", err)
		}
	}

	// 关闭优雅停机管理器
	if c.gracefulShutdown != nil {
		if err := c.gracefulShutdown.Close(); err != nil {
			c.logger.Errorf("关闭优雅停机管理器失败: %v", err)
		}
	}
}

// detectReorg 检测链重组
func (c *Collector) detectReorg(ctx context.Context, currentBlock *types.Block) *models.ReorgNotification {
	if currentBlock == nil {
		return nil
	}

	blockNumber := currentBlock.NumberU64()
	parentNumber := blockNumber - 1

	// 获取当前区块的父哈希
	parentHashFromChain := currentBlock.ParentHash().Hex()

	// 从缓存中获取记录的父区块哈希
	c.mu.RLock()
	cachedParentHash, exists := c.blockHashCache[parentNumber]
	c.mu.RUnlock()

	// 如果缓存中没有父区块记录，这可能是第一次运行，不是重组
	if !exists {
		return nil
	}

	// 比较哈希值
	if parentHashFromChain != cachedParentHash {
		// 检测到重组！需要找到分叉点
		rollbackToBlock := c.findCommonAncestor(ctx, parentNumber)

		// 创建重组通知
		reorg := &models.ReorgNotification{
			Type:                "reorg",
			DetectedBlockNumber: blockNumber,
			RollbackToBlock:     rollbackToBlock,
			OldBlockHash:        cachedParentHash,
			NewBlockHash:        parentHashFromChain,
			DetectionTime:       time.Now(),
			AffectedBlocks:      blockNumber - rollbackToBlock,
			Message:             fmt.Sprintf("检测到链重组：区块 %d 的父哈希从 %s 变为 %s", blockNumber, cachedParentHash, parentHashFromChain),
		}

		// 确定严重程度
		reorg.DetermineSeverity()

		// 清理重组范围内的缓存
		c.cleanupHashCache(rollbackToBlock + 1)

		return reorg
	}

	return nil
}

// findCommonAncestor 找到共同祖先区块（分叉点）
func (c *Collector) findCommonAncestor(ctx context.Context, startBlock uint64) uint64 {
	client := c.getClient()
	if client == nil {
		// 如果无法获取客户端，保守地返回较早的区块
		if startBlock >= 10 {
			return startBlock - 10
		}
		return 0
	}

	// 向前查找，最多检查100个区块
	maxLookback := uint64(100)
	for i := uint64(0); i < maxLookback && startBlock >= i; i++ {
		checkBlock := startBlock - i

		// 从链上获取区块哈希
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(checkBlock)))
		if err != nil {
			c.logger.Warnf("获取区块 %d 失败，停止查找共同祖先: %v", checkBlock, err)
			break
		}

		if block == nil {
			continue
		}

		chainHash := block.Hash().Hex()

		// 检查缓存中的哈希
		c.mu.RLock()
		cachedHash, exists := c.blockHashCache[checkBlock]
		c.mu.RUnlock()

		if exists && chainHash == cachedHash {
			// 找到共同祖先
			return checkBlock
		}
	}

	// 如果找不到共同祖先，保守地返回更早的区块
	if startBlock >= maxLookback {
		return startBlock - maxLookback
	}
	return 0
}

// updateBlockHashCache 更新区块哈希缓存
func (c *Collector) updateBlockHashCache(blockNumber uint64, blockHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 更新缓存
	c.blockHashCache[blockNumber] = blockHash

	// 限制缓存大小，只保留最近的1000个区块
	maxCacheSize := 1000
	if len(c.blockHashCache) > maxCacheSize {
		// 找到最小的区块号
		minBlock := blockNumber
		for bn := range c.blockHashCache {
			if bn < minBlock {
				minBlock = bn
			}
		}

		// 删除过旧的记录
		deleteCount := len(c.blockHashCache) - maxCacheSize + 1
		for i := 0; i < deleteCount; i++ {
			delete(c.blockHashCache, minBlock+uint64(i))
		}
	}
}

// cleanupHashCache 清理指定区块号之后的缓存
func (c *Collector) cleanupHashCache(fromBlock uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 删除fromBlock及之后的所有缓存记录
	for blockNumber := range c.blockHashCache {
		if blockNumber >= fromBlock {
			delete(c.blockHashCache, blockNumber)
		}
	}

	c.logger.Infof("已清理区块 %d 及之后的哈希缓存", fromBlock)
}

// decodeTransactionInput 解码交易输入数据
func (c *Collector) decodeTransactionInput(tx *models.Transaction) {
	if tx == nil || tx.Input == "" || tx.Input == "0x" {
		tx.IsContractCall = false
		return
	}

	// 使用解码器解码输入数据
	methodSig, methodName, decodedParams, isContractCall := c.inputDecoder.DecodeInput(tx.Input)

	tx.IsContractCall = isContractCall
	if isContractCall {
		tx.MethodSignature = methodSig
		tx.MethodName = methodName
		tx.DecodedInput = decodedParams

		c.logger.Debugf("解码交易 %s: 方法=%s, 参数数量=%d",
			tx.Hash, methodName, len(decodedParams))
	}
}

// analyzeContractType 分析合约类型
func (c *Collector) analyzeContractType(ctx context.Context, contractCreation *models.ContractCreation, contractAddr common.Address) {
	client := c.getClient()
	if client == nil {
		contractCreation.ContractType = "unknown"
		return
	}

	codeHex := contractCreation.Code
	if codeHex == "" {
		contractCreation.ContractType = "eoa"
		return
	}

	// 基本合约类型判断
	contractCreation.ContractType = "contract"

	// 检查是否为代理合约
	if c.isProxyContract(codeHex) {
		contractCreation.IsProxy = true
		contractCreation.ContractType = "proxy"

		// 尝试获取实现合约地址
		if implAddr := c.getProxyImplementation(ctx, contractAddr); implAddr != "" {
			contractCreation.ImplementationAddress = implAddr
		}
	}

	// 检查是否为ERC20代币合约
	if c.isERC20Contract(codeHex) {
		if contractCreation.ContractType == "contract" {
			contractCreation.ContractType = "token"
		}
	}
}

// isProxyContract 检查是否为代理合约
func (c *Collector) isProxyContract(codeHex string) bool {
	if len(codeHex) < 8 {
		return false
	}

	// 检查常见的代理合约字节码模式
	proxyPatterns := []string{
		"363d3d373d3d3d363d73", // EIP-1167 Minimal Proxy
		"60806040526004361061", // 常见的代理合约开头
		"60a060405260043610",   // 另一种代理模式
		"363d3d373d3d3d363d30", // CREATE2 minimal proxy
	}

	for _, pattern := range proxyPatterns {
		if strings.Contains(strings.ToLower(codeHex), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// isERC20Contract 检查是否为ERC20代币合约
func (c *Collector) isERC20Contract(codeHex string) bool {
	if len(codeHex) < 16 {
		return false
	}

	// 检查ERC20标准方法的函数选择器
	erc20Selectors := []string{
		"a9059cbb", // transfer(address,uint256)
		"095ea7b3", // approve(address,uint256)
		"23b872dd", // transferFrom(address,address,uint256)
		"70a08231", // balanceOf(address)
		"18160ddd", // totalSupply()
	}

	foundSelectors := 0
	codeHexLower := strings.ToLower(codeHex)

	for _, selector := range erc20Selectors {
		if strings.Contains(codeHexLower, selector) {
			foundSelectors++
		}
	}

	// 如果找到3个或以上的ERC20方法选择器，认为是ERC20合约
	return foundSelectors >= 3
}

// getProxyImplementation 获取代理合约的实现地址
func (c *Collector) getProxyImplementation(ctx context.Context, proxyAddr common.Address) string {
	client := c.getClient()
	if client == nil {
		return ""
	}

	// EIP-1967 标准代理存储槽
	// implementation slot: keccak256("eip1967.proxy.implementation") - 1
	implementationSlot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")

	// 读取存储槽
	result, err := client.StorageAt(ctx, proxyAddr, implementationSlot, nil)
	if err != nil {
		c.logger.Debugf("读取代理实现地址失败: %v", err)
		return ""
	}

	// 提取地址（存储在低20字节）
	if len(result) >= 20 {
		implAddr := common.BytesToAddress(result[12:32])
		if implAddr != (common.Address{}) {
			return implAddr.Hex()
		}
	}

	return ""
}

// traceBlock 批量追踪整个区块的内部交易
func (c *Collector) traceBlock(ctx context.Context, blockNumber uint64) ([]*models.InternalTransaction, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的节点")
	}

	// 首先获取区块信息以获得时间戳（使用重试机制）
	var block *types.Block
	err := c.retrier.Execute(ctx, fmt.Sprintf("获取区块信息%d", blockNumber), func() error {
		var blockErr error
		block, blockErr = client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		return blockErr
	})
	if err != nil {
		return nil, fmt.Errorf("获取区块信息失败: %w", err)
	}

	blockTime := block.Time()

	// 使用 trace_block 获取整个区块的追踪信息（使用重试机制）
	var traceResults []map[string]interface{}
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	err = c.retrier.Execute(ctx, fmt.Sprintf("trace_block%d", blockNumber), func() error {
		return client.Client().CallContext(ctx, &traceResults, "trace_block", blockNumberHex)
	})
	if err != nil {
		// trace_block可能不被支持，返回错误让调用者回退到单个交易追踪
		return nil, fmt.Errorf("trace_block调用失败: %w", err)
	}

	var allInternalTxs []*models.InternalTransaction

	// 解析trace_block的结果
	for _, traceResult := range traceResults {
		internalTxs := c.parseTraceBlockResult(traceResult, blockNumber, blockTime)
		allInternalTxs = append(allInternalTxs, internalTxs...)
	}

	c.logger.Debugf("trace_block成功获取区块 %d 的 %d 个内部交易", blockNumber, len(allInternalTxs))
	return allInternalTxs, nil
}

// parseTraceBlockResult 解析trace_block的单个结果
func (c *Collector) parseTraceBlockResult(traceResult map[string]interface{}, blockNumber uint64, blockTime uint64) []*models.InternalTransaction {
	var internalTxs []*models.InternalTransaction

	// 检查trace类型
	traceType, ok := traceResult["type"].(string)
	if !ok {
		return internalTxs
	}

	// 只处理call类型的trace
	if traceType != "call" {
		return internalTxs
	}

	// 获取交易哈希
	txHash, ok := traceResult["transactionHash"].(string)
	if !ok {
		return internalTxs
	}

	// 获取action信息
	action, ok := traceResult["action"].(map[string]interface{})
	if !ok {
		return internalTxs
	}

	// 创建内部交易记录
	internalTx := &models.InternalTransaction{
		TransactionHash: txHash,
		BlockNumber:     blockNumber,
		Timestamp:       time.Unix(int64(blockTime), 0),
	}

	// 解析基本信息
	if from, ok := action["from"].(string); ok {
		internalTx.From = from
	}
	if to, ok := action["to"].(string); ok {
		internalTx.To = to
	}
	if value, ok := action["value"].(string); ok {
		if bigValue, ok := new(big.Int).SetString(value[2:], 16); ok { // 移除 "0x" 前缀
			internalTx.Value = bigValue
		}
	}
	if gas, ok := action["gas"].(string); ok {
		if gasUint, ok := new(big.Int).SetString(gas[2:], 16); ok {
			internalTx.Gas = gasUint.Uint64()
		}
	}
	if input, ok := action["input"].(string); ok {
		internalTx.Input = input
	}

	// 设置调用类型
	internalTx.CallType = "call"

	// 获取结果信息
	if result, ok := traceResult["result"].(map[string]interface{}); ok {
		if gasUsed, ok := result["gasUsed"].(string); ok {
			if gasUsedUint, ok := new(big.Int).SetString(gasUsed[2:], 16); ok {
				internalTx.GasUsed = gasUsedUint.Uint64()
			}
		}
		if output, ok := result["output"].(string); ok {
			internalTx.Output = output
		}
	}

	// 检查错误状态
	if errorStr, ok := traceResult["error"].(string); ok && errorStr != "" {
		internalTx.Error = errorStr
	}

	// 获取trace地址
	if traceAddress, ok := traceResult["traceAddress"].([]interface{}); ok {
		addressInts := make([]int, len(traceAddress))
		for i, addr := range traceAddress {
			if addrInt, ok := addr.(float64); ok {
				addressInts[i] = int(addrInt)
			}
		}
		internalTx.TraceAddress = addressInts
	}

	internalTxs = append(internalTxs, internalTx)
	return internalTxs
}

// ConcurrentCollectBatch 高性能并发批量采集
func (c *Collector) ConcurrentCollectBatch(ctx context.Context, startBlock, endBlock uint64, workers, batchSize int) (*BatchResult, error) {
	// 验证参数
	if err := validateBatchParams(startBlock, endBlock, workers, batchSize); err != nil {
		return nil, NewValidationError(err.Error())
	}

	c.logger.Infof("开始高性能并发采集区块 %d - %d，使用 %d 个工作者", startBlock, endBlock, workers)

	result := &BatchResult{
		StartBlock: startBlock,
		EndBlock:   endBlock,
		StartTime:  time.Now(),
	}

	// 计算合适的缓冲区大小
	bufferSize := workers * 3
	if bufferSize < 100 {
		bufferSize = 100
	}
	if bufferSize > 1000 {
		bufferSize = 1000
	}

	// 创建流水线通道
	taskChan := make(chan uint64, bufferSize)
	fetchedChan := make(chan *BlockResult, bufferSize)
	processedChan := make(chan *BlockResult, bufferSize)
	outputChan := make(chan *BlockResult, bufferSize)
	errorChan := make(chan error, workers*2)

	// 控制goroutine生命周期
	var fetchWG, processWG, outputWG sync.WaitGroup

	// 1. 任务生产者
	go c.concurrentTaskProducer(ctx, startBlock, endBlock, taskChan)

	// 2. 数据获取者 - 多个worker并发获取区块数据
	for i := 0; i < workers; i++ {
		fetchWG.Add(1)
		go c.concurrentBlockFetcher(ctx, taskChan, fetchedChan, errorChan, &fetchWG, i)
	}

	// 3. 数据处理者 - 单个goroutine处理数据，保证顺序
	processWG.Add(1)
	go c.concurrentDataProcessor(ctx, fetchedChan, processedChan, &processWG)

	// 4. 数据输出者 - 单个goroutine输出到Kafka
	outputWG.Add(1)
	go c.concurrentDataSender(ctx, processedChan, outputChan, &outputWG)

	// 5. 结果收集者 - 统计结果
	go c.concurrentResultCollector(outputChan, result)

	// 管理goroutine生命周期
	go func() {
		fetchWG.Wait()
		close(fetchedChan)
	}()

	go func() {
		processWG.Wait()
		close(processedChan)
	}()

	go func() {
		outputWG.Wait()
		close(outputChan)
	}()

	// 等待完成或处理错误
	return c.waitForCompletion(ctx, result, errorChan)
}

// concurrentTaskProducer 生产区块号任务
func (c *Collector) concurrentTaskProducer(ctx context.Context, startBlock, endBlock uint64, taskChan chan<- uint64) {
	defer close(taskChan)

	total := endBlock - startBlock + 1
	c.logger.Debugf("任务生产者开始，总共 %d 个区块", total)

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		select {
		case taskChan <- blockNum:
		case <-ctx.Done():
			c.logger.Debug("任务生产者停止（上下文取消）")
			return
		}
	}
	c.logger.Debug("任务生产者完成，所有任务已发送")
}

// concurrentBlockFetcher 并发获取区块数据
func (c *Collector) concurrentBlockFetcher(ctx context.Context, taskChan <-chan uint64, fetchedChan chan<- *BlockResult, errorChan chan<- error, wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	c.logger.Debugf("区块获取器 %d 启动", workerID)
	processedCount := 0

	for blockNum := range taskChan {
		select {
		case <-ctx.Done():
			c.logger.Debugf("区块获取器 %d 停止（上下文取消，已处理 %d 个）", workerID, processedCount)
			return
		default:
		}

		// 获取区块数据，包含所有相关信息
		result, err := c.collectBlock(ctx, blockNum)
		if err != nil {
			select {
			case errorChan <- fmt.Errorf("获取器 %d 处理区块 %d 失败: %w", workerID, blockNum, err):
			case <-ctx.Done():
				return
			}
			continue
		}

		select {
		case fetchedChan <- result:
			processedCount++
			if processedCount%50 == 0 {
				c.logger.Debugf("获取器 %d 已处理 %d 个区块", workerID, processedCount)
			}
		case <-ctx.Done():
			c.logger.Debugf("区块获取器 %d 停止（上下文取消，已处理 %d 个）", workerID, processedCount)
			return
		}
	}

	c.logger.Debugf("区块获取器 %d 完成（共处理 %d 个区块）", workerID, processedCount)
}

// concurrentDataProcessor 处理区块数据（可以在这里添加额外的数据处理逻辑）
func (c *Collector) concurrentDataProcessor(ctx context.Context, fetchedChan <-chan *BlockResult, processedChan chan<- *BlockResult, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(processedChan)

	c.logger.Debug("数据处理器启动")
	processedCount := 0

	for blockResult := range fetchedChan {
		select {
		case <-ctx.Done():
			c.logger.Debugf("数据处理器停止（上下文取消，已处理 %d 个）", processedCount)
			return
		default:
		}

		// 这里可以添加额外的数据处理、验证、转换等逻辑
		// 目前直接传递数据

		select {
		case processedChan <- blockResult:
			processedCount++
			if processedCount%100 == 0 {
				c.logger.Debugf("数据处理器已处理 %d 个区块", processedCount)
			}
		case <-ctx.Done():
			c.logger.Debugf("数据处理器停止（上下文取消，已处理 %d 个）", processedCount)
			return
		}
	}

	c.logger.Debugf("数据处理器完成（共处理 %d 个区块）", processedCount)
}

// concurrentDataSender 发送数据到输出系统
func (c *Collector) concurrentDataSender(ctx context.Context, processedChan <-chan *BlockResult, outputChan chan<- *BlockResult, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(outputChan)

	c.logger.Debug("数据发送器启动")
	sentCount := 0

	for blockResult := range processedChan {
		select {
		case <-ctx.Done():
			c.logger.Debugf("数据发送器停止（上下文取消，已发送 %d 个）", sentCount)
			return
		default:
		}

		// 发送到Kafka等输出系统
		if err := c.processAndOutput(blockResult); err != nil {
			c.logger.Errorf("输出区块 %d 失败: %v", blockResult.BlockNumber, err)
			// 继续处理其他区块，不因单个错误停止
			continue
		}

		// 更新进度
		transactionCount := 0
		if blockResult.Transactions != nil {
			transactionCount = len(blockResult.Transactions)
		}
		c.updateProcessingProgress(blockResult.BlockNumber, transactionCount)

		select {
		case outputChan <- blockResult:
			sentCount++
			if sentCount%100 == 0 {
				c.logger.Infof("已发送 %d 个区块到输出系统", sentCount)
			}
		case <-ctx.Done():
			c.logger.Debugf("数据发送器停止（上下文取消，已发送 %d 个）", sentCount)
			return
		}
	}

	c.logger.Debugf("数据发送器完成（共发送 %d 个区块）", sentCount)
}

// concurrentResultCollector 收集处理结果和统计信息
func (c *Collector) concurrentResultCollector(outputChan <-chan *BlockResult, result *BatchResult) {
	c.logger.Debug("结果收集器启动")

	for blockResult := range outputChan {
		// 更新统计信息
		if blockResult.Block != nil {
			result.TotalBlocks++
		}
		if blockResult.Transactions != nil {
			result.TotalTransactions += uint64(len(blockResult.Transactions))
		}
		if blockResult.Logs != nil {
			result.TotalLogs += uint64(len(blockResult.Logs))
		}

		result.ProcessedBlocks++
	}

	c.logger.Debugf("结果收集器完成（共收集 %d 个区块结果）", result.ProcessedBlocks)
}

// waitForCompletion 等待所有goroutine完成或处理错误
func (c *Collector) waitForCompletion(ctx context.Context, result *BatchResult, errorChan chan error) (*BatchResult, error) {
	c.logger.Debug("等待采集完成...")

	errorCount := 0
	completionTimer := time.NewTimer(5 * time.Minute) // 5分钟超时
	defer completionTimer.Stop()

	for {
		select {
		case err, ok := <-errorChan:
			if !ok {
				// 错误通道关闭，表示完成
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(result.StartTime)

				if result.Duration.Seconds() > 0 {
					result.BlocksPerSecond = float64(result.ProcessedBlocks) / result.Duration.Seconds()
					result.TransactionsPerSecond = float64(result.TotalTransactions) / result.Duration.Seconds()
				}

				if errorCount > 0 {
					c.logger.Warnf("并发采集完成，共发生 %d 个错误", errorCount)
				} else {
					c.logger.Infof("并发采集成功完成，处理了 %d 个区块", result.ProcessedBlocks)
				}

				return result, nil
			}

			if err != nil {
				c.logger.Errorf("采集错误: %v", err)
				result.Errors = append(result.Errors, err)
				errorCount++

				// 如果错误过多，考虑终止
				if errorCount > 100 {
					c.logger.Errorf("错误过多（%d个），停止采集", errorCount)
					result.EndTime = time.Now()
					result.Duration = result.EndTime.Sub(result.StartTime)
					return result, fmt.Errorf("错误过多，停止采集")
				}
			}

		case <-ctx.Done():
			c.logger.Warn("并发采集被取消")
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, ctx.Err()

		case <-completionTimer.C:
			c.logger.Warn("采集超时，可能存在问题")
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, fmt.Errorf("采集超时")
		}
	}
}

// getResumeBlock 获取断点续传的起始区块
func (c *Collector) getResumeBlock(configStartBlock uint64) uint64 {
	if c.progressManager == nil {
		return configStartBlock
	}

	lastProcessed := c.progressManager.GetLastProcessedBlock()
	if lastProcessed == 0 {
		// 没有进度记录，使用配置的起始区块
		if err := c.progressManager.SetStartBlock(configStartBlock); err != nil {
			c.logger.Warnf("设置起始区块失败: %v", err)
		}
		return configStartBlock
	}

	// 从上次处理的区块的下一个区块开始
	resumeBlock := lastProcessed + 1

	// 确保不会超过配置的起始区块
	if resumeBlock < configStartBlock {
		resumeBlock = configStartBlock
	}

	return resumeBlock
}

// updateProcessingProgress 更新处理进度
func (c *Collector) updateProcessingProgress(blockNumber uint64, transactionCount int) {
	if c.progressManager == nil {
		return
	}

	// 更新区块进度
	if err := c.progressManager.UpdateProgress(blockNumber); err != nil {
		c.logger.Debugf("更新进度失败: %v", err)
	}

	// 更新交易数量
	if transactionCount > 0 {
		c.progressManager.UpdateTransactionCount(uint64(transactionCount))
	}
}

// GetProgressInfo 获取当前进度信息
func (c *Collector) GetProgressInfo() map[string]interface{} {
	if c.progressManager == nil {
		return map[string]interface{}{
			"progress_tracking": "disabled",
		}
	}

	return c.progressManager.GetStats()
}

// ResetProgress 重置进度（谨慎使用）
func (c *Collector) ResetProgress() error {
	if c.progressManager == nil {
		return fmt.Errorf("进度管理器未初始化")
	}

	c.logger.Warn("重置采集进度...")
	return c.progressManager.Reset()
}

// SaveProgressCheckpoint 保存进度检查点
func (c *Collector) SaveProgressCheckpoint() error {
	if c.progressManager == nil {
		return fmt.Errorf("进度管理器未初始化")
	}

	// 获取当前进度并保存
	progress := c.progressManager.GetProgress()
	if err := c.progressManager.SaveCheckpoint(progress); err != nil {
		return fmt.Errorf("保存进度检查点失败: %w", err)
	}

	c.logger.Infof("已保存进度检查点，最后处理区块: %d", progress.LastProcessedBlock)
	return nil
}

// registerShutdownHandlers 注册停机处理函数
func (c *Collector) registerShutdownHandlers() {
	if c.gracefulShutdown == nil {
		return
	}

	// 1. 停止接受新的区块处理请求
	c.gracefulShutdown.RegisterShutdownFunc(
		"stop_accepting_blocks",
		func(ctx context.Context) error {
			c.logger.Info("停止接受新的区块处理请求...")
			// 这里可以设置一个标志来拒绝新的处理请求
			return nil
		},
		shutdown.OrderStopAcceptingRequests,
	)

	// 2. 等待当前活跃的区块处理完成
	c.gracefulShutdown.RegisterShutdownFunc(
		"wait_active_processing",
		func(ctx context.Context) error {
			c.logger.Info("等待当前活跃的区块处理完成...")
			// 实际实现中需要等待所有worker完成
			// 这里简化处理，等待2秒
			select {
			case <-time.After(2 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		shutdown.OrderWaitForActiveRequests,
	)

	// 3. 刷新Kafka生产者缓冲区
	c.gracefulShutdown.RegisterShutdownFunc(
		"flush_kafka_producer",
		func(ctx context.Context) error {
			c.logger.Info("刷新Kafka生产者缓冲区...")
			// AsyncKafkaOutput需要实现Flush方法
			if flusher, ok := c.outputter.(interface{ Flush() error }); ok {
				return flusher.Flush()
			}
			return nil
		},
		shutdown.OrderFlushProducers,
	)

	// 4. 保存当前进度
	c.gracefulShutdown.RegisterShutdownFunc(
		"save_progress",
		func(ctx context.Context) error {
			c.logger.Info("保存当前采集进度...")
			return c.SaveProgressCheckpoint()
		},
		shutdown.OrderSaveState,
	)

	// 5. 关闭数据库连接和外部服务
	c.gracefulShutdown.RegisterShutdownFunc(
		"close_connections",
		func(ctx context.Context) error {
			c.logger.Info("关闭外部连接...")
			c.closeResources()
			return nil
		},
		shutdown.OrderCloseConnections,
	)

	c.logger.Info("已注册优雅停机处理函数")
}

// StartGracefulShutdown 启动优雅停机监听
func (c *Collector) StartGracefulShutdown() {
	if c.gracefulShutdown != nil {
		c.gracefulShutdown.Start()
	}
}

// WaitForShutdown 等待停机完成
func (c *Collector) WaitForShutdown() {
	if c.gracefulShutdown != nil {
		c.gracefulShutdown.Wait()
	}
}

// GetShutdownContext 获取停机上下文
func (c *Collector) GetShutdownContext() context.Context {
	if c.gracefulShutdown != nil {
		return c.gracefulShutdown.Context()
	}
	return context.Background()
}

// LogBlock 记录区块处理日志
func (c *Collector) LogBlock(blockNumber uint64, message string, fields map[string]any) {
	if c.structuredLogger != nil {
		allFields := map[string]any{
			"component":    "block_processor",
			"block_number": blockNumber,
		}
		for k, v := range fields {
			allFields[k] = v
		}
		c.structuredLogger.InfoWithFields(message, allFields)
	} else {
		c.logger.Infof("[Block %d] %s", blockNumber, message)
	}
}

// LogTransaction 记录交易处理日志
func (c *Collector) LogTransaction(blockNumber uint64, txHash string, message string, fields map[string]any) {
	if c.structuredLogger != nil {
		allFields := map[string]any{
			"component":    "transaction_processor",
			"block_number": blockNumber,
			"tx_hash":      txHash,
		}
		for k, v := range fields {
			allFields[k] = v
		}
		c.structuredLogger.InfoWithFields(message, allFields)
	} else {
		c.logger.Infof("[Block %d][Tx %s] %s", blockNumber, txHash, message)
	}
}

// LogRPC 记录RPC调用日志
func (c *Collector) LogRPC(method string, nodeURL string, message string, fields map[string]any) {
	if c.structuredLogger != nil {
		allFields := map[string]any{
			"component": "rpc_client",
			"method":    method,
			"node_url":  nodeURL,
		}
		for k, v := range fields {
			allFields[k] = v
		}
		c.structuredLogger.InfoWithFields(message, allFields)
	} else {
		c.logger.Infof("[RPC %s] %s", method, message)
	}
}

// LogError 记录错误日志
func (c *Collector) LogError(component string, message string, err error, fields map[string]any) {
	if c.structuredLogger != nil {
		allFields := map[string]any{
			"component": component,
			"error":     err.Error(),
		}
		for k, v := range fields {
			allFields[k] = v
		}
		c.structuredLogger.ErrorWithFields(message, allFields)
	} else {
		c.logger.Errorf("[%s] %s: %v", component, message, err)
	}
}

// LogPerformance 记录性能指标日志
func (c *Collector) LogPerformance(operation string, duration time.Duration, fields map[string]any) {
	if c.structuredLogger != nil {
		allFields := map[string]any{
			"component":       "performance",
			"operation":       operation,
			"duration_ms":     duration.Milliseconds(),
			"duration_string": duration.String(),
		}
		for k, v := range fields {
			allFields[k] = v
		}
		c.structuredLogger.InfoWithFields("性能指标", allFields)
	} else {
		c.logger.Infof("[Performance] %s took %v", operation, duration)
	}
}
